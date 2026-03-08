package agent

import (
	"context"
	"errors"
	"testing"

	"goose-go/internal/conversation"
	"goose-go/internal/provider"
	"goose-go/internal/session"
	sqlitestore "goose-go/internal/storage/sqlite"
	"goose-go/internal/tools"
	"goose-go/internal/tools/shell"
)

func TestReplyPlainAssistantMessage(t *testing.T) {
	agent, store, record := newTestAgent(t, scriptedProvider{
		respond: func(_ provider.Request) []provider.Event {
			msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("pong"))
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, ApprovalModeAuto, nil)

	result, err := agent.Reply(context.Background(), record.ID, "ping")
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed status, got %q", result.Status)
	}
	if result.Turns != 1 {
		t.Fatalf("expected one turn, got %d", result.Turns)
	}

	got, err := store.GetSession(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(got.Conversation.Messages) != 2 {
		t.Fatalf("expected user + assistant messages, got %d", len(got.Conversation.Messages))
	}
}

func TestReplyToolCallThenFollowup(t *testing.T) {
	agent, store, record := newTestAgent(t, scriptedProvider{
		respond: func(req provider.Request) []provider.Event {
			if hasToolResponse(req.Messages) {
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("done"))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			}
			msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"printf hello"}`)))
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, ApprovalModeAuto, nil)

	result, err := agent.Reply(context.Background(), record.ID, "say hello")
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed status, got %q", result.Status)
	}
	if result.Turns != 2 {
		t.Fatalf("expected two turns, got %d", result.Turns)
	}

	got, err := store.GetSession(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(got.Conversation.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(got.Conversation.Messages))
	}
	if got.Conversation.Messages[2].Role != conversation.RoleTool {
		t.Fatalf("expected third message to be tool role, got %q", got.Conversation.Messages[2].Role)
	}
}

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

func TestReplyReturnsMaxTurnsExceeded(t *testing.T) {
	agent, _, record := newTestAgent(t, scriptedProvider{
		respond: func(_ provider.Request) []provider.Event {
			msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"printf loop"}`)))
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, ApprovalModeAuto, nil)
	agent.config.MaxTurns = 1

	_, err := agent.Reply(context.Background(), record.ID, "loop")
	if !errors.Is(err, ErrMaxTurnsExceeded) {
		t.Fatalf("expected max turns error, got %v", err)
	}
}

func newTestAgent(t *testing.T, p provider.Provider, mode ApprovalMode, approver Approver) (*Agent, session.Store, session.Session) {
	t.Helper()

	store, err := sqlitestore.Open(t.TempDir() + "/sessions.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	record, err := store.CreateSession(context.Background(), session.CreateParams{Name: "test", WorkingDir: t.TempDir(), Type: session.TypeTerminal})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	registry := tools.NewRegistry()
	if err := registry.Register(shell.New()); err != nil {
		t.Fatalf("register shell: %v", err)
	}

	agent, err := New(store, p, registry, Config{
		SystemPrompt: "You are helpful.",
		Model:        provider.ModelConfig{Provider: "test", Model: "test-model"},
		MaxTurns:     3,
		ApprovalMode: mode,
	}, approver)
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}

	return agent, store, record
}

type scriptedProvider struct {
	respond func(provider.Request) []provider.Event
}

func (s scriptedProvider) streamWithRequest(req provider.Request) (<-chan provider.Event, error) {
	ch := make(chan provider.Event, len(s.respond(req)))
	for _, event := range s.respond(req) {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func (s scriptedProvider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error) {
	_ = ctx
	return s.streamWithRequest(req)
}

func hasToolResponse(messages []conversation.Message) bool {
	for _, message := range messages {
		for _, content := range message.Content {
			if content.Type == conversation.ContentTypeToolResponse {
				return true
			}
		}
	}
	return false
}
