package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHelpCommandAppendsCommandList(t *testing.T) {
	m := newModel(context.Background(), &fakeRuntime{}, Options{})
	m.input.SetValue("/help")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	if !containsText(m.items, "system", "commands:") || !containsText(m.items, "system", "/new") {
		t.Fatalf("expected help command output, got %#v", m.items)
	}
}

func TestSessionCommandReportsCurrentState(t *testing.T) {
	runtime := &fakeRuntime{providerName: "openai-codex", modelName: "gpt-5.4", workingDir: "/tmp/project"}
	m := newModel(context.Background(), runtime, Options{})
	m.sessionID = "sess_current"
	m.workingDir = "/tmp/project"
	m.input.SetValue("/session")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	if !containsText(m.items, "system", "session: sess_current") || !containsText(m.items, "system", "model: gpt-5.4") {
		t.Fatalf("expected session command output, got %#v", m.items)
	}
}

func TestNewCommandResetsInteractiveState(t *testing.T) {
	m := newModel(context.Background(), &fakeRuntime{}, Options{})
	m.sessionID = "sess_old"
	m.workingDir = "/tmp/old"
	m.items = []transcriptItem{
		{Kind: kindUser, Prefix: "user", Text: "old"},
	}
	m.input.SetValue("/new")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	if m.sessionID != "" {
		t.Fatalf("expected session id reset, got %q", m.sessionID)
	}
	if len(m.items) != 2 || !containsText(m.items, "system", "started a new session") {
		t.Fatalf("expected reset transcript, got %#v", m.items)
	}
}
