package tui

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"goose-go/internal/agent"
	"goose-go/internal/app"
	"goose-go/internal/conversation"
	"goose-go/internal/session"
	"goose-go/internal/tools"
)

type fakeRuntime struct {
	workingDir         string
	loadSession        session.Session
	loadErr            error
	replay             session.Session
	replayErr          error
	trace              *fakeTraceWriter
	traceErr           error
	streamEvents       []agent.Event
	streamErr          error
	pendingApproval    *agent.ApprovalRequest
	pendingApprovalErr error
	resolveEvents      []agent.Event
	resolveErr         error
}

type fakeTraceWriter struct {
	written []agent.Event
	closed  bool
}

func (f *fakeRuntime) LoadOrCreateSession(context.Context, string, string) (session.Session, int, error) {
	if f.loadErr != nil {
		return session.Session{}, 0, f.loadErr
	}
	return f.loadSession, len(f.loadSession.Conversation.Messages), nil
}

func (f *fakeRuntime) ReplayConversation(context.Context, string) (session.Session, error) {
	if f.replayErr != nil {
		return f.replay, f.replayErr
	}
	return f.replay, nil
}

func (f *fakeRuntime) OpenTraceWriter(string) (app.EventRecorder, error) {
	if f.traceErr != nil {
		return nil, f.traceErr
	}
	if f.trace == nil {
		f.trace = &fakeTraceWriter{}
	}
	return f.trace, nil
}

