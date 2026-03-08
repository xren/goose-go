package agent

import (
	"context"
	"errors"
	"testing"

	"goose-go/internal/conversation"
	"goose-go/internal/provider"
)

func TestReplyAwaitsApprovalWhenNoApprover(t *testing.T) {
	agent, _, record := newTestAgent(t, scriptedProvider{
		respond: func(_ provider.Request) []provider.Event {
			msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"pwd"}`)))
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, ApprovalModeApprove, nil)

	result, err := agent.Reply(context.Background(), record.ID, "run pwd")
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if result.Status != StatusAwaitingApproval {
		t.Fatalf("expected awaiting approval, got %q", result.Status)
	}
	if result.PendingApprovalFor == nil || result.PendingApprovalFor.Name != "shell" {
		t.Fatalf("expected pending shell approval, got %#v", result.PendingApprovalFor)
	}
}

func TestPendingApprovalReturnsFirstPendingCall(t *testing.T) {
	agent, _, record := newTestAgent(t, scriptedProvider{
		respond: func(_ provider.Request) []provider.Event {
			msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"pwd"}`)))
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, ApprovalModeApprove, nil)

	result, err := agent.Reply(context.Background(), record.ID, "run pwd")
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if result.Status != StatusAwaitingApproval {
		t.Fatalf("expected awaiting approval, got %q", result.Status)
	}

	req, err := agent.PendingApproval(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("pending approval: %v", err)
	}
	if req.ToolCall.ID != "call_1" || req.ToolCall.Name != "shell" {
		t.Fatalf("unexpected pending approval request: %#v", req)
	}
}

func TestResolveApprovalAllowContinuesRun(t *testing.T) {
	agent, store, record := newTestAgent(t, scriptedProvider{
		respond: func(req provider.Request) []provider.Event {
			if hasToolResponse(req.Messages) {
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("done"))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			}
			msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"printf hello"}`)))
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, ApprovalModeApprove, nil)

	result, err := agent.Reply(context.Background(), record.ID, "say hello")
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if result.Status != StatusAwaitingApproval {
		t.Fatalf("expected awaiting approval, got %q", result.Status)
	}

	result, err = agent.ResolveApproval(context.Background(), record.ID, ApprovalDecisionAllow)
	if err != nil {
		t.Fatalf("resolve approval: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed status, got %q", result.Status)
	}

	got, err := store.GetSession(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(got.Conversation.Messages) != 4 {
		t.Fatalf("expected 4 messages after approval continuation, got %d", len(got.Conversation.Messages))
	}
	if got.Conversation.Messages[2].Role != conversation.RoleTool {
		t.Fatalf("expected tool response after approval, got %#v", got.Conversation.Messages[2])
	}
}

func TestResolveApprovalCanPauseOnNextPendingToolCall(t *testing.T) {
	agent, store, record := newTestAgent(t, scriptedProvider{
		respond: func(_ provider.Request) []provider.Event {
			msg := conversation.NewMessage(
				conversation.RoleAssistant,
				conversation.ToolRequest("call_1", "shell", []byte(`{"command":"printf first"}`)),
				conversation.ToolRequest("call_2", "shell", []byte(`{"command":"printf second"}`)),
			)
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, ApprovalModeApprove, nil)

	result, err := agent.Reply(context.Background(), record.ID, "run two tools")
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if result.Status != StatusAwaitingApproval || result.PendingApprovalFor == nil || result.PendingApprovalFor.ID != "call_1" {
		t.Fatalf("expected first call pending, got %#v", result)
	}

	result, err = agent.ResolveApproval(context.Background(), record.ID, ApprovalDecisionAllow)
	if err != nil {
		t.Fatalf("resolve approval: %v", err)
	}
	if result.Status != StatusAwaitingApproval || result.PendingApprovalFor == nil || result.PendingApprovalFor.ID != "call_2" {
		t.Fatalf("expected second call pending after first approval, got %#v", result)
	}

	got, err := store.GetSession(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(got.Conversation.Messages) != 3 {
		t.Fatalf("expected assistant + one tool response persisted, got %d messages", len(got.Conversation.Messages))
	}
}

func TestResolveApprovalReturnsNotPendingWhenSessionHasNoPendingApproval(t *testing.T) {
	agent, _, record := newTestAgent(t, scriptedProvider{
		respond: func(_ provider.Request) []provider.Event {
			msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("pong"))
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, ApprovalModeApprove, nil)

	if _, err := agent.Reply(context.Background(), record.ID, "ping"); err != nil {
		t.Fatalf("reply: %v", err)
	}

	_, err := agent.ResolveApproval(context.Background(), record.ID, ApprovalDecisionAllow)
	if !errors.Is(err, ErrApprovalNotPending) {
		t.Fatalf("expected ErrApprovalNotPending, got %v", err)
	}
}

func TestReplyDeniedToolContinues(t *testing.T) {
	agent, store, record := newTestAgent(t, scriptedProvider{
		respond: func(req provider.Request) []provider.Event {
			if hasToolResponse(req.Messages) {
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("understood"))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			}
			msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"pwd"}`)))
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, ApprovalModeApprove, ApproverFunc(func(context.Context, ApprovalRequest) (ApprovalDecision, error) {
		return ApprovalDecisionDeny, nil
	}))

	result, err := agent.Reply(context.Background(), record.ID, "run pwd")
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed status, got %q", result.Status)
	}

	got, err := store.GetSession(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	toolMsg := got.Conversation.Messages[2]
	if toolMsg.Role != conversation.RoleTool || !toolMsg.Content[0].ToolResponse.IsError {
		t.Fatalf("expected denied tool response, got %#v", toolMsg)
	}
}
