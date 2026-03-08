package evals

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goose-go/internal/agent"
	"goose-go/internal/conversation"
	"goose-go/internal/provider"
	"goose-go/internal/session"
	sqlitestore "goose-go/internal/storage/sqlite"
	"goose-go/internal/tools"
	"goose-go/internal/tools/shell"
)

type traceRecord struct {
	Type       string         `json:"type"`
	SessionID  string         `json:"session_id,omitempty"`
	Turn       int            `json:"turn,omitempty"`
	Decision   string         `json:"approval_decision,omitempty"`
	Delta      string         `json:"delta,omitempty"`
	Error      string         `json:"error,omitempty"`
	Result     map[string]any `json:"result,omitempty"`
	ToolResult map[string]any `json:"tool_result,omitempty"`
	Message    map[string]any `json:"message,omitempty"`
}

func TestEvalScenarios(t *testing.T) {
	t.Run("plain_chat_completion", func(t *testing.T) {
		trace := runScenario(t, scenario{
			prompt: "ping",
			provider: scriptedProvider{
				respond: func(_ provider.Request) []provider.Event {
					msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("pong"))
					return []provider.Event{
						{Type: provider.EventTypeTextDelta, Delta: "pong"},
						{Type: provider.EventTypeMessageComplete, Message: &msg},
						{Type: provider.EventTypeDone},
					}
				},
			},
		})

		assertContainsTypes(t, trace, "run_started", "user_message_persisted", "turn_started", "provider_text_delta", "assistant_message_complete", "assistant_message_persisted", "run_completed")
		assertNotContainsTypes(t, trace, "tool_call_detected", "tool_execution_started", "tool_execution_finished")
		assertFinalType(t, trace, "run_completed")
	})

	t.Run("tool_round_trip", func(t *testing.T) {
		trace := runScenario(t, scenario{
			prompt: "say hello",
			provider: scriptedProvider{
				respond: func(req provider.Request) []provider.Event {
					if hasToolResponse(req.Messages) {
						msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("done"))
						return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
					}
					msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"printf hello"}`)))
					return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
				},
			},
		})

		assertContainsTypes(t, trace, "tool_call_detected", "approval_resolved", "tool_execution_started", "tool_execution_finished", "tool_message_persisted", "run_completed")
		assertFinalType(t, trace, "run_completed")
	})

	t.Run("approval_deny", func(t *testing.T) {
		trace := runScenario(t, scenario{
			prompt: "run pwd",
			provider: scriptedProvider{
				respond: func(req provider.Request) []provider.Event {
					if hasToolResponse(req.Messages) {
						msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("understood"))
						return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
					}
					msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"pwd"}`)))
					return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
				},
			},
			mode: agent.ApprovalModeApprove,
			approver: agent.ApproverFunc(func(context.Context, agent.ApprovalRequest) (agent.ApprovalDecision, error) {
				return agent.ApprovalDecisionDeny, nil
			}),
		})

		assertContainsTypes(t, trace, "approval_resolved", "tool_message_persisted", "run_completed")
		assertNotContainsTypes(t, trace, "tool_execution_started", "tool_execution_finished")
		foundDeny := false
		for _, record := range trace {
			if record.Type == "approval_resolved" && record.Decision == "deny" {
				foundDeny = true
				break
			}
		}
		if !foundDeny {
			t.Fatal("expected approval_resolved event with deny decision")
		}
		assertFinalType(t, trace, "run_completed")
	})

	t.Run("interrupt", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		trace := runScenarioWithContext(t, ctx, scenario{
			prompt: "interrupt me",
			provider: scriptedProvider{
				respondStream: func(ctx context.Context, _ provider.Request) <-chan provider.Event {
					ch := make(chan provider.Event, 1)
					go func() {
						defer close(ch)
						cancel()
						<-ctx.Done()
						ch <- provider.Event{Type: provider.EventTypeError, Err: ctx.Err()}
					}()
					return ch
				},
			},
		})

		assertContainsTypes(t, trace, "run_started", "user_message_persisted", "turn_started", "run_interrupted")
		assertFinalType(t, trace, "run_interrupted")
	})

	t.Run("resume_session", func(t *testing.T) {
		workdir := t.TempDir()
		store := openEvalStore(t)
		record := createEvalSession(t, context.Background(), store, workdir)

		runtime := newEvalRuntime(t, store, scriptedProvider{
			respond: func(req provider.Request) []provider.Event {
				userTexts := collectUserTexts(req.Messages)
				last := userTexts[len(userTexts)-1]
				reply := "reply to: " + last
				if len(userTexts) >= 2 && userTexts[0] == "first prompt" && userTexts[1] == "second prompt" {
					reply = "saw resume context"
				}
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text(reply))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			},
		}, agent.ApprovalModeAuto, nil, 4)

		drainTrace(t, runtime, context.Background(), record.ID, "first prompt")
		trace := drainTrace(t, runtime, context.Background(), record.ID, "second prompt")

		assertContainsTypes(t, trace, "run_started", "user_message_persisted", "assistant_message_complete", "run_completed")
		assertFinalType(t, trace, "run_completed")
		if !traceContainsAssistantText(trace, "saw resume context") {
			t.Fatalf("expected resumed trace to include assistant message %q, got %#v", "saw resume context", trace)
		}
	})

	t.Run("awaiting_approval", func(t *testing.T) {
		trace := runScenario(t, scenario{
			prompt: "run pwd",
			provider: scriptedProvider{
				respond: func(_ provider.Request) []provider.Event {
					msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"pwd"}`)))
					return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
				},
			},
			mode: agent.ApprovalModeApprove,
		})

		assertContainsTypes(t, trace, "tool_call_detected", "approval_required", "run_completed")
		assertFinalType(t, trace, "run_completed")
		if !finalResultHasStatus(trace, "awaiting_approval") {
			t.Fatalf("expected final result status awaiting_approval, got %#v", trace[len(trace)-1].Result)
		}
	})

	t.Run("max_turns", func(t *testing.T) {
		trace := runScenario(t, scenario{
			prompt: "loop",
			provider: scriptedProvider{
				respond: func(_ provider.Request) []provider.Event {
					msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"printf loop"}`)))
					return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
				},
			},
			maxTurns: 1,
		})

		assertContainsTypes(t, trace, "tool_execution_started", "tool_execution_finished", "tool_message_persisted", "run_failed")
		assertFinalType(t, trace, "run_failed")
		if trace[len(trace)-1].Error == "" || !strings.Contains(trace[len(trace)-1].Error, "max turns exceeded") {
			t.Fatalf("expected max-turn error in final trace, got %#v", trace[len(trace)-1])
		}
	})
}

