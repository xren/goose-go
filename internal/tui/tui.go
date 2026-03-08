package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"goose-go/internal/agent"
	"goose-go/internal/app"
	"goose-go/internal/conversation"
	"goose-go/internal/session"
)

type Runtime interface {
	LoadOrCreateSession(ctx context.Context, prompt string, sessionID string) (session.Session, int, error)
	ReplayConversation(ctx context.Context, sessionID string) (session.Session, error)
	OpenTraceWriter(sessionID string) (app.EventRecorder, error)
	ReplyStream(ctx context.Context, sessionID string, prompt string) (<-chan agent.Event, error)
	WorkingDir() string
}

type Options struct {
	SessionID string
}

type itemKind string

const (
	kindUser       itemKind = "user"
	kindAssistant  itemKind = "assistant"
	kindTool       itemKind = "tool"
	kindSystem     itemKind = "system"
	kindError      itemKind = "error"
	kindLiveBuffer itemKind = "live_buffer"
)

type transcriptItem struct {
	Kind   itemKind
	Prefix string
	Text   string
	Key    string
	Meta   string
}

type model struct {
	ctx        context.Context
	runtime    Runtime
	opts       Options
	input      textinput.Model
	viewport   viewport.Model
	width      int
	height     int
	status     string
	errorText  string
	sessionID  string
	workingDir string
	items      []transcriptItem
	async      chan tea.Msg
	running    bool
	cancelRun  context.CancelFunc
	trace      app.EventRecorder
}

type sessionLoadedMsg struct {
	session session.Session
}

type sessionLoadFailedMsg struct{ err error }

type runStartedMsg struct {
	session session.Session
	trace   app.EventRecorder
	cancel  context.CancelFunc
}

type runStartFailedMsg struct{ err error }

type agentEventMsg struct{ event agent.Event }

type traceWriteFailedMsg struct{ err error }

type noOpMsg struct{}

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 1)
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Padding(0, 1)
)

func Run(ctx context.Context, in io.Reader, out io.Writer, runtime Runtime, opts Options) error {
	m := newModel(ctx, runtime, opts)
	p := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newModel(ctx context.Context, runtime Runtime, opts Options) model {
	input := textinput.New()
	input.Placeholder = "Ask goose-go"
	input.Focus()
	input.Prompt = "> "
	input.CharLimit = 0

	vp := viewport.New(0, 0)
	vp.SetContent("")

	return model{
		ctx:        ctx,
		runtime:    runtime,
		opts:       opts,
		input:      input,
		viewport:   vp,
		status:     "idle",
		workingDir: runtime.WorkingDir(),
		async:      make(chan tea.Msg, 128),
	}
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{waitForAsync(m.async)}
	if m.opts.SessionID != "" {
		cmds = append(cmds, loadSessionCmd(m.ctx, m.runtime, m.opts.SessionID))
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.running && m.cancelRun != nil {
				m.cancelRun()
				m.status = "interrupting"
				return m, nil
			}
			return m, tea.Quit
		case "esc":
			if m.running && m.cancelRun != nil {
				m.cancelRun()
				m.status = "interrupting"
			}
			return m, nil
		case "enter":
			if m.running {
				return m, nil
			}
			prompt := strings.TrimSpace(m.input.Value())
			if prompt == "" {
				return m, nil
			}
			m.errorText = ""
			m.status = "starting"
			m.input.SetValue("")
			return m, startRunCmd(m.ctx, m.runtime, m.async, prompt, m.sessionID)
		case "ctrl+d":
			if !m.running {
				return m, tea.Quit
			}
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	case sessionLoadedMsg:
		m.sessionID = msg.session.ID
		m.workingDir = msg.session.WorkingDir
		m.items = buildTranscriptFromConversation(msg.session.Conversation)
		m.status = "idle"
		m.syncViewport()
		return m, nil
	case sessionLoadFailedMsg:
		m.status = "failed"
		m.errorText = msg.err.Error()
		return m, nil
	case runStartedMsg:
		m.sessionID = msg.session.ID
		m.workingDir = msg.session.WorkingDir
		m.trace = msg.trace
		m.cancelRun = msg.cancel
		m.running = true
		m.status = "running"
		m.syncViewport()
		return m, nil
	case runStartFailedMsg:
		m.status = "failed"
		m.errorText = msg.err.Error()
		return m, nil
	case agentEventMsg:
		m.applyAgentEvent(msg.event)
		return m, waitForAsync(m.async)
	case traceWriteFailedMsg:
		m.status = "failed"
		m.errorText = msg.err.Error()
		return m, waitForAsync(m.async)
	case noOpMsg:
		return m, waitForAsync(m.async)
	case error:
		m.status = "failed"
		m.errorText = msg.Error()
		return m, nil
	default:
		return m, waitForAsync(m.async)
	}
}

func (m model) View() string {
	header := headerStyle.Render(fmt.Sprintf("session: %s", fallback(m.sessionID, "new")))
	cwd := statusStyle.Render(fmt.Sprintf("cwd: %s", fallback(m.workingDir, "-")))
	status := statusStyle.Render(fmt.Sprintf("status: %s", m.status))
	if m.errorText != "" {
		status += "\n" + errorStyle.Render(m.errorText)
	}
	footer := statusStyle.Render("enter submit  esc/ctrl+c interrupt  ctrl+d quit")
	return lipgloss.JoinVertical(lipgloss.Left, header, cwd, status, m.viewport.View(), m.input.View(), footer)
}

func (m *model) layout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	headerLines := 3
	footerLines := 2
	composerLines := 1
	bodyHeight := m.height - headerLines - footerLines - composerLines
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	m.viewport.Width = m.width
	m.viewport.Height = bodyHeight
	m.syncViewport()
}

