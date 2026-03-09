package agent

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"goose-go/internal/compaction"
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

func TestReplyThresholdCompactionUsesCompactedContext(t *testing.T) {
	var requests []provider.Request
	agent, store, record := newTestAgent(t, scriptedProvider{
		respond: func(req provider.Request) []provider.Event {
			requests = append(requests, req)
			msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("done"))
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, ApprovalModeAuto, nil)
	agent.config.Model.ContextWindow = 120
	agent.config.Compaction.ReserveTokens = 20
	agent.config.Compaction.KeepRecentTokens = 20

	for i := 0; i < 6; i++ {
		if _, err := store.AddMessage(context.Background(), record.ID, conversation.NewMessage(
			conversation.RoleUser,
			conversation.Text(strings.Repeat("history ", 20)),
		)); err != nil {
			t.Fatalf("seed history: %v", err)
		}
	}

	result, err := agent.Reply(context.Background(), record.ID, "continue")
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed status, got %q", result.Status)
	}

	latest, err := store.GetLatestCompaction(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("get latest compaction: %v", err)
	}
	if latest.Trigger != session.CompactionTriggerThreshold {
		t.Fatalf("expected threshold compaction, got %q", latest.Trigger)
	}

	if len(requests) != 2 {
		t.Fatalf("expected one summarizer request plus one provider request after threshold compaction, got %d", len(requests))
	}
	if len(requests[1].Messages) == 0 {
		t.Fatal("expected provider messages")
	}
	first := requests[1].Messages[0]
	if first.Role != conversation.RoleAssistant {
		t.Fatalf("expected compacted summary assistant message first, got %q", first.Role)
	}
	if first.Content[0].Text == nil || !strings.Contains(first.Content[0].Text.Text, "Compacted session summary") {
		t.Fatalf("expected synthetic summary message, got %#v", first.Content)
	}
}

func TestReplyOverflowRecoveryCompactsAndRetries(t *testing.T) {
	var requests []provider.Request
	agent, store, record := newTestAgent(t, scriptedProvider{
		respond: func(req provider.Request) []provider.Event {
			requests = append(requests, req)
			if len(requests) == 1 {
				return []provider.Event{{Type: provider.EventTypeError, Err: errors.New("context window exceeded")}}
			}
			msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("recovered"))
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, ApprovalModeAuto, nil)
	agent.config.Model.ContextWindow = 1000
	agent.config.Compaction.ReserveTokens = 20
	agent.config.Compaction.KeepRecentTokens = 20

	for i := 0; i < 4; i++ {
		if _, err := store.AddMessage(context.Background(), record.ID, conversation.NewMessage(
			conversation.RoleUser,
			conversation.Text(strings.Repeat("history ", 20)),
		)); err != nil {
			t.Fatalf("seed history: %v", err)
		}
	}

	result, err := agent.Reply(context.Background(), record.ID, "continue")
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed status, got %q", result.Status)
	}
	if len(requests) != 3 {
		t.Fatalf("expected overflow request, summarizer request, and retry, got %d requests", len(requests))
	}

	latest, err := store.GetLatestCompaction(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("get latest compaction: %v", err)
	}
	if latest.Trigger != session.CompactionTriggerOverflow {
		t.Fatalf("expected overflow compaction, got %q", latest.Trigger)
	}

	first := requests[2].Messages[0]
	if first.Role != conversation.RoleAssistant {
		t.Fatalf("expected compacted summary assistant message first, got %q", first.Role)
	}
}