type scenario struct {
	prompt   string
	provider provider.Provider
	mode     agent.ApprovalMode
	approver agent.Approver
	maxTurns int
}

func runScenario(t *testing.T, s scenario) []traceRecord {
	t.Helper()
	return runScenarioWithContext(t, context.Background(), s)
}

func runScenarioWithContext(t *testing.T, ctx context.Context, s scenario) []traceRecord {
	t.Helper()

	workdir := t.TempDir()
	store := openEvalStore(t)
	record := createEvalSession(t, ctx, store, workdir)
	runtime := newEvalRuntime(t, store, s.provider, s.mode, s.approver, s.maxTurns)
	return drainTrace(t, runtime, ctx, record.ID, s.prompt)
}

func assertContainsTypes(t *testing.T, trace []traceRecord, want ...string) {
	t.Helper()
	for _, expected := range want {
		found := false
		for _, record := range trace {
			if record.Type == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected trace to contain %q, got %v", expected, collectTypes(trace))
		}
	}
}

func assertNotContainsTypes(t *testing.T, trace []traceRecord, forbidden ...string) {
	t.Helper()
	for _, blocked := range forbidden {
		for _, record := range trace {
			if record.Type == blocked {
				t.Fatalf("expected trace to omit %q, got %v", blocked, collectTypes(trace))
			}
		}
	}
}

func assertFinalType(t *testing.T, trace []traceRecord, want string) {
	t.Helper()
	if len(trace) == 0 {
		t.Fatal("expected non-empty trace")
	}
	if got := trace[len(trace)-1].Type; got != want {
		t.Fatalf("expected final trace type %q, got %q", want, got)
	}
}

func collectTypes(trace []traceRecord) []string {
	out := make([]string, 0, len(trace))
	for _, record := range trace {
		out = append(out, record.Type)
	}
	return out
}