func (m *model) syncViewport() {
	m.viewport.SetContent(renderItems(m.items, m.viewport.Width))
	m.viewport.GotoBottom()
}

func (m *model) applyAgentEvent(event agent.Event) {
	if m.trace != nil {
		if err := m.trace.Write(event); err != nil {
			select {
			case m.async <- traceWriteFailedMsg{err: fmt.Errorf("write trace event: %w", err)}:
			default:
			}
		}
	}

	switch event.Type {
	case agent.EventTypeRunStarted:
		m.status = "running"
	case agent.EventTypeUserMessagePersisted:
		if event.Message != nil {
			appendMessageItems(&m.items, *event.Message)
		}
	case agent.EventTypeProviderTextDelta:
		m.upsertLiveAssistant(event.Delta)
	case agent.EventTypeAssistantMessageComplete:
		m.clearLiveAssistant()
		if event.Message != nil {
			appendMessageItems(&m.items, *event.Message)
		}
	case agent.EventTypeToolCallDetected:
		if event.ToolCall != nil {
			m.items = append(m.items, transcriptItem{
				Kind:   kindSystem,
				Prefix: "assistant requested tool",
				Text:   fmt.Sprintf("%s %s", event.ToolCall.Name, compactArgs(event.ToolCall.Arguments)),
			})
		}
	case agent.EventTypeToolExecutionStarted:
		if event.ToolCall != nil {
			m.items = append(m.items, transcriptItem{Kind: kindSystem, Prefix: "tool", Text: fmt.Sprintf("running %s", event.ToolCall.Name)})
		}
	case agent.EventTypeToolMessagePersisted:
		if event.ToolResult != nil {
			name := "tool"
			if event.ToolCall != nil && event.ToolCall.Name != "" {
				name = fmt.Sprintf("tool[%s]", event.ToolCall.Name)
			}
			for _, part := range event.ToolResult.Content {
				m.items = append(m.items, transcriptItem{Kind: kindTool, Prefix: name, Text: part.Text})
			}
		}
	case agent.EventTypeCompactionStarted:
		m.items = append(m.items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: fmt.Sprintf("compacting context (%s, %d tokens)", event.CompactionReason, event.TokensBefore)})
	case agent.EventTypeCompactionCompleted:
		m.items = append(m.items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: fmt.Sprintf("compaction complete (%s)", event.CompactionReason)})
	case agent.EventTypeCompactionFailed:
		m.items = append(m.items, transcriptItem{Kind: kindError, Prefix: "system", Text: fmt.Sprintf("compaction failed (%s)", event.CompactionReason)})
	case agent.EventTypeApprovalRequired:
		m.status = "awaiting approval"
		m.items = append(m.items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: "approval required (interactive approval UI is not available in Stage 1)"})
	case agent.EventTypeRunCompleted:
		m.finishRun(runtimeResultStatus(event.Result))
	case agent.EventTypeRunInterrupted:
		m.items = append(m.items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: "interrupted"})
		m.finishRun("interrupted")
	case agent.EventTypeRunFailed:
		m.items = append(m.items, transcriptItem{Kind: kindError, Prefix: "error", Text: errorText(event.Err)})
		m.finishRun("failed")
	}
	m.syncViewport()
}

func (m *model) finishRun(status string) {
	m.running = false
	m.cancelRun = nil
	m.status = status
	if m.trace != nil {
		_ = m.trace.Close()
		m.trace = nil
	}
}

func (m *model) upsertLiveAssistant(delta string) {
	if len(m.items) > 0 && m.items[len(m.items)-1].Kind == kindLiveBuffer {
		m.items[len(m.items)-1].Text += delta
		return
	}
	m.items = append(m.items, transcriptItem{Kind: kindLiveBuffer, Prefix: "assistant", Text: delta})
}

func (m *model) clearLiveAssistant() {
	if len(m.items) > 0 && m.items[len(m.items)-1].Kind == kindLiveBuffer {
		m.items = m.items[:len(m.items)-1]
	}
}

