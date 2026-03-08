package tui

import (
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"goose-go/internal/agent"
	"goose-go/internal/app"
	"goose-go/internal/session"
)

type sessionLoadedMsg struct {
	session  session.Session
	approval *agent.ApprovalRequest
}

type sessionLoadFailedMsg struct{ err error }

type runStartedMsg struct {
	session session.Session
	trace   app.EventRecorder
	cancel  context.CancelFunc
}

type runStartFailedMsg struct{ err error }

type approvalStartedMsg struct {
	trace  app.EventRecorder
	cancel context.CancelFunc
}

type approvalStartFailedMsg struct{ err error }

type agentEventMsg struct{ event agent.Event }

type traceWriteFailedMsg struct{ err error }

type noOpMsg struct{}

func loadSessionCmd(ctx context.Context, runtime Runtime, sessionID string) tea.Cmd {
	return func() tea.Msg {
		session, err := runtime.ReplayConversation(ctx, sessionID)
		if err != nil {
			return sessionLoadFailedMsg{err: err}
		}
		approval, err := runtime.PendingApproval(ctx, sessionID)
		if err != nil && !errors.Is(err, agent.ErrApprovalNotPending) {
			return sessionLoadFailedMsg{err: err}
		}
		return sessionLoadedMsg{session: session, approval: approval}
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

func resolveApprovalCmd(ctx context.Context, runtime Runtime, async chan tea.Msg, sessionID string, decision agent.ApprovalDecision) tea.Cmd {
	return func() tea.Msg {
		trace, err := runtime.OpenTraceWriter(sessionID)
		if err != nil {
			return approvalStartFailedMsg{err: err}
		}
		runCtx, cancel := context.WithCancel(ctx)
		stream, err := runtime.ResolveApprovalStream(runCtx, sessionID, decision)
		if err != nil {
			_ = trace.Close()
			cancel()
			return approvalStartFailedMsg{err: err}
		}
		go bridgeStream(async, runCtx, stream)
		return approvalStartedMsg{trace: trace, cancel: cancel}
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
	if value == "" {
		return defaultValue
	}
	return value
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
		if string(status) == "" {
			return "idle"
		}
		return string(status)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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

func (m *model) applyTrace(event agent.Event) {
	if m.trace == nil {
		return
	}
	if err := m.trace.Write(event); err != nil {
		select {
		case m.async <- traceWriteFailedMsg{err: fmt.Errorf("write trace event: %w", err)}:
		default:
		}
	}
}