func TestReplyThresholdCompactionIncludesPersistedSummaryInPlanning(t *testing.T) {
	var requests []provider.Request
	agent, store, record := newTestAgent(t, scriptedProvider{
		respond: func(req provider.Request) []provider.Event {
			requests = append(requests, req)
			if req.SystemPrompt != "You are helpful." {
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("updated summary"))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			}
			msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("done"))
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, ApprovalModeAuto, nil)
	agent.config.Model.ContextWindow = 160
	agent.config.Compaction.ReserveTokens = 20
	agent.config.Compaction.KeepRecentTokens = 20

	var keptID string
	for i := 0; i < 3; i++ {
		msg := conversation.NewMessage(conversation.RoleUser, conversation.Text("small suffix"))
		if _, err := store.AddMessage(context.Background(), record.ID, msg); err != nil {
			t.Fatalf("seed history: %v", err)
		}
		if i == 1 {
			keptID = msg.ID
		}
	}
	if _, err := store.AppendCompaction(context.Background(), record.ID, session.CompactionParams{
		Summary:            strings.Repeat("large summary ", 40),
		FirstKeptMessageID: keptID,
		TokensBefore:       999,
		Trigger:            session.CompactionTriggerThreshold,
	}); err != nil {
		t.Fatalf("append compaction: %v", err)
	}

	result, err := agent.Reply(context.Background(), record.ID, "continue")
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed status, got %q", result.Status)
	}
	if len(requests) != 2 {
		t.Fatalf("expected one summarizer request plus one provider request, got %d", len(requests))
	}

	latest, err := store.GetLatestCompaction(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("get latest compaction: %v", err)
	}
	if latest.Summary != "updated summary" {
		t.Fatalf("expected latest compaction summary to be refreshed, got %q", latest.Summary)
	}
}

func TestReplyThresholdCompactionForcesReductionWhenPlannerWouldSummarizeNothing(t *testing.T) {
	var requests []provider.Request
	agent, store, record := newTestAgent(t, scriptedProvider{
		respond: func(req provider.Request) []provider.Event {
			requests = append(requests, req)
			if req.SystemPrompt != "You are helpful." {
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("forced summary"))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			}
			msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("done"))
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, ApprovalModeAuto, nil)
	agent.config.Model.ContextWindow = 120
	agent.config.Compaction.ReserveTokens = 20
	agent.config.Compaction.KeepRecentTokens = 0

	for i := 0; i < 3; i++ {
		if _, err := store.AddMessage(context.Background(), record.ID, conversation.NewMessage(
			conversation.RoleUser,
			conversation.Text(strings.Repeat("history ", 20)),
		)); err != nil {
			t.Fatalf("seed history: %v", err)
		}
	}

	result, err := agent.Reply(context.Background(), record.ID, "continue")
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed status, got %q", result.Status)
	}
	if len(requests) != 2 {
		t.Fatalf("expected forced compaction to produce summarizer request plus provider request, got %d", len(requests))
	}

	first := requests[1].Messages[0]
	if first.Role != conversation.RoleAssistant {
		t.Fatalf("expected compacted summary assistant message first, got %q", first.Role)
	}
}

func TestNewPreservesDisabledCompaction(t *testing.T) {
	store, err := sqlitestore.Open(t.TempDir() + "/sessions.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	registry := tools.NewRegistry()
	if err := registry.Register(shell.New()); err != nil {
		t.Fatalf("register shell: %v", err)
	}

	runtime, err := New(store, scriptedProvider{
		respond: func(req provider.Request) []provider.Event {
			_ = req
			msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("pong"))
			return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
		},
	}, registry, Config{
		SystemPrompt: "You are helpful.",
		Model:        provider.ModelConfig{Provider: "test", Model: "test-model"},
		MaxTurns:     1,
		ApprovalMode: ApprovalModeAuto,
		Compaction:   compaction.Settings{Enabled: false},
	}, nil)
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}
	if runtime.config.Compaction.Enabled {
		t.Fatalf("expected compaction to stay disabled, got %#v", runtime.config.Compaction)
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

	return newTestAgentWithStoreAndRegistry(t, store, record, p, mode, approver, registry)
}

func newTestAgentWithRegistry(t *testing.T, p provider.Provider, mode ApprovalMode, approver Approver, registry *tools.Registry) (*Agent, session.Store, session.Session) {
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

	return newTestAgentWithStoreAndRegistry(t, store, record, p, mode, approver, registry)
}

func newTestAgentWithStoreAndRegistry(t *testing.T, store session.Store, record session.Session, p provider.Provider, mode ApprovalMode, approver Approver, registry *tools.Registry) (*Agent, session.Store, session.Session) {
	t.Helper()

	agent, err := New(store, p, registry, Config{
		SystemPrompt: "You are helpful.",
		Model:        provider.ModelConfig{Provider: "test", Model: "test-model"},
		MaxTurns:     3,
		ApprovalMode: mode,
		Compaction:   compaction.DefaultSettings(),
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
	events := s.respond(req)
	ch := make(chan provider.Event, len(events))
	for _, event := range events {
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
