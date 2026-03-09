package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"goose-go/internal/models"
	tuitheme "goose-go/internal/tui/theme"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"goose-go/internal/agent"
	"goose-go/internal/app"
	"goose-go/internal/conversation"
	"goose-go/internal/session"
)

type Runtime interface {
	LoadOrCreateSession(ctx context.Context, prompt string, sessionID string) (session.Session, int, error)
	ReplayConversation(ctx context.Context, sessionID string) (session.Session, error)
	ListSessions(ctx context.Context) ([]session.Summary, error)
	OpenTraceWriter(sessionID string) (app.EventRecorder, error)
	ReplyStream(ctx context.Context, sessionID string, prompt string) (<-chan agent.Event, error)
	PendingApproval(ctx context.Context, sessionID string) (*agent.ApprovalRequest, error)
	ResolveApprovalStream(ctx context.Context, sessionID string, decision agent.ApprovalDecision) (<-chan agent.Event, error)
	ListAvailableModels(ctx context.Context) ([]models.Availability, error)
	SetSelection(ctx context.Context, provider string, model string, sessionID string) error
	WorkingDir() string
	ProviderModel() (string, string)
}

type Options struct {
	SessionID string
	Theme     string
	Debug     bool
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

type approvalViewState struct {
	Request *agent.ApprovalRequest
	Busy    bool
	Err     string
}

type modelPickerState struct {
	Open     bool
	Items    []models.Availability
	Selected int
	Busy     bool
	Err      string
}

type sessionPickerState struct {
	Open     bool
	Items    []session.Summary
	Selected int
	Busy     bool
	Err      string
}

type themePickerState struct {
	Open     bool
	Items    []tuitheme.Name
	Selected int
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
	approval   approvalViewState
	picker     modelPickerState
	sessions   sessionPickerState
	themes     themePickerState
	theme      tuitheme.Palette
	debug      bool
}

func (m model) canScrollTranscript() bool {
	return !m.sessions.Open && !m.picker.Open && !m.themes.Open && m.approval.Request == nil
}

func (m *model) handleViewportKey(msg tea.KeyMsg) bool {
	if !m.canScrollTranscript() {
		return false
	}
	switch msg.String() {
	case "pgup", "ctrl+u":
		m.viewport.HalfPageUp()
		return true
	case "pgdown":
		m.viewport.HalfPageDown()
		return true
	case "home":
		m.viewport.GotoTop()
		return true
	case "end":
		m.viewport.GotoBottom()
		return true
	case "ctrl+f":
		if m.running {
			return false
		}
		m.viewport.HalfPageDown()
		return true
	}
	return false
}

type modelsLoadedMsg struct{ items []models.Availability }
type modelsLoadFailedMsg struct{ err error }
type modelSetMsg struct{}
type modelSetFailedMsg struct{ err error }
type sessionsLoadedMsg struct{ items []session.Summary }
type sessionsLoadFailedMsg struct{ err error }

func Run(ctx context.Context, in io.Reader, out io.Writer, runtime Runtime, opts Options) error {
	m := newModel(ctx, runtime, opts)
	p := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

func newModel(ctx context.Context, runtime Runtime, opts Options) model {
	palette, err := tuitheme.Resolve(opts.Theme)
	if err != nil {
		palette, _ = tuitheme.Resolve("")
	}
	input := textinput.New()
	input.Placeholder = "Ask goose-go"
	input.Focus()
	input.Prompt = "> "
	input.CharLimit = 0

	vp := viewport.New(0, 0)
	vp.SetContent("")
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 4

	return model{
		ctx:        ctx,
		runtime:    runtime,
		opts:       opts,
		input:      input,
		viewport:   vp,
		status:     "idle",
		workingDir: runtime.WorkingDir(),
		async:      make(chan tea.Msg, 128),
		theme:      palette,
		debug:      opts.Debug,
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
	case tea.MouseMsg:
		if m.canScrollTranscript() {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		return m, nil
	case tea.KeyMsg:
		if m.sessions.Open {
			switch msg.String() {
			case "esc":
				m.sessions = sessionPickerState{}
				m.status = "idle"
				m.layout()
				return m, nil
			case "up", "k":
				if m.sessions.Selected > 0 {
					m.sessions.Selected--
				}
				return m, nil
			case "down", "j":
				if m.sessions.Selected < len(m.sessions.Items)-1 {
					m.sessions.Selected++
				}
				return m, nil
			case "enter":
				if m.sessions.Busy || len(m.sessions.Items) == 0 {
					return m, nil
				}
				item := m.sessions.Items[m.sessions.Selected]
				m.sessions.Busy = true
				m.sessions.Err = ""
				m.status = "loading session"
				return m, loadSessionCmd(m.ctx, m.runtime, item.ID)
			}
		}

		if m.picker.Open {
			switch msg.String() {
			case "esc":
				m.picker = modelPickerState{}
				m.status = "idle"
				m.layout()
				return m, nil
			case "up", "k":
				if m.picker.Selected > 0 {
					m.picker.Selected--
				}
				return m, nil
			case "down", "j":
				if m.picker.Selected < len(m.picker.Items)-1 {
					m.picker.Selected++
				}
				return m, nil
			case "enter":
				if m.picker.Busy || len(m.picker.Items) == 0 {
					return m, nil
				}
				item := m.picker.Items[m.picker.Selected]
				if !item.Available {
					m.picker.Err = item.Reason
					return m, nil
				}
				m.picker.Busy = true
				m.picker.Err = ""
				m.status = "switching model"
				providerName, _ := m.runtime.ProviderModel()
				return m, setModelCmd(m.ctx, m.runtime, providerName, string(item.Model.ID), m.sessionID)
			}
		}

		if m.themes.Open {
			switch msg.String() {
			case "esc":
				m.themes = themePickerState{}
				m.status = "idle"
				m.layout()
				return m, nil
			case "up", "k":
				if m.themes.Selected > 0 {
					m.themes.Selected--
				}
				return m, nil
			case "down", "j":
				if m.themes.Selected < len(m.themes.Items)-1 {
					m.themes.Selected++
				}
				return m, nil
			case "enter":
				if len(m.themes.Items) == 0 {
					return m, nil
				}
				selected := m.themes.Items[m.themes.Selected]
				palette, err := tuitheme.Resolve(string(selected))
				if err != nil {
					m.status = "failed"
					m.errorText = err.Error()
					return m, nil
				}
				m.theme = palette
				m.themes = themePickerState{}
				m.status = "idle"
				m.items = append(m.items,
					transcriptItem{Kind: kindSystem, Prefix: "system", Text: "/theme"},
					transcriptItem{Kind: kindSystem, Prefix: "system", Text: fmt.Sprintf("selected theme: %s", palette.Name)},
				)
				m.layout()
				m.syncViewport(true)
				return m, nil
			}
		}

		if m.approval.Request != nil {
			switch msg.String() {
			case "ctrl+c", "esc":
				if m.running && m.cancelRun != nil {
					m.cancelRun()
					m.status = "interrupting"
				}
				return m, nil
			case "a", "y":
				if m.approval.Busy || m.sessionID == "" {
					return m, nil
				}
				m.approval.Busy = true
				m.approval.Err = ""
				m.status = "resolving approval"
				return m, resolveApprovalCmd(m.ctx, m.runtime, m.async, m.sessionID, agent.ApprovalDecisionAllow)
			case "d", "n":
				if m.approval.Busy || m.sessionID == "" {
					return m, nil
				}
				m.approval.Busy = true
				m.approval.Err = ""
				m.status = "resolving approval"
				return m, resolveApprovalCmd(m.ctx, m.runtime, m.async, m.sessionID, agent.ApprovalDecisionDeny)
			}
		}

		switch msg.String() {
		case "pgup", "pgdown", "home", "end", "ctrl+u", "ctrl+f":
			if m.handleViewportKey(msg) {
				return m, nil
			}
		case "ctrl+r":
			if m.running || m.approval.Request != nil || m.picker.Open {
				return m, nil
			}
			m.errorText = ""
			return m, loadSessionsCmd(m.ctx, m.runtime)
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
			if m.running || m.approval.Request != nil || m.picker.Open || m.themes.Open {
				return m, nil
			}
			prompt := strings.TrimSpace(m.input.Value())
			if prompt == "" {
				return m, nil
			}
			m.errorText = ""
			m.input.SetValue("")
			if strings.TrimSpace(prompt) == "/sessions" {
				return m, loadSessionsCmd(m.ctx, m.runtime)
			}
			if strings.TrimSpace(prompt) == "/model" {
				m.errorText = ""
				return m, loadModelsCmd(m.ctx, m.runtime)
			}
			if m.handleLocalCommand(prompt) {
				m.syncViewport(true)
				return m, nil
			}
			m.status = "starting"
			return m, startRunCmd(m.ctx, m.runtime, m.async, prompt, m.sessionID)
		case "ctrl+d":
			if m.handleViewportKey(msg) {
				return m, nil
			}
			if !m.running {
				return m, tea.Quit
			}
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	case modelsLoadedMsg:
		providerName, modelName := m.runtime.ProviderModel()
		m.picker = modelPickerState{
			Open:     true,
			Items:    msg.items,
			Selected: selectedModelIndex(msg.items, providerName, modelName),
		}
		m.status = "select model"
		m.layout()
		return m, nil
	case sessionsLoadedMsg:
		m.sessions = sessionPickerState{
			Open:     true,
			Items:    msg.items,
			Selected: selectedSessionIndex(msg.items, m.sessionID),
		}
		m.status = "select session"
		m.layout()
		return m, nil
	case sessionsLoadFailedMsg:
		m.status = "failed"
		m.errorText = msg.err.Error()
		return m, nil
	case modelsLoadFailedMsg:
		m.status = "failed"
		m.errorText = msg.err.Error()
		return m, nil
	case modelSetMsg:
		providerName, modelName := m.runtime.ProviderModel()
		m.items = append(m.items,
			transcriptItem{Kind: kindSystem, Prefix: "system", Text: "/model"},
			transcriptItem{Kind: kindSystem, Prefix: "system", Text: fmt.Sprintf("selected provider: %s\nselected model: %s", providerName, modelName)},
		)
		m.picker = modelPickerState{}
		m.status = "idle"
		m.layout()
		m.syncViewport(true)
		return m, nil
	case modelSetFailedMsg:
		m.picker.Busy = false
		m.picker.Err = msg.err.Error()
		m.status = "select model"
		return m, nil
	case sessionLoadedMsg:
		m.sessions = sessionPickerState{}
		m.sessionID = msg.session.ID
		m.workingDir = msg.session.WorkingDir
		m.items = buildTranscriptFromConversation(msg.session.Conversation)
		m.approval.Request = msg.approval
		m.approval.Busy = false
		m.approval.Err = ""
		if msg.approval != nil {
			m.status = "awaiting approval"
		} else {
			m.status = "idle"
		}
		m.layout()
		m.syncViewport(true)
		return m, nil
	case sessionLoadFailedMsg:
		m.sessions = sessionPickerState{}
		m.status = "failed"
		m.errorText = msg.err.Error()
		m.layout()
		return m, nil
	case runStartedMsg:
		m.sessionID = msg.session.ID
		m.workingDir = msg.session.WorkingDir
		m.trace = msg.trace
		m.cancelRun = msg.cancel
		m.running = true
		m.status = "running"
		m.approval = approvalViewState{}
		m.layout()
		m.syncViewport(true)
		return m, nil
	case runStartFailedMsg:
		m.status = "failed"
		m.errorText = msg.err.Error()
		return m, nil
	case approvalStartedMsg:
		m.trace = msg.trace
		m.cancelRun = msg.cancel
		m.running = true
		m.status = "running"
		m.errorText = ""
		m.layout()
		m.syncViewport(true)
		return m, nil
	case approvalStartFailedMsg:
		m.running = false
		m.cancelRun = nil
		m.status = "awaiting approval"
		m.approval.Busy = false
		m.approval.Err = msg.err.Error()
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

func (m *model) applyAgentEvent(event agent.Event) {
	m.applyTrace(event)

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
			upsertToolGroup(&m.items, *event.ToolCall, "requested")
		}
	case agent.EventTypeToolExecutionStarted:
		if event.ToolCall != nil {
			markToolGroupRunning(&m.items, *event.ToolCall)
		}
	case agent.EventTypeToolMessagePersisted:
		if event.ToolResult != nil {
			if event.ToolCall != nil && findToolGroup(m.items, event.ToolCall.ID) < 0 {
				upsertToolGroup(&m.items, *event.ToolCall, "requested")
			}
			response := conversation.ToolResponseContent{
				ID:      event.ToolResult.ToolCallID,
				IsError: event.ToolResult.IsError,
				Content: event.ToolResult.Content,
			}
			upsertToolResult(&m.items, response)
		}
	case agent.EventTypeCompactionStarted:
		m.items = append(m.items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: fmt.Sprintf("compacting context (%s, %d tokens)", event.CompactionReason, event.TokensBefore)})
	case agent.EventTypeCompactionCompleted:
		m.items = append(m.items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: fmt.Sprintf("compaction complete (%s)", event.CompactionReason)})
	case agent.EventTypeCompactionFailed:
		m.items = append(m.items, transcriptItem{Kind: kindError, Prefix: "system", Text: fmt.Sprintf("compaction failed (%s)", event.CompactionReason)})
	case agent.EventTypeApprovalRequired:
		m.status = "awaiting approval"
		m.running = false
		m.cancelRun = nil
		m.approval = approvalViewState{Request: event.ApprovalRequest}
	case agent.EventTypeApprovalResolved:
		m.approval.Busy = false
		m.approval.Err = ""
		m.approval.Request = nil
	case agent.EventTypeRunCompleted:
		if event.Result != nil && event.Result.Status == agent.StatusAwaitingApproval {
			m.finishRun("awaiting approval")
		} else {
			m.finishRun(runtimeResultStatus(event.Result))
		}
	case agent.EventTypeRunInterrupted:
		m.items = append(m.items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: "interrupted"})
		m.finishRun("interrupted")
	case agent.EventTypeRunFailed:
		m.items = append(m.items, transcriptItem{Kind: kindError, Prefix: "error", Text: errorText(event.Err)})
		m.finishRun("failed")
	}
	m.layout()
	m.syncViewport(false)
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