func openEvalStore(t *testing.T) *sqlitestore.Store {
	t.Helper()
	store, err := sqlitestore.Open(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func createEvalSession(t *testing.T, ctx context.Context, store *sqlitestore.Store, workdir string) session.Session {
	t.Helper()
	record, err := store.CreateSession(ctx, session.CreateParams{
		Name:       "eval",
		WorkingDir: workdir,
		Type:       session.TypeTerminal,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return record
}

func newEvalRuntime(t *testing.T, store *sqlitestore.Store, p provider.Provider, mode agent.ApprovalMode, approver agent.Approver, maxTurns int) *agent.Agent {
	t.Helper()
	registry := tools.NewRegistry()
	if err := registry.Register(shell.New()); err != nil {
		t.Fatalf("register shell: %v", err)
	}
	if mode == "" {
		mode = agent.ApprovalModeAuto
	}
	if maxTurns <= 0 {
		maxTurns = 4
	}
	runtime, err := agent.New(store, p, registry, agent.Config{
		SystemPrompt: "You are helpful.",
		Model:        provider.ModelConfig{Provider: "test", Model: "test-model"},
		MaxTurns:     maxTurns,
		ApprovalMode: mode,
	}, approver)
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}
	return runtime
}

func drainTrace(t *testing.T, runtime *agent.Agent, ctx context.Context, sessionID string, prompt string) []traceRecord {
	t.Helper()
	stream, err := runtime.ReplyStream(ctx, sessionID, prompt)
	if err != nil {
		t.Fatalf("reply stream: %v", err)
	}

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	file, err := os.Create(tracePath)
	if err != nil {
		t.Fatalf("create trace file: %v", err)
	}
	enc := json.NewEncoder(file)
	for event := range stream {
		record := traceRecord{
			Type:      string(event.Type),
			SessionID: event.SessionID,
			Turn:      event.Turn,
			Decision:  string(event.ApprovalDecision),
			Delta:     event.Delta,
		}
		if event.Err != nil {
			record.Error = event.Err.Error()
		}
		if event.Result != nil {
			data, err := json.Marshal(event.Result)
			if err != nil {
				t.Fatalf("marshal result: %v", err)
			}
			if err := json.Unmarshal(data, &record.Result); err != nil {
				t.Fatalf("unmarshal result: %v", err)
			}
		}
		if event.ToolResult != nil {
			data, err := json.Marshal(event.ToolResult)
			if err != nil {
				t.Fatalf("marshal tool result: %v", err)
			}
			if err := json.Unmarshal(data, &record.ToolResult); err != nil {
				t.Fatalf("unmarshal tool result: %v", err)
			}
		}
		if event.Message != nil {
			data, err := json.Marshal(event.Message)
			if err != nil {
				t.Fatalf("marshal message: %v", err)
			}
			if err := json.Unmarshal(data, &record.Message); err != nil {
				t.Fatalf("unmarshal message: %v", err)
			}
		}
		if err := enc.Encode(record); err != nil {
			t.Fatalf("encode trace record: %v", err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close trace file: %v", err)
	}

	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read trace file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	out := make([]traceRecord, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var record traceRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decode trace line: %v", err)
		}
		out = append(out, record)
	}
	return out
}

func collectUserTexts(messages []conversation.Message) []string {
	var out []string
	for _, message := range messages {
		if message.Role != conversation.RoleUser {
			continue
		}
		for _, content := range message.Content {
			if content.Type == conversation.ContentTypeText && content.Text != nil {
				out = append(out, content.Text.Text)
			}
		}
	}
	return out
}

func traceContainsAssistantText(trace []traceRecord, want string) bool {
	for _, record := range trace {
		if record.Type != "assistant_message_complete" || record.Message == nil {
			continue
		}
		content, ok := record.Message["content"].([]any)
		if !ok {
			continue
		}
		for _, item := range content {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			textContainer, ok := block["text"].(map[string]any)
			if !ok {
				continue
			}
			if text, ok := textContainer["text"].(string); ok && text == want {
				return true
			}
		}
	}
	return false
}

func finalResultHasStatus(trace []traceRecord, want string) bool {
	if len(trace) == 0 || trace[len(trace)-1].Result == nil {
		return false
	}
	got, _ := trace[len(trace)-1].Result["status"].(string)
	return got == want
}

type scriptedProvider struct {
	respond       func(provider.Request) []provider.Event
	respondStream func(context.Context, provider.Request) <-chan provider.Event
}

func (s scriptedProvider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error) {
	if s.respondStream != nil {
		return s.respondStream(ctx, req), nil
	}
	events := s.respond(req)
	ch := make(chan provider.Event, len(events))
	go func() {
		defer close(ch)
		for _, event := range events {
			select {
			case ch <- event:
			case <-ctx.Done():
				ch <- provider.Event{Type: provider.EventTypeError, Err: ctx.Err()}
				return
			}
		}
	}()
	return ch, nil
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
