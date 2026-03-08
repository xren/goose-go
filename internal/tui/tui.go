package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"goose-go/internal/models"

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
}

type modelsLoadedMsg struct{ items []models.Availability }
type modelsLoadFailedMsg struct{ err error }
type modelSetMsg struct{}
type modelSetFailedMsg struct{ err error }
type sessionsLoadedMsg struct{ items []session.Summary }
type sessionsLoadFailedMsg struct{ err error }

var (
	headerStyle        = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	statusStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 1)
	errorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Padding(0, 1)
	approvalPanelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("220")).Padding(0, 1)
	approvalTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
	approvalErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	pickerPanelStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("69")).Padding(0, 1)
	pickerTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
	pickerCursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("69")).Bold(true)
	pickerDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	sessionPanelStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("111")).Padding(0, 1)
	sessionTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
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
			if m.running && m.cancelRun != nil {
				m.cancelRun()
				m.status = "interrupting"
			}
			return m, nil
		case "enter":
			if m.running || m.approval.Request != nil || m.picker.Open {
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
				m.syncViewport()
				return m, nil
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
		m.items = append(m.items,
			transcriptItem{Kind: kindSystem, Prefix: "system", Text: "/model"},
			transcriptItem{Kind: kindSystem, Prefix: "system", Text: fmt.Sprintf("selected provider: %s\nselected model: %s", providerName, modelName)},
		)
		m.picker = modelPickerState{}
		m.status = "idle"
		m.syncViewport()
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
		m.approval = approvalViewState{}
		m.syncViewport()
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
		m.syncViewport()
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

func (m model) View() string {
	header := headerStyle.Render(fmt.Sprintf("session: %s", fallback(m.sessionID, "new")))
	cwd := statusStyle.Render(fmt.Sprintf("cwd: %s", fallback(m.workingDir, "-")))
	status := statusStyle.Render(fmt.Sprintf("status: %s", m.status))
	if m.errorText != "" {
		status += "\n" + errorStyle.Render(m.errorText)
	}
	parts := []string{header, cwd, status, m.viewport.View()}
	if panel := m.approvalPanel(); panel != "" {
		parts = append(parts, panel)
	}
	if panel := m.sessionPickerPanel(); panel != "" {
		parts = append(parts, panel)
	}
	if panel := m.modelPickerPanel(); panel != "" {
		parts = append(parts, panel)
	}
	parts = append(parts, m.input.View(), statusStyle.Render(m.footerText()))
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *model) layout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	headerLines := 3
	footerLines := 2
	composerLines := 1
	approvalLines := 0
	if m.approval.Request != nil {
		approvalLines = 7
		if m.approval.Err != "" {
			approvalLines++
		}
	}
	pickerLines := 0
	if m.picker.Open {
		pickerLines = len(m.picker.Items) + 3
		if m.picker.Err != "" {
			pickerLines++
		}
		if pickerLines < 6 {
			pickerLines = 6
		}
	}
	sessionLines := 0
	if m.sessions.Open {
		sessionLines = len(m.sessions.Items)*2 + 3
		if m.sessions.Err != "" {
			sessionLines++
		}
		if sessionLines < 6 {
			sessionLines = 6
		}
	}
	bodyHeight := m.height - headerLines - footerLines - composerLines - approvalLines - pickerLines - sessionLines
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
	m.syncViewport()
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
