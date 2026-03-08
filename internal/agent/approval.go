package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"goose-go/internal/conversation"
	"goose-go/internal/session"
	"goose-go/internal/tools"
)

func (a *Agent) PendingApproval(ctx context.Context, sessionID string) (*ApprovalRequest, error) {
	record, err := a.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load session %s: %w", sessionID, err)
	}

	pendingCalls, err := extractPendingToolCalls(record.Conversation.Messages)
	if err != nil {
		return nil, err
	}
	if len(pendingCalls) == 0 {
		return nil, ErrApprovalNotPending
	}
	return &ApprovalRequest{SessionID: sessionID, ToolCall: pendingCalls[0]}, nil
}

func (a *Agent) resolveApproval(ctx context.Context, sessionID string, decision ApprovalDecision, emit func(Event)) (Result, error) {
	if decision != ApprovalDecisionAllow && decision != ApprovalDecisionDeny {
		return Result{}, fmt.Errorf("approval decision: unsupported decision %q", decision)
	}

	record, err := a.store.GetSession(ctx, sessionID)
	if err != nil {
		return Result{}, fmt.Errorf("load session %s: %w", sessionID, err)
	}

	turn := currentTurn(record.Conversation.Messages)
	pendingCalls, err := extractPendingToolCalls(record.Conversation.Messages)
	if err != nil {
		return Result{}, err
	}
	if len(pendingCalls) == 0 {
		return Result{}, ErrApprovalNotPending
	}

	emit(Event{Type: EventTypeRunStarted, SessionID: sessionID})

	record, pending, err := a.processToolCalls(ctx, record, turn, pendingCalls, map[string]ApprovalDecision{
		pendingCalls[0].ID: decision,
	}, emit)
	if err != nil {
		return Result{}, err
	}
	if pending != nil {
		return *pending, nil
	}

	return a.runTurns(ctx, record, turn+1, emit)
}

func (a *Agent) processToolCalls(
	ctx context.Context,
	record session.Session,
	turn int,
	toolCalls []tools.Call,
	preapproved map[string]ApprovalDecision,
	emit func(Event),
) (session.Session, *Result, error) {
	for _, toolCall := range toolCalls {
		decision, ok := preapproved[toolCall.ID]
		if !ok {
			call := toolCall
			emit(Event{Type: EventTypeToolCallDetected, SessionID: record.ID, Turn: turn, ToolCall: &call})

			var pending *tools.Call
			var err error
			decision, pending, err = a.approvalDecision(ctx, record.ID, toolCall)
			if err != nil {
				return session.Session{}, nil, err
			}
			if pending != nil {
				emit(Event{Type: EventTypeApprovalRequired, SessionID: record.ID, Turn: turn, ApprovalRequest: &ApprovalRequest{SessionID: record.ID, ToolCall: *pending}})
				return record, &Result{Status: StatusAwaitingApproval, Session: record, Turns: turn, PendingApprovalFor: pending}, nil
			}
		}

		emit(Event{Type: EventTypeApprovalResolved, SessionID: record.ID, Turn: turn, ApprovalDecision: decision, ApprovalRequest: &ApprovalRequest{SessionID: record.ID, ToolCall: toolCall}})

		result, err := a.executeTool(ctx, record, toolCall, decision, turn, emit)
		if err != nil {
			return session.Session{}, nil, err
		}

		toolMessage := conversation.NewMessage(conversation.RoleTool, result.ToConversationContent())
		record, err = a.store.AddMessage(ctx, record.ID, toolMessage)
		if err != nil {
			return session.Session{}, nil, fmt.Errorf("append tool response: %w", err)
		}
		callCopy := toolCall
		emit(Event{Type: EventTypeToolMessagePersisted, SessionID: record.ID, Turn: turn, Message: &toolMessage, ToolCall: &callCopy, ToolResult: &result})
	}
	return record, nil, nil
}

func (a *Agent) approvalDecision(ctx context.Context, sessionID string, call tools.Call) (ApprovalDecision, *tools.Call, error) {
	switch a.config.ApprovalMode {
	case ApprovalModeAuto:
		return ApprovalDecisionAllow, nil, nil
	case ApprovalModeApprove:
		if a.approver == nil {
			pending := call
			return "", &pending, nil
		}
		decision, err := a.approver.Decide(ctx, ApprovalRequest{SessionID: sessionID, ToolCall: call})
		if err != nil {
			return "", nil, fmt.Errorf("approval decision: %w", err)
		}
		if decision != ApprovalDecisionAllow && decision != ApprovalDecisionDeny {
			return "", nil, fmt.Errorf("approval decision: unsupported decision %q", decision)
		}
		return decision, nil, nil
	default:
		return "", nil, fmt.Errorf("unsupported approval mode %q", a.config.ApprovalMode)
	}
}