func loadSessionCmd(ctx context.Context, runtime Runtime, sessionID string) tea.Cmd {
	return func() tea.Msg {
		session, err := runtime.ReplayConversation(ctx, sessionID)
		if err != nil {
			return sessionLoadFailedMsg{err: err}
		}
		return sessionLoadedMsg{session: session}
	}
}

func startRunCmd(ctx context.Context, runtime Runtime, async chan tea.Msg, prompt string, sessionID string) tea.Cmd {
	return func() tea.Msg {
		record, _, err := runtime.LoadOrCreateSession(ctx, prompt, sessionID)
		if err != nil {
			return runStartFailedMsg{err: err}
		}
		trace, err := runtime.OpenTraceWriter(record.ID)
		if err != nil {
			return runStartFailedMsg{err: err}
		}
		runCtx, cancel := context.WithCancel(ctx)
		stream, err := runtime.ReplyStream(runCtx, record.ID, prompt)
		if err != nil {
			_ = trace.Close()
			cancel()
			return runStartFailedMsg{err: err}
		}
		go bridgeStream(async, runCtx, stream)
		return runStartedMsg{session: record, trace: trace, cancel: cancel}
	}
}

func bridgeStream(async chan tea.Msg, ctx context.Context, stream <-chan agent.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-stream:
			if !ok {
				return
			}
			async <- agentEventMsg{event: event}
		}
	}
}

func waitForAsync(async <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-async
		if !ok {
			return noOpMsg{}
		}
		return msg
	}
}

func buildTranscriptFromConversation(conv conversation.Conversation) []transcriptItem {
	items := make([]transcriptItem, 0, len(conv.Messages))
	for _, message := range conv.Messages {
		appendMessageItems(&items, message)
	}
	return items
}

func appendMessageItems(items *[]transcriptItem, message conversation.Message) {
	for _, content := range message.Content {
		switch content.Type {
		case conversation.ContentTypeText:
			if content.Text == nil {
				continue
			}
			prefix := string(message.Role)
			kind := kindSystem
			switch message.Role {
			case conversation.RoleUser:
				kind = kindUser
			case conversation.RoleAssistant:
				kind = kindAssistant
			case conversation.RoleTool:
				kind = kindTool
			}
			*items = append(*items, transcriptItem{Kind: kind, Prefix: prefix, Text: content.Text.Text})
		case conversation.ContentTypeToolRequest:
			if content.ToolRequest == nil {
				continue
			}
			*items = append(*items, transcriptItem{Kind: kindSystem, Prefix: "assistant requested tool", Text: fmt.Sprintf("%s %s", content.ToolRequest.Name, compactArgs(content.ToolRequest.Arguments))})
		case conversation.ContentTypeToolResponse:
			if content.ToolResponse == nil {
				continue
			}
			for _, result := range content.ToolResponse.Content {
				*items = append(*items, transcriptItem{Kind: kindTool, Prefix: "tool", Text: result.Text})
			}
		case conversation.ContentTypeSystemNotification:
			if content.SystemNotification == nil {
				continue
			}
			*items = append(*items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: content.SystemNotification.Message})
		}
	}
}

func renderItems(items []transcriptItem, width int) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		prefix := item.Prefix
		if prefix == "" {
			prefix = string(item.Kind)
		}
		text := strings.TrimRight(item.Text, "\n")
		if text == "" {
			text = ""
		}
		parts := strings.Split(text, "\n")
		for i, part := range parts {
			if i == 0 {
				lines = append(lines, fmt.Sprintf("%s> %s", prefix, part))
				continue
			}
			lines = append(lines, fmt.Sprintf("%s  %s", strings.Repeat(" ", len(prefix)), part))
		}
		if len(parts) == 0 {
			lines = append(lines, prefix+"> ")
		}
	}
	content := strings.Join(lines, "\n")
	if width > 0 {
		return lipgloss.NewStyle().Width(width).Render(content)
	}
	return content
}

func compactArgs(raw json.RawMessage) string {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return "{}"
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return string(raw)
	}
	return string(data)
}

func runtimeResultStatus(result *agent.Result) string {
	if result == nil {
		return "idle"
	}
	return appRuntimeStatus(result.Status)
}

func appRuntimeStatus(status agent.Status) string {
	switch status {
	case agent.StatusCompleted:
		return "completed"
	case agent.StatusAwaitingApproval:
		return "awaiting approval"
	default:
		if strings.TrimSpace(string(status)) == "" {
			return "idle"
		}
		return string(status)
	}
}

func errorText(err error) string {
	if err == nil {
		return "run failed"
	}
	var diag *app.DiagnosticError
	if errors.As(err, &diag) {
		return diag.Error()
	}
	if errors.Is(err, agent.ErrMaxTurnsExceeded) {
		return "max turns exceeded"
	}
	return err.Error()
}

func fallback(value string, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
