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
	"goose-go/internal/models"
	"goose-go/internal/session"
	"goose-go/internal/tools"
)

type fakeRuntime struct {
	workingDir         string
	loadSession        session.Session
	loadErr            error
	replay             session.Session
	replayErr          error
	sessionSummaries   []session.Summary
	sessionSummaryErr  error
	trace              *fakeTraceWriter
	traceErr           error
	streamEvents       []agent.Event
	streamErr          error
	pendingApproval    *agent.ApprovalRequest
	pendingApprovalErr error
	resolveEvents      []agent.Event
	resolveErr         error
	availableModels    []models.Availability
	availableModelsErr error
	setSelectionErr    error
	providerName       string
	modelName          string
	lastSetProvider    string
	lastSetModel       string
	lastSetSessionID   string
}

type fakeTraceWriter struct {
	written []agent.Event
	closed  bool
}

type capturePrinter struct {
	blocks []string
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

func (f *fakeRuntime) ListSessions(context.Context) ([]session.Summary, error) {
	if f.sessionSummaryErr != nil {
		return nil, f.sessionSummaryErr
	}
	return f.sessionSummaries, nil
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
	providerName := f.providerName
	modelName := f.modelName
	if providerName == "" {
		providerName = "openai-codex"
	}
	if modelName == "" {
		modelName = "gpt-5-codex"
	}
	return providerName, modelName
}

func (f *fakeRuntime) ListAvailableModels(context.Context) ([]models.Availability, error) {
	if f.availableModelsErr != nil {
		return nil, f.availableModelsErr
	}
	return f.availableModels, nil
}

func (f *fakeRuntime) SetSelection(_ context.Context, provider string, model string, sessionID string) error {
	if f.setSelectionErr != nil {
		return f.setSelectionErr
	}
	f.providerName = provider
	f.modelName = model
	f.lastSetProvider = provider
	f.lastSetModel = model
	f.lastSetSessionID = sessionID
	return nil
}

func (f *fakeTraceWriter) Write(event agent.Event) error {
	f.written = append(f.written, event)
	return nil
}

func (f *fakeTraceWriter) Close() error {
	f.closed = true
	return nil
}

func (c *capturePrinter) Cmd(blocks ...string) tea.Cmd {
	c.blocks = append(c.blocks, blocks...)
	return nil
}

func newCaptureModel(t *testing.T, runtime *fakeRuntime, opts Options) (model, *capturePrinter) {
	t.Helper()
	m := newModel(context.Background(), runtime, opts)
	capture := &capturePrinter{}
	m.printer = capture
	return m, capture
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
	if len(items) != 3 {
		t.Fatalf("expected 3 transcript items, got %d", len(items))
	}
	if items[0].Prefix != "user" || items[0].Text != "hello" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].Prefix != "tool[shell]" {
		t.Fatalf("unexpected tool group item: %#v", items[1])
	}
	if !strings.Contains(items[1].Text, "status: completed") || !strings.Contains(items[1].Text, "/tmp/work") {
		t.Fatalf("unexpected tool group text: %#v", items[1])
	}
	if items[2].Prefix != "assistant" || items[2].Text != "done" {
		t.Fatalf("unexpected assistant item: %#v", items[2])
	}
}