func (a *Agent) executeTool(ctx context.Context, record session.Session, call tools.Call, decision ApprovalDecision, turn int, emit func(Event)) (tools.Result, error) {
	if decision == ApprovalDecisionDeny {
		return deniedToolResult(call), nil
	}

	if call.DefaultWorkingDir == "" {
		call.DefaultWorkingDir = record.WorkingDir
	}

	callCopy := call
	emit(Event{Type: EventTypeToolExecutionStarted, SessionID: record.ID, Turn: turn, ToolCall: &callCopy})

	result, err := a.tools.Execute(ctx, call)
	if err == nil {
		emit(Event{Type: EventTypeToolExecutionFinished, SessionID: record.ID, Turn: turn, ToolCall: &callCopy, ToolResult: &result})
		return result, nil
	}

	result, wrapErr := errorToolResult(call, err)
	if wrapErr != nil {
		return tools.Result{}, wrapErr
	}
	emit(Event{Type: EventTypeToolExecutionFinished, SessionID: record.ID, Turn: turn, ToolCall: &callCopy, ToolResult: &result})
	return result, nil
}

func extractToolCalls(message conversation.Message) []tools.Call {
	calls := make([]tools.Call, 0, len(message.Content))
	for _, content := range message.Content {
		if content.Type != conversation.ContentTypeToolRequest || content.ToolRequest == nil {
			continue
		}
		calls = append(calls, tools.Call{
			ID:        content.ToolRequest.ID,
			Name:      content.ToolRequest.Name,
			Arguments: content.ToolRequest.Arguments,
		})
	}
	return calls
}

func extractPendingToolCalls(messages []conversation.Message) ([]tools.Call, error) {
	callsByID := make(map[string]tools.Call)
	callOrder := make([]string, 0)
	resolved := make(map[string]bool)

	for _, message := range messages {
		for _, content := range message.Content {
			switch content.Type {
			case conversation.ContentTypeToolRequest:
				if content.ToolRequest == nil {
					continue
				}
				if _, exists := callsByID[content.ToolRequest.ID]; !exists {
					callOrder = append(callOrder, content.ToolRequest.ID)
				}
				callsByID[content.ToolRequest.ID] = tools.Call{
					ID:        content.ToolRequest.ID,
					Name:      content.ToolRequest.Name,
					Arguments: content.ToolRequest.Arguments,
				}
			case conversation.ContentTypeToolResponse:
				if content.ToolResponse == nil {
					continue
				}
				resolved[content.ToolResponse.ID] = true
			}
		}
	}

	pending := make([]tools.Call, 0)
	for _, id := range callOrder {
		if resolved[id] {
			continue
		}
		call, ok := callsByID[id]
		if !ok {
			return nil, fmt.Errorf("pending tool call %q not found", id)
		}
		pending = append(pending, call)
	}
	return pending, nil
}

func currentTurn(messages []conversation.Message) int {
	turn := 0
	for _, message := range messages {
		if message.Role == conversation.RoleAssistant {
			turn++
		}
	}
	if turn == 0 {
		return 1
	}
	return turn
}

func deniedToolResult(call tools.Call) tools.Result {
	payload, _ := json.Marshal(map[string]any{
		"status":  "denied",
		"tool":    call.Name,
		"call_id": call.ID,
	})
	return tools.Result{
		ToolCallID: call.ID,
		IsError:    true,
		Content: []conversation.ToolResult{{
			Type: "text",
			Text: "tool execution denied by user",
		}},
		Structured: payload,
	}
}

func errorToolResult(call tools.Call, runErr error) (tools.Result, error) {
	payload, err := json.Marshal(map[string]any{
		"status":  "error",
		"tool":    call.Name,
		"call_id": call.ID,
		"error":   runErr.Error(),
	})
	if err != nil {
		return tools.Result{}, fmt.Errorf("marshal tool error result: %w", err)
	}
	return tools.Result{
		ToolCallID: call.ID,
		IsError:    true,
		Content: []conversation.ToolResult{{
			Type: "text",
			Text: runErr.Error(),
		}},
		Structured: payload,
	}, nil
}
