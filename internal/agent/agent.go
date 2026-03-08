package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"goose-go/internal/conversation"
	"goose-go/internal/provider"
	"goose-go/internal/session"
	"goose-go/internal/tools"
)

type ApprovalMode string

type Status string

type ApprovalDecision string

const (
	ApprovalModeAuto    ApprovalMode = "auto"
	ApprovalModeApprove ApprovalMode = "approve"

	StatusCompleted        Status = "completed"
	StatusAwaitingApproval Status = "awaiting_approval"

	ApprovalDecisionAllow ApprovalDecision = "allow"
	ApprovalDecisionDeny  ApprovalDecision = "deny"
)

var ErrMaxTurnsExceeded = errors.New("max turns exceeded")

type Approver interface {
	Decide(context.Context, ApprovalRequest) (ApprovalDecision, error)
}

type ApproverFunc func(context.Context, ApprovalRequest) (ApprovalDecision, error)

func (f ApproverFunc) Decide(ctx context.Context, req ApprovalRequest) (ApprovalDecision, error) {
	return f(ctx, req)
}

type Config struct {
	SystemPrompt string
	Model        provider.ModelConfig
	MaxTurns     int
	ApprovalMode ApprovalMode
}

type ApprovalRequest struct {
	SessionID string     `json:"session_id"`
	ToolCall  tools.Call `json:"tool_call"`
}

type Result struct {
	Status             Status          `json:"status"`
	Session            session.Session `json:"session"`
	Turns              int             `json:"turns"`
	PendingApprovalFor *tools.Call     `json:"pending_approval_for,omitempty"`
}

type Agent struct {
	provider provider.Provider
	store    session.Store
	tools    *tools.Registry
	config   Config
	approver Approver
}

func New(store session.Store, p provider.Provider, registry *tools.Registry, config Config, approver Approver) (*Agent, error) {
	if store == nil {
		return nil, errors.New("session store is required")
	}
	if p == nil {
		return nil, errors.New("provider is required")
	}
	if registry == nil {
		return nil, errors.New("tool registry is required")
	}
	if config.ApprovalMode == "" {
		config.ApprovalMode = ApprovalModeAuto
	}
	if err := config.validate(); err != nil {
		return nil, err
	}

	return &Agent{
		provider: p,
		store:    store,
		tools:    registry,
		config:   config,
		approver: approver,
	}, nil
}

func (a *Agent) Reply(ctx context.Context, sessionID string, userText string) (Result, error) {
	if userText == "" {
		return Result{}, errors.New("user text is required")
	}

	userMessage := conversation.NewMessage(conversation.RoleUser, conversation.Text(userText))
	record, err := a.store.AddMessage(ctx, sessionID, userMessage)
	if err != nil {
		return Result{}, fmt.Errorf("append user message: %w", err)
	}

	for turn := 1; turn <= a.config.MaxTurns; turn++ {
		assistantMessage, err := a.runProviderTurn(ctx, record)
		if err != nil {
			return Result{}, err
		}

		record, err = a.store.AddMessage(ctx, sessionID, assistantMessage)
		if err != nil {
			return Result{}, fmt.Errorf("append assistant message: %w", err)
		}

		toolCalls := extractToolCalls(assistantMessage)
		if len(toolCalls) == 0 {
			return Result{Status: StatusCompleted, Session: record, Turns: turn}, nil
		}

		for _, toolCall := range toolCalls {
			decision, pending, err := a.approvalDecision(ctx, sessionID, toolCall)
			if err != nil {
				return Result{}, err
			}
			if pending != nil {
				return Result{Status: StatusAwaitingApproval, Session: record, Turns: turn, PendingApprovalFor: pending}, nil
			}

			result, err := a.executeTool(ctx, toolCall, decision)
			if err != nil {
				return Result{}, err
			}

			toolMessage := conversation.NewMessage(conversation.RoleTool, result.ToConversationContent())
			record, err = a.store.AddMessage(ctx, sessionID, toolMessage)
			if err != nil {
				return Result{}, fmt.Errorf("append tool response: %w", err)
			}
		}
	}

	return Result{Status: StatusCompleted, Session: record, Turns: a.config.MaxTurns}, ErrMaxTurnsExceeded
}

func (a *Agent) runProviderTurn(ctx context.Context, record session.Session) (conversation.Message, error) {
	stream, err := a.provider.Stream(ctx, provider.Request{
		SessionID:    record.ID,
		SystemPrompt: a.config.SystemPrompt,
		Messages:     record.Conversation.Messages,
		Tools:        toolDefinitions(a.tools.Definitions()),
		Model:        a.config.Model,
	})
	if err != nil {
		return conversation.Message{}, fmt.Errorf("start provider turn: %w", err)
	}

	var finalMessage *conversation.Message
	for event := range stream {
		switch event.Type {
		case provider.EventTypeMessageComplete:
			finalMessage = event.Message
		case provider.EventTypeError:
			return conversation.Message{}, fmt.Errorf("provider turn failed: %w", event.Err)
		}
	}

	if finalMessage == nil {
		return conversation.Message{}, errors.New("provider turn did not produce a final assistant message")
	}

	return *finalMessage, nil
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

func (a *Agent) executeTool(ctx context.Context, call tools.Call, decision ApprovalDecision) (tools.Result, error) {
	if decision == ApprovalDecisionDeny {
		return deniedToolResult(call), nil
	}

	result, err := a.tools.Execute(ctx, call)
	if err == nil {
		return result, nil
	}

	return errorToolResult(call, err)
}

func (c Config) validate() error {
	if err := c.Model.Validate(); err != nil {
		return fmt.Errorf("model: %w", err)
	}
	if c.MaxTurns <= 0 {
		return errors.New("max turns must be > 0")
	}
	return nil
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

func toolDefinitions(defs []tools.Definition) []provider.ToolDefinition {
	out := make([]provider.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		out = append(out, provider.ToolDefinition{
			Name:        def.Name,
			Description: def.Description,
			InputSchema: def.InputSchema,
		})
	}
	return out
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
