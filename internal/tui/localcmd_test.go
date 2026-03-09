package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"goose-go/internal/app"
)

func TestHelpCommandPrintsCommandList(t *testing.T) {
	m, printer := newCaptureModel(t, &fakeRuntime{}, Options{})
	m.input.SetValue("/help")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	_ = cmd

	if !containsPrinted(printer.blocks, "commands:") || !containsPrinted(printer.blocks, "/new") || !containsPrinted(printer.blocks, "/context") {
		t.Fatalf("expected help command output, got %#v", printer.blocks)
	}
}

func TestContextCommandTogglesPanelAndLoadsSnapshot(t *testing.T) {
	runtime := &fakeRuntime{
		contextSnapshot: app.ContextSnapshot{
			SystemPrompt: "You are helpful.",
		},
	}
	m, _ := newCaptureModel(t, runtime, Options{})
	m.width = 100
	m.height = 24
	m.layout()
	m.input.SetValue("/context")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if !m.contextPanel.Open || !m.contextPanel.Busy {
		t.Fatalf("expected context panel to open and start loading, got %+v", m.contextPanel)
	}
	if cmd == nil {
		t.Fatal("expected context load command")
	}

	updated, _ = m.Update(cmd())
	m = updated.(model)
	if m.contextPanel.Busy {
		t.Fatal("expected context load to finish")
	}
	if runtime.contextCalls != 1 {
		t.Fatalf("expected one context snapshot load, got %d", runtime.contextCalls)
	}
	if !strings.Contains(m.View(), "Current context") || !strings.Contains(m.View(), "System prompt") {
		t.Fatalf("expected context panel in view, got %q", m.View())
	}

	m.input.SetValue("/context")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.contextPanel.Open {
		t.Fatal("expected context panel to close on second toggle")
	}
}

func TestSessionCommandReportsCurrentState(t *testing.T) {
	runtime := &fakeRuntime{providerName: "openai-codex", modelName: "gpt-5.4", workingDir: "/tmp/project"}
	m, printer := newCaptureModel(t, runtime, Options{})
	m.sessionID = "sess_current"
	m.workingDir = "/tmp/project"
	m.input.SetValue("/session")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	_ = cmd

	if !containsPrinted(printer.blocks, "session: sess_current") || !containsPrinted(printer.blocks, "model: gpt-5.4") {
		t.Fatalf("expected session command output, got %#v", printer.blocks)
	}
}

func TestNewCommandResetsInteractiveState(t *testing.T) {
	m, printer := newCaptureModel(t, &fakeRuntime{}, Options{})
	m.sessionID = "sess_old"
	m.workingDir = "/tmp/old"
	m.liveAssistant = "preview"
	m.activeTools = []transcriptItem{{Kind: kindTool, Key: "call_1"}}
	m.input.SetValue("/new")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	_ = cmd

	if m.sessionID != "" {
		t.Fatalf("expected session id reset, got %q", m.sessionID)
	}
	if m.liveAssistant != "" || len(m.activeTools) != 0 || m.approval.Request != nil {
		t.Fatalf("expected interactive state reset, got preview=%q tools=%d approval=%#v", m.liveAssistant, len(m.activeTools), m.approval)
	}
	if !containsPrinted(printer.blocks, "started a new session") {
		t.Fatalf("expected reset output, got %#v", printer.blocks)
	}
}

func TestThemeCommandOpensThemePicker(t *testing.T) {
	m, _ := newCaptureModel(t, &fakeRuntime{}, Options{})
	m.input.SetValue("/theme")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	if !m.themes.Open {
		t.Fatal("expected theme picker to open")
	}
	if m.status != "select theme" {
		t.Fatalf("expected select theme status, got %q", m.status)
	}
	if len(m.themes.Items) == 0 {
		t.Fatal("expected built-in themes to be available")
	}
}

func TestDebugCommandTogglesMode(t *testing.T) {
	m, printer := newCaptureModel(t, &fakeRuntime{}, Options{})
	m.input.SetValue("/debug")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	_ = cmd

	if !m.debug {
		t.Fatal("expected debug mode to be enabled")
	}
	if !containsPrinted(printer.blocks, "debug mode: on") {
		t.Fatalf("expected debug confirmation, got %#v", printer.blocks)
	}
}

func TestHandleLocalCommandUnknownFallsThrough(t *testing.T) {
	m, _ := newCaptureModel(t, &fakeRuntime{}, Options{})
	handled, cmd := m.handleLocalCommand("/nope")
	if handled || cmd != nil {
		t.Fatalf("expected unknown local command to fall through, handled=%v cmd=%v", handled, cmd)
	}
}

func TestNewModelUsesIdleStatusWithoutSession(t *testing.T) {
	m, _ := newCaptureModel(t, &fakeRuntime{}, Options{})
	if m.status != "idle" {
		t.Fatalf("expected idle status, got %q", m.status)
	}
}

func TestNewModelStartsInLoadingSessionStateWhenSessionRequested(t *testing.T) {
	m, _ := newCaptureModel(t, &fakeRuntime{}, Options{SessionID: "sess_replay"})
	if m.status != "loading session" {
		t.Fatalf("expected loading session status, got %q", m.status)
	}
}