func (f *fakeRuntime) ReplyStream(context.Context, string, string) (<-chan agent.Event, error) {
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	ch := make(chan agent.Event, len(f.streamEvents))
	for _, event := range f.streamEvents {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func (f *fakeRuntime) PendingApproval(context.Context, string) (*agent.ApprovalRequest, error) {
	if f.pendingApprovalErr != nil {
		return nil, f.pendingApprovalErr
	}
	return f.pendingApproval, nil
}

func (f *fakeRuntime) ResolveApprovalStream(context.Context, string, agent.ApprovalDecision) (<-chan agent.Event, error) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	ch := make(chan agent.Event, len(f.resolveEvents))
	for _, event := range f.resolveEvents {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func (f *fakeRuntime) WorkingDir() string {
	if f.workingDir == "" {
		return "/tmp/work"
	}
	return f.workingDir
}

func (f *fakeRuntime) ProviderModel() (string, string) {
	return "openai-codex", "gpt-5-codex"
}

func (f *fakeTraceWriter) Write(event agent.Event) error {
	f.written = append(f.written, event)
	return nil
}

func (f *fakeTraceWriter) Close() error {
	f.closed = true
	return nil
}

func TestBuildTranscriptFromConversation(t *testing.T) {
	msgTime := time.Now().UTC()
	conv := conversation.Conversation{Messages: []conversation.Message{
		{ID: "m1", Role: conversation.RoleUser, CreatedAt: msgTime, Content: []conversation.Content{conversation.Text("hello")}},
		{ID: "m2", Role: conversation.RoleAssistant, CreatedAt: msgTime, Content: []conversation.Content{conversation.ToolRequest("call_1", "shell", []byte(`{"command":"pwd"}`))}},
		{ID: "m3", Role: conversation.RoleTool, CreatedAt: msgTime, Content: []conversation.Content{conversation.ToolResponse("call_1", false, []conversation.ToolResult{{Type: "text", Text: "/tmp/work"}}, nil)}},
		{ID: "m4", Role: conversation.RoleAssistant, CreatedAt: msgTime, Content: []conversation.Content{conversation.Text("done")}},
	}}

	items := buildTranscriptFromConversation(conv)
	if len(items) != 4 {
		t.Fatalf("expected 4 transcript items, got %d", len(items))
	}
	if items[0].Prefix != "user" || items[0].Text != "hello" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].Prefix != "assistant requested tool" {
		t.Fatalf("unexpected tool request item: %#v", items[1])
	}
	if items[2].Prefix != "tool" || items[2].Text != "/tmp/work" {
		t.Fatalf("unexpected tool response item: %#v", items[2])
	}
	if items[3].Prefix != "assistant" || items[3].Text != "done" {
		t.Fatalf("unexpected assistant item: %#v", items[3])
	}
}

func TestApplyAgentEventStreamsAssistantWithoutDuplicateFinalText(t *testing.T) {
	m := newModel(context.Background(), &fakeRuntime{}, Options{})

	m.applyAgentEvent(agent.Event{Type: agent.EventTypeProviderTextDelta, Delta: "pon"})
	m.applyAgentEvent(agent.Event{Type: agent.EventTypeProviderTextDelta, Delta: "g"})
	if got := len(m.items); got != 1 {
		t.Fatalf("expected one live buffer item, got %d", got)
	}
	if m.items[0].Kind != kindLiveBuffer || m.items[0].Text != "pong" {
		t.Fatalf("unexpected live buffer item: %#v", m.items[0])
	}

	msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("pong"))
	m.applyAgentEvent(agent.Event{Type: agent.EventTypeAssistantMessageComplete, Message: &msg})
	if got := len(m.items); got != 1 {
		t.Fatalf("expected one finalized assistant item, got %d", got)
	}
	if m.items[0].Kind != kindAssistant || m.items[0].Text != "pong" {
		t.Fatalf("unexpected final assistant item: %#v", m.items[0])
	}
}

func TestLoadSessionCmdReplaysTranscript(t *testing.T) {
	msgTime := time.Now().UTC()
	runtime := &fakeRuntime{
		replay: session.Session{
			ID:         "sess_replay",
			WorkingDir: "/tmp/project",
			Conversation: conversation.Conversation{Messages: []conversation.Message{
				{ID: "m1", Role: conversation.RoleUser, CreatedAt: msgTime, Content: []conversation.Content{conversation.Text("first")}},
				{ID: "m2", Role: conversation.RoleAssistant, CreatedAt: msgTime, Content: []conversation.Content{conversation.Text("second")}},
			}},
		},
	}

	m := newModel(context.Background(), runtime, Options{SessionID: "sess_replay"})
	msg := loadSessionCmd(context.Background(), runtime, "sess_replay")()
	updated, _ := m.Update(msg)
	got := updated.(model)

	if got.sessionID != "sess_replay" {
		t.Fatalf("expected session id to be loaded, got %q", got.sessionID)
	}
	if got.workingDir != "/tmp/project" {
		t.Fatalf("expected working dir to be loaded, got %q", got.workingDir)
	}
	if len(got.items) != 2 {
		t.Fatalf("expected 2 replayed transcript items, got %d", len(got.items))
	}
	if got.items[0].Text != "first" || got.items[1].Text != "second" {
		t.Fatalf("unexpected replayed transcript: %#v", got.items)
	}
}

func TestLoadSessionCmdDetectsPendingApproval(t *testing.T) {
	runtime := &fakeRuntime{
		replay: session.Session{ID: "sess_pending", WorkingDir: "/tmp/project"},
		pendingApproval: &agent.ApprovalRequest{SessionID: "sess_pending", ToolCall: tools.Call{
			ID:        "call_1",
			Name:      "shell",
			Arguments: json.RawMessage(`{"command":"pwd"}`),
		}},
	}

	m := newModel(context.Background(), runtime, Options{SessionID: "sess_pending"})
	updated, _ := m.Update(loadSessionCmd(context.Background(), runtime, "sess_pending")())
	m = updated.(model)

	if m.status != "awaiting approval" {
		t.Fatalf("expected awaiting approval status, got %q", m.status)
	}
	if m.approval.Request == nil || m.approval.Request.ToolCall.Name != "shell" {
		t.Fatalf("expected pending approval to be loaded, got %#v", m.approval)
	}
	if !strings.Contains(m.View(), "Approval required") {
		t.Fatalf("expected approval panel in view, got %q", m.View())
	}
}

func TestStartRunCmdStreamsEventsAndWritesTrace(t *testing.T) {
	runtime := &fakeRuntime{
		loadSession: session.Session{ID: "sess_run", WorkingDir: "/tmp/project"},
		trace:       &fakeTraceWriter{},
		streamEvents: []agent.Event{
			{Type: agent.EventTypeRunStarted, SessionID: "sess_run"},
			{Type: agent.EventTypeUserMessagePersisted, SessionID: "sess_run", Message: message(conversation.RoleUser, "ping")},
			{Type: agent.EventTypeProviderTextDelta, SessionID: "sess_run", Delta: "pon"},
			{Type: agent.EventTypeProviderTextDelta, SessionID: "sess_run", Delta: "g"},
			{Type: agent.EventTypeAssistantMessageComplete, SessionID: "sess_run", Message: message(conversation.RoleAssistant, "pong")},
			{Type: agent.EventTypeRunCompleted, SessionID: "sess_run", Result: &agent.Result{Status: agent.StatusCompleted}},
		},
	}

	m := newModel(context.Background(), runtime, Options{})
	startMsg := startRunCmd(context.Background(), runtime, m.async, "ping", "")()
	updated, _ := m.Update(startMsg)
	m = updated.(model)
	if !m.running || m.sessionID != "sess_run" {
		t.Fatalf("expected running model with session id, got running=%v session=%q", m.running, m.sessionID)
	}

	for range runtime.streamEvents {
		msg := <-m.async
		updated, cmd := m.Update(msg)
		m = updated.(model)
		if cmd != nil {
			_ = cmd
		}
	}

	if m.status != "completed" {
		t.Fatalf("expected completed status, got %q", m.status)
	}
	if m.running {
		t.Fatal("expected model to stop running after completion")
	}
	if len(runtime.trace.written) != len(runtime.streamEvents) {
		t.Fatalf("expected %d trace events, got %d", len(runtime.streamEvents), len(runtime.trace.written))
	}
	if !runtime.trace.closed {
		t.Fatal("expected trace writer to be closed after completion")
	}
	if !containsText(m.items, "user", "ping") {
		t.Fatalf("expected user message in transcript, got %#v", m.items)
	}
	if !containsText(m.items, "assistant", "pong") {
		t.Fatalf("expected assistant message in transcript, got %#v", m.items)
	}
}

func TestToolLifecycleAndInterruptUpdateState(t *testing.T) {
	trace := &fakeTraceWriter{}
	m := newModel(context.Background(), &fakeRuntime{}, Options{})
	m.trace = trace
	m.running = true
	cancelled := false
	m.cancelRun = func() { cancelled = true }

	toolCall := toolCall("call_1", "shell", `{"command":"pwd"}`)
	m.applyAgentEvent(agent.Event{Type: agent.EventTypeToolCallDetected, ToolCall: toolCall})
	m.applyAgentEvent(agent.Event{Type: agent.EventTypeToolExecutionStarted, ToolCall: toolCall})
	m.applyAgentEvent(agent.Event{Type: agent.EventTypeToolMessagePersisted, ToolCall: toolCall, ToolResult: toolResult("call_1", "/tmp/project")})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(model)
	if !cancelled {
		t.Fatal("expected ctrl+c to invoke cancel when running")
	}
	if m.status != "interrupting" {
		t.Fatalf("expected interrupting status, got %q", m.status)
	}

	m.applyAgentEvent(agent.Event{Type: agent.EventTypeRunInterrupted, Err: context.Canceled})
	if m.status != "interrupted" {
		t.Fatalf("expected interrupted status, got %q", m.status)
	}
	if !containsText(m.items, "assistant requested tool", "shell {") {
		t.Fatalf("expected tool request item, got %#v", m.items)
	}
	if !containsText(m.items, "tool[shell]", "/tmp/project") {
		t.Fatalf("expected tool result item, got %#v", m.items)
	}
	if !trace.closed {
		t.Fatal("expected trace to be closed after interruption")
	}
}

func TestApprovalKeyApproveStartsContinuation(t *testing.T) {
	runtime := &fakeRuntime{
		trace: &fakeTraceWriter{},
		resolveEvents: []agent.Event{
			{Type: agent.EventTypeRunStarted, SessionID: "sess_pending"},
			{Type: agent.EventTypeApprovalResolved, SessionID: "sess_pending", ApprovalDecision: agent.ApprovalDecisionAllow, ApprovalRequest: &agent.ApprovalRequest{SessionID: "sess_pending", ToolCall: tools.Call{ID: "call_1", Name: "shell", Arguments: json.RawMessage(`{"command":"pwd"}`)}}},
			{Type: agent.EventTypeToolExecutionStarted, SessionID: "sess_pending", ToolCall: toolCall("call_1", "shell", `{"command":"pwd"}`)},
			{Type: agent.EventTypeToolMessagePersisted, SessionID: "sess_pending", ToolCall: toolCall("call_1", "shell", `{"command":"pwd"}`), ToolResult: toolResult("call_1", "/tmp/project")},
			{Type: agent.EventTypeAssistantMessageComplete, SessionID: "sess_pending", Message: message(conversation.RoleAssistant, "done")},
			{Type: agent.EventTypeRunCompleted, SessionID: "sess_pending", Result: &agent.Result{Status: agent.StatusCompleted}},
		},
	}
	m := newModel(context.Background(), runtime, Options{})
	m.sessionID = "sess_pending"
	m.workingDir = "/tmp/project"
	m.status = "awaiting approval"
	m.approval.Request = &agent.ApprovalRequest{SessionID: "sess_pending", ToolCall: tools.Call{ID: "call_1", Name: "shell", Arguments: json.RawMessage(`{"command":"pwd"}`)}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(model)
	if !m.approval.Busy || m.status != "resolving approval" {
		t.Fatalf("expected busy resolving approval state, got %#v", m.approval)
	}
	if cmd == nil {
		t.Fatal("expected resolve command")
	}
	updated, _ = m.Update(cmd())
	m = updated.(model)
	if !m.running {
		t.Fatal("expected running state after approval start")
	}
	for range runtime.resolveEvents {
		updated, _ = m.Update(<-m.async)
		m = updated.(model)
	}
	if m.status != "completed" {
		t.Fatalf("expected completed status, got %q", m.status)
	}
	if m.approval.Request != nil {
		t.Fatalf("expected approval panel to clear, got %#v", m.approval)
	}
	if !containsText(m.items, "tool[shell]", "/tmp/project") || !containsText(m.items, "assistant", "done") {
		t.Fatalf("expected continued transcript, got %#v", m.items)
	}
}

func TestApprovalKeyDenyStartsContinuation(t *testing.T) {
	runtime := &fakeRuntime{
		trace: &fakeTraceWriter{},
		resolveEvents: []agent.Event{
			{Type: agent.EventTypeRunStarted, SessionID: "sess_pending"},
			{Type: agent.EventTypeApprovalResolved, SessionID: "sess_pending", ApprovalDecision: agent.ApprovalDecisionDeny, ApprovalRequest: &agent.ApprovalRequest{SessionID: "sess_pending", ToolCall: tools.Call{ID: "call_1", Name: "shell", Arguments: json.RawMessage(`{"command":"pwd"}`)}}},
			{Type: agent.EventTypeToolMessagePersisted, SessionID: "sess_pending", ToolCall: toolCall("call_1", "shell", `{"command":"pwd"}`), ToolResult: toolErrorResult("call_1", "tool execution denied by user")},
			{Type: agent.EventTypeAssistantMessageComplete, SessionID: "sess_pending", Message: message(conversation.RoleAssistant, "denied")},
			{Type: agent.EventTypeRunCompleted, SessionID: "sess_pending", Result: &agent.Result{Status: agent.StatusCompleted}},
		},
	}
	m := newModel(context.Background(), runtime, Options{})
	m.sessionID = "sess_pending"
	m.status = "awaiting approval"
	m.approval.Request = &agent.ApprovalRequest{SessionID: "sess_pending", ToolCall: tools.Call{ID: "call_1", Name: "shell", Arguments: json.RawMessage(`{"command":"pwd"}`)}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(model)
	if cmd == nil {
		t.Fatal("expected resolve command")
	}
	updated, _ = m.Update(cmd())
	m = updated.(model)
	for range runtime.resolveEvents {
		updated, _ = m.Update(<-m.async)
		m = updated.(model)
	}
	if m.status != "completed" {
		t.Fatalf("expected completed status, got %q", m.status)
	}
	if !containsText(m.items, "tool[shell]", "tool execution denied by user") {
		t.Fatalf("expected denied tool result in transcript, got %#v", m.items)
	}
}

func TestStartRunCmdReturnsErrorMessage(t *testing.T) {
	runtime := &fakeRuntime{loadErr: errors.New("boom")}
	msg := startRunCmd(context.Background(), runtime, make(chan tea.Msg, 1), "ping", "")()
	if _, ok := msg.(runStartFailedMsg); !ok {
		t.Fatalf("expected runStartFailedMsg, got %T", msg)
	}
}

func TestEnterModelCommandAppendsLocalTranscript(t *testing.T) {
	m := newModel(context.Background(), &fakeRuntime{}, Options{})
	m.input.SetValue("/model")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	if m.status != "idle" {
		t.Fatalf("expected idle status after local command, got %q", m.status)
	}
	if len(m.items) != 2 {
		t.Fatalf("expected 2 transcript items for local command, got %d", len(m.items))
	}
	if m.items[0].Text != "/model" {
		t.Fatalf("expected command echo in transcript, got %#v", m.items[0])
	}
	if !strings.Contains(m.items[1].Text, "provider: openai-codex") || !strings.Contains(m.items[1].Text, "model: gpt-5-codex") {
		t.Fatalf("unexpected local command output: %#v", m.items[1])
	}
}

func message(role conversation.Role, text string) *conversation.Message {
	msg := conversation.NewMessage(role, conversation.Text(text))
	return &msg
}

func toolCall(id, name, args string) *tools.Call {
	return &tools.Call{
		ID:        id,
		Name:      name,
		Arguments: json.RawMessage(args),
	}
}

func toolResult(id, text string) *tools.Result {
	return &tools.Result{
		ToolCallID: id,
		Content:    []conversation.ToolResult{{Type: "text", Text: text}},
	}
}

func toolErrorResult(id, text string) *tools.Result {
	return &tools.Result{
		ToolCallID: id,
		IsError:    true,
		Content:    []conversation.ToolResult{{Type: "text", Text: text}},
	}
}

func containsText(items []transcriptItem, prefix string, text string) bool {
	for _, item := range items {
		if item.Prefix == prefix && strings.Contains(item.Text, text) {
			return true
		}
	}
	return false
}
