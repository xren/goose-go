package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"goose-go/internal/models"
	tuitheme "goose-go/internal/tui/theme"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"goose-go/internal/agent"
	"goose-go/internal/app"
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
	ContextSnapshot(ctx context.Context, sessionID string) (app.ContextSnapshot, error)
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
	kindUser      itemKind = "user"
	kindAssistant itemKind = "assistant"
	kindTool      itemKind = "tool"
	kindSystem    itemKind = "system"
	kindError     itemKind = "error"
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

type contextPanelState struct {
	Open     bool
	Busy     bool
	Err      string
	Snapshot app.ContextSnapshot
	Viewport viewport.Model
}

type model struct {
	ctx           context.Context
	runtime       Runtime
	opts          Options
	input         textinput.Model
	width         int
	height        int
	status        string
	errorText     string
	sessionID     string
	workingDir    string
	async         chan tea.Msg
	running       bool
	cancelRun     context.CancelFunc
	trace         app.EventRecorder
	approval      approvalViewState
	picker        modelPickerState
	sessions      sessionPickerState
	themes        themePickerState
	contextPanel  contextPanelState
	theme         tuitheme.Palette
	debug         bool
	liveAssistant string
	activeTools   []transcriptItem
	printer       transcriptPrinter
}

type modelsLoadedMsg struct{ items []models.Availability }
type modelsLoadFailedMsg struct{ err error }
type modelSetMsg struct{}
type modelSetFailedMsg struct{ err error }
type sessionsLoadedMsg struct{ items []session.Summary }
type sessionsLoadFailedMsg struct{ err error }

func Run(ctx context.Context, in io.Reader, out io.Writer, runtime Runtime, opts Options) error {
	m := newModel(ctx, runtime, opts)
	p := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out))
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
	contextViewport := viewport.New(0, 0)
	contextViewport.SetContent("")

	return model{
		ctx:        ctx,
		runtime:    runtime,
		opts:       opts,
		input:      input,
		status:     initialStatus(opts),
		workingDir: runtime.WorkingDir(),
		async:      make(chan tea.Msg, 128),
		theme:      palette,
		debug:      opts.Debug,
		printer:    bubbleTranscriptPrinter{},
		contextPanel: contextPanelState{
			Viewport: contextViewport,
		},
	}
}

func initialStatus(opts Options) string {
	if opts.SessionID != "" {
		return "loading session"
	}
	return "idle"
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{waitForAsync(m.async), tea.WindowSize()}
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
			case "pgup":
				visibleItems, _ := m.sessionPickerMetrics()
				m.sessions.Selected = max(0, m.sessions.Selected-visibleItems)
				return m, nil
			case "pgdown":
				visibleItems, _ := m.sessionPickerMetrics()
				m.sessions.Selected = min(len(m.sessions.Items)-1, m.sessions.Selected+visibleItems)
				return m, nil
			case "home":
				m.sessions.Selected = 0
				return m, nil
			case "end":
				if len(m.sessions.Items) > 0 {
					m.sessions.Selected = len(m.sessions.Items) - 1
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
				return m, m.printItemsCmd([]transcriptItem{
					{Kind: kindSystem, Prefix: "system", Text: "/theme"},
					{Kind: kindSystem, Prefix: "system", Text: fmt.Sprintf("selected theme: %s", palette.Name)},
				})
			}
		}

		if m.contextPanel.Open && m.handleContextPanelKey(msg) {
			return m, nil
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
			if m.contextPanel.Open {
				m.contextPanel.Open = false
				m.contextPanel.Busy = false
				m.contextPanel.Err = ""
				m.layout()
				return m, nil
			}
			if m.running && m.cancelRun != nil {
				m.cancelRun()
				m.status = "interrupting"
			}
			return m, nil
		case "enter":
			if m.running || m.approval.Request != nil || m.picker.Open || m.themes.Open || m.sessions.Open {
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
			if handled, cmd := m.handleLocalCommand(prompt); handled {
				return m, cmd
			}
			m.status = "starting"
			return m, startRunCmd(m.ctx, m.runtime, m.async, prompt, m.sessionID)
		case "ctrl+d":
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
		m.picker = modelPickerState{}
		m.status = "idle"
		cmd := m.printItemsCmd([]transcriptItem{
			{Kind: kindSystem, Prefix: "system", Text: "/model"},
			{Kind: kindSystem, Prefix: "system", Text: fmt.Sprintf("selected provider: %s\nselected model: %s", providerName, modelName)},
		})
		if m.contextPanel.Open {
			return m, tea.Batch(cmd, m.refreshContextCmd())
		}
		return m, cmd
	case modelSetFailedMsg:
		m.picker.Busy = false
		m.picker.Err = msg.err.Error()
		m.status = "select model"
		return m, nil
	case sessionLoadedMsg:
		m.sessions = sessionPickerState{}
		m.sessionID = msg.session.ID
		m.workingDir = msg.session.WorkingDir
		m.liveAssistant = ""
		m.activeTools = nil
		m.approval.Request = msg.approval
		m.approval.Busy = false
		m.approval.Err = ""
		if msg.approval != nil {
			m.status = "awaiting approval"
		} else {
			m.status = "idle"
		}
		cmd := m.printConversationCmd(msg.session.Conversation)
		if m.contextPanel.Open {
			return m, tea.Batch(cmd, m.refreshContextCmd())
		}
		return m, cmd
	case sessionLoadFailedMsg:
		m.sessions = sessionPickerState{}
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
		m.approval = approvalViewState{}
		m.liveAssistant = ""
		m.activeTools = nil
		if m.contextPanel.Open {
			return m, m.refreshContextCmd()
		}
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
		m.liveAssistant = ""
		return m, nil
	case approvalStartFailedMsg:
		m.running = false
		m.cancelRun = nil
		m.status = "awaiting approval"
		m.approval.Busy = false
		m.approval.Err = msg.err.Error()
		return m, nil
	case agentEventMsg:
		cmd := m.applyAgentEvent(msg.event)
		return m, waitWith(cmd, m.async)
	case traceWriteFailedMsg:
		m.status = "failed"
		m.errorText = msg.err.Error()
		return m, waitForAsync(m.async)
	case noOpMsg:
		return m, waitForAsync(m.async)
	case contextLoadedMsg:
		m.contextPanel.Busy = false
		m.contextPanel.Err = ""
		m.contextPanel.Snapshot = msg.snapshot
		m.syncContextViewport()
		return m, nil
	case contextLoadFailedMsg:
		m.contextPanel.Busy = false
		m.contextPanel.Err = msg.err.Error()
		m.syncContextViewport()
		return m, nil
	case error:
		m.status = "failed"
		m.errorText = msg.Error()
		return m, nil
	default:
		return m, waitForAsync(m.async)
	}
}

func (m *model) handleContextPanelKey(msg tea.KeyMsg) bool {
	if !m.contextPanel.Open || m.sessions.Open || m.picker.Open || m.themes.Open || m.approval.Request != nil {
		return false
	}
	switch msg.String() {
	case "up", "k":
		m.contextPanel.Viewport.LineUp(1)
		return true
	case "down", "j":
		m.contextPanel.Viewport.LineDown(1)
		return true
	case "pgup", "ctrl+u", "ctrl+b":
		m.contextPanel.Viewport.HalfPageUp()
		return true
	case "pgdown", "ctrl+d", "ctrl+f":
		m.contextPanel.Viewport.HalfPageDown()
		return true
	case "home":
		m.contextPanel.Viewport.GotoTop()
		return true
	case "end":
		m.contextPanel.Viewport.GotoBottom()
		return true
	}
	return false
}