func TestApplyAgentEventUsesPreviewAndPrintsFinalAssistantOnce(t *testing.T) {
	m, printer := newCaptureModel(t, &fakeRuntime{}, Options{Debug: true})

	cmd := m.applyAgentEvent(agent.Event{Type: agent.EventTypeProviderTextDelta, Delta: "pon"})
	if cmd != nil {
		t.Fatal("did not expect print command for provider delta")
	}
	_ = m.applyAgentEvent(agent.Event{Type: agent.EventTypeProviderTextDelta, Delta: "g"})
	if m.liveAssistant != "pong" {
		t.Fatalf("expected live preview buffer, got %q", m.liveAssistant)
	}
	if len(printer.blocks) != 0 {
		t.Fatalf("did not expect printed output during delta streaming, got %#v", printer.blocks)
	}

	msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("pong"))
	cmd = m.applyAgentEvent(agent.Event{Type: agent.EventTypeAssistantMessageComplete, Message: &msg})
	_ = cmd
	if m.liveAssistant != "" {
		t.Fatalf("expected live preview to clear, got %q", m.liveAssistant)
	}
	if !containsPrinted(printer.blocks, "pong") {
		t.Fatalf("expected finalized assistant output to be printed, got %#v", printer.blocks)
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

	m, printer := newCaptureModel(t, runtime, Options{SessionID: "sess_replay"})
	msg := loadSessionCmd(context.Background(), runtime, "sess_replay")()
	updated, cmd := m.Update(msg)
	m = updated.(model)
	_ = cmd

	if m.sessionID != "sess_replay" {
		t.Fatalf("expected session id to be loaded, got %q", m.sessionID)
	}
	if m.workingDir != "/tmp/project" {
		t.Fatalf("expected working dir to be loaded, got %q", m.workingDir)
	}
	if !containsPrinted(printer.blocks, "first") || !containsPrinted(printer.blocks, "second") {
		t.Fatalf("expected replayed transcript to be printed, got %#v", printer.blocks)
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

	m, _ := newCaptureModel(t, runtime, Options{SessionID: "sess_pending"})
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

	m, printer := newCaptureModel(t, runtime, Options{Debug: true})
	startMsg := startRunCmd(context.Background(), runtime, m.async, "ping", "")()
	updated, _ := m.Update(startMsg)
	m = updated.(model)
	if !m.running || m.sessionID != "sess_run" {
		t.Fatalf("expected running model with session id, got running=%v session=%q", m.running, m.sessionID)
	}

	for range runtime.streamEvents {
		msg := <-m.async
		updated, _ = m.Update(msg)
		m = updated.(model)
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
	if !containsPrinted(printer.blocks, "ping") || !containsPrinted(printer.blocks, "pong") {
		t.Fatalf("expected printed user and assistant output, got %#v", printer.blocks)
	}
}

func TestToolLifecycleAndInterruptPrintState(t *testing.T) {
	trace := &fakeTraceWriter{}
	m, printer := newCaptureModel(t, &fakeRuntime{}, Options{Debug: true})
	m.trace = trace
	m.running = true
	cancelled := false
	m.cancelRun = func() { cancelled = true }

	toolCall := toolCall("call_1", "shell", `{"command":"pwd"}`)
	if cmd := m.applyAgentEvent(agent.Event{Type: agent.EventTypeToolCallDetected, ToolCall: toolCall}); cmd != nil {
		_ = cmd
	}
	if cmd := m.applyAgentEvent(agent.Event{Type: agent.EventTypeToolExecutionStarted, ToolCall: toolCall}); cmd != nil {
		_ = cmd
	}
	if cmd := m.applyAgentEvent(agent.Event{Type: agent.EventTypeToolMessagePersisted, ToolCall: toolCall, ToolResult: toolResult("call_1", "/tmp/project")}); cmd != nil {
		_ = cmd
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(model)
	if !cancelled {
		t.Fatal("expected ctrl+c to invoke cancel when running")
	}
	if m.status != "interrupting" {
		t.Fatalf("expected interrupting status, got %q", m.status)
	}

	if cmd := m.applyAgentEvent(agent.Event{Type: agent.EventTypeRunInterrupted, Err: context.Canceled}); cmd != nil {
		_ = cmd
	}
	if m.status != "interrupted" {
		t.Fatalf("expected interrupted status, got %q", m.status)
	}
	if !containsPrinted(printer.blocks, "/tmp/project") {
		t.Fatalf("expected tool result in printed output, got %#v", printer.blocks)
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
	m, printer := newCaptureModel(t, runtime, Options{Debug: true})
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
	if !containsPrinted(printer.blocks, "/tmp/project") || !containsPrinted(printer.blocks, "done") {
		t.Fatalf("expected continuation output, got %#v", printer.blocks)
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
	m, printer := newCaptureModel(t, runtime, Options{Debug: true})
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
	if !containsPrinted(printer.blocks, "tool execution denied by user") {
		t.Fatalf("expected denied tool result in printed output, got %#v", printer.blocks)
	}
}

func TestStartRunCmdReturnsErrorMessage(t *testing.T) {
	runtime := &fakeRuntime{loadErr: errors.New("boom")}
	msg := startRunCmd(context.Background(), runtime, make(chan tea.Msg, 1), "ping", "")()
	if _, ok := msg.(runStartFailedMsg); !ok {
		t.Fatalf("expected runStartFailedMsg, got %T", msg)
	}
}

func TestEnterModelCommandOpensPicker(t *testing.T) {
	runtime := &fakeRuntime{
		providerName: "openai-codex",
		modelName:    "gpt-5-codex",
		availableModels: []models.Availability{
			{Model: models.ModelSpec{Provider: models.ProviderOpenAICodex, ID: models.ModelGPT5Codex, DisplayName: "GPT-5 Codex"}, Available: true},
			{Model: models.ModelSpec{Provider: models.ProviderOpenAICodex, ID: models.ModelGPT53Codex, DisplayName: "GPT-5.3 Codex"}, Available: true},
		},
	}
	m, _ := newCaptureModel(t, runtime, Options{})
	m.input.SetValue("/model")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if cmd == nil {
		t.Fatal("expected load models command")
	}
	updated, _ = m.Update(cmd())
	m = updated.(model)

	if m.status != "select model" {
		t.Fatalf("expected select model status, got %q", m.status)
	}
	if !m.picker.Open {
		t.Fatal("expected picker to be open")
	}
	if m.picker.Selected != 0 {
		t.Fatalf("expected current model to be preselected, got %d", m.picker.Selected)
	}
	if !strings.Contains(m.View(), "Select model") {
		t.Fatalf("expected picker in view, got %q", m.View())
	}
}

func TestModelPickerSelectsModelAndPersistsSession(t *testing.T) {
	runtime := &fakeRuntime{
		providerName: "openai-codex",
		modelName:    "gpt-5-codex",
		availableModels: []models.Availability{
			{Model: models.ModelSpec{Provider: models.ProviderOpenAICodex, ID: models.ModelGPT5Codex, DisplayName: "GPT-5 Codex"}, Available: true},
			{Model: models.ModelSpec{Provider: models.ProviderOpenAICodex, ID: models.ModelGPT53Codex, DisplayName: "GPT-5.3 Codex"}, Available: true},
		},
	}
	m, printer := newCaptureModel(t, runtime, Options{})
	m.sessionID = "sess_model"
	m.picker = modelPickerState{Open: true, Items: runtime.availableModels, Selected: 1}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if cmd == nil {
		t.Fatal("expected set model command")
	}
	updated, _ = m.Update(cmd())
	m = updated.(model)

	if m.picker.Open {
		t.Fatal("expected picker to close after successful model change")
	}
	if runtime.lastSetProvider != "openai-codex" || runtime.lastSetModel != string(models.ModelGPT53Codex) {
		t.Fatalf("unexpected set selection call: provider=%q model=%q", runtime.lastSetProvider, runtime.lastSetModel)
	}
	if runtime.lastSetSessionID != "sess_model" {
		t.Fatalf("expected session id to be persisted, got %q", runtime.lastSetSessionID)
	}
	if !containsPrinted(printer.blocks, "selected model: gpt-5.3-codex") {
		t.Fatalf("expected selection output to be printed, got %#v", printer.blocks)
	}
}

func TestModelPickerUnavailableModelShowsReason(t *testing.T) {
	runtime := &fakeRuntime{
		availableModels: []models.Availability{
			{
				Model:     models.ModelSpec{Provider: models.ProviderOpenAICodex, ID: models.ModelGPT54Codex, DisplayName: "GPT-5.4"},
				Available: false,
				Reason:    "missing Codex auth",
			},
		},
	}
	m, _ := newCaptureModel(t, runtime, Options{})
	m.picker = modelPickerState{Open: true, Items: runtime.availableModels}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if cmd != nil {
		t.Fatal("did not expect set model command for unavailable item")
	}
	if m.picker.Err != "missing Codex auth" {
		t.Fatalf("expected unavailable reason, got %q", m.picker.Err)
	}
}

func TestSessionsCommandOpensPicker(t *testing.T) {
	runtime := &fakeRuntime{
		sessionSummaries: []session.Summary{
			{ID: "sess_recent", Name: "Recent session", WorkingDir: "/tmp/project", Provider: "openai-codex", Model: "gpt-5-codex", MessageCount: 3},
			{ID: "sess_old", Name: "Older session", WorkingDir: "/tmp/old", Provider: "openai-codex", Model: "gpt-5.3-codex", MessageCount: 8},
		},
	}
	m, _ := newCaptureModel(t, runtime, Options{})
	m.input.SetValue("/sessions")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if cmd == nil {
		t.Fatal("expected load sessions command")
	}
	updated, _ = m.Update(cmd())
	m = updated.(model)

	if !m.sessions.Open {
		t.Fatal("expected sessions picker to be open")
	}
	if m.status != "select session" {
		t.Fatalf("expected select session status, got %q", m.status)
	}
	if !strings.Contains(m.View(), "Recent sessions") {
		t.Fatalf("expected session picker in view, got %q", m.View())
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

func containsPrinted(blocks []string, text string) bool {
	for _, block := range blocks {
		if strings.Contains(block, text) {
			return true
		}
	}
	return false
}
