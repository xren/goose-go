package agent

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
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

func TestReplyToolUsesSessionWorkingDirByDefault(t *testing.T) {
	agent, store, record := newTestAgent(t, scriptedProvider{
		respond: func(req provider.Request) []provider.Event {
			if hasToolResponse(req.Messages) {
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("done"))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			}
			msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"pwd"}`)))
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, ApprovalModeAuto, nil)

	result, err := agent.Reply(context.Background(), record.ID, "show cwd")
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
	if len(got.Conversation.Messages) < 3 {
		t.Fatalf("expected tool response message, got %d messages", len(got.Conversation.Messages))
	}

	toolMsg := got.Conversation.Messages[2]
	if toolMsg.Role != conversation.RoleTool {
		t.Fatalf("expected tool role message, got %q", toolMsg.Role)
	}
	if len(toolMsg.Content) == 0 || toolMsg.Content[0].ToolResponse == nil {
		t.Fatalf("expected tool response content, got %#v", toolMsg.Content)
	}
	toolText := strings.TrimSpace(toolMsg.Content[0].ToolResponse.Content[0].Text)
	if filepath.Clean(toolText) != record.WorkingDir {
		t.Fatalf("expected tool to run in session working dir %q, got %q", record.WorkingDir, toolText)
	}
}

func TestReplyStreamPlainAssistantMessage(t *testing.T) {
	agent, _, record := newTestAgent(t, scriptedProvider{
		respond: func(_ provider.Request) []provider.Event {
			return []provider.Event{
				{Type: provider.EventTypeTextDelta, Delta: "po"},
				{Type: provider.EventTypeTextDelta, Delta: "ng"},
				{Type: provider.EventTypeMessageComplete, Message: messagePtr(conversation.NewMessage(conversation.RoleAssistant, conversation.Text("pong")))},
				{Type: provider.EventTypeDone},
			}
		},
	}, ApprovalModeAuto, nil)

	stream, err := agent.ReplyStream(context.Background(), record.ID, "ping")
	if err != nil {
		t.Fatalf("reply stream: %v", err)
	}

	var types []EventType
	for event := range stream {
		types = append(types, event.Type)
	}

	want := []EventType{
		EventTypeRunStarted,
		EventTypeUserMessagePersisted,
		EventTypeTurnStarted,
		EventTypeProviderTextDelta,
		EventTypeProviderTextDelta,
		EventTypeAssistantMessageComplete,
		EventTypeAssistantMessagePersisted,
		EventTypeRunCompleted,
	}
	if !slices.Equal(types, want) {
		t.Fatalf("expected event types %v, got %v", want, types)
	}
}

func TestReplyStreamToolLifecycle(t *testing.T) {
	agent, _, record := newTestAgent(t, scriptedProvider{
		respond: func(req provider.Request) []provider.Event {
			if hasToolResponse(req.Messages) {
				return []provider.Event{
					{Type: provider.EventTypeMessageComplete, Message: messagePtr(conversation.NewMessage(conversation.RoleAssistant, conversation.Text("done")))},
					{Type: provider.EventTypeDone},
				}
			}
			return []provider.Event{
				{Type: provider.EventTypeMessageComplete, Message: messagePtr(conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"printf hello"}`))))},
				{Type: provider.EventTypeDone},
			}
		},
	}, ApprovalModeAuto, nil)

	stream, err := agent.ReplyStream(context.Background(), record.ID, "say hello")
	if err != nil {
		t.Fatalf("reply stream: %v", err)
	}

	var types []EventType
	for event := range stream {
		types = append(types, event.Type)
	}

	for _, want := range []EventType{
		EventTypeToolCallDetected,
		EventTypeApprovalResolved,
		EventTypeToolExecutionStarted,
		EventTypeToolExecutionFinished,
		EventTypeToolMessagePersisted,
		EventTypeRunCompleted,
	} {
		if !slices.Contains(types, want) {
			t.Fatalf("expected event stream to contain %q, got %v", want, types)
		}
	}
}

func TestReplyStreamApprovalRequired(t *testing.T) {
	agent, _, record := newTestAgent(t, scriptedProvider{
		respond: func(_ provider.Request) []provider.Event {
			return []provider.Event{
				{Type: provider.EventTypeMessageComplete, Message: messagePtr(conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"pwd"}`))))},
				{Type: provider.EventTypeDone},
			}
		},
	}, ApprovalModeApprove, nil)

	stream, err := agent.ReplyStream(context.Background(), record.ID, "run pwd")
	if err != nil {
		t.Fatalf("reply stream: %v", err)
	}

	var (
		sawApprovalRequired bool
		terminal            *Event
	)
	for event := range stream {
		if event.Type == EventTypeApprovalRequired {
			sawApprovalRequired = true
		}
		if event.Type == EventTypeRunCompleted {
			copy := event
			terminal = &copy
		}
	}

	if !sawApprovalRequired {
		t.Fatal("expected approval required event")
	}
	if terminal == nil || terminal.Result == nil || terminal.Result.Status != StatusAwaitingApproval {
		t.Fatalf("expected terminal awaiting approval result, got %#v", terminal)
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

func messagePtr(message conversation.Message) *conversation.Message {
	return &message
}
