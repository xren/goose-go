package tui

import (
	"context"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"goose-go/internal/agent"
	"goose-go/internal/conversation"
	tuitheme "goose-go/internal/tui/theme"
)

func TestViewportScrollKeysMoveTranscript(t *testing.T) {
	m := newModel(context.Background(), &fakeRuntime{}, Options{})
	m.width = 80
	m.height = 12
	for i := 0; i < 40; i++ {
		m.items = append(m.items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: strings.Repeat("line ", 4) + strconv.Itoa(i)})
	}
	m.layout()
	m.syncViewport(true)

	if !m.viewport.AtBottom() {
		t.Fatal("expected viewport to start at bottom")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = updated.(model)
	if m.viewport.AtBottom() {
		t.Fatal("expected page up to move away from bottom")
	}
	afterPgUp := m.viewport.YOffset
	if afterPgUp <= 0 {
		t.Fatalf("expected positive viewport offset after page up, got %d", afterPgUp)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m = updated.(model)
	if m.viewport.YOffset != 0 {
		t.Fatalf("expected home to jump to top, got %d", m.viewport.YOffset)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(model)
	if !m.viewport.AtBottom() {
		t.Fatal("expected end to jump back to bottom")
	}
}

func TestViewportDoesNotSnapToBottomWhenUserScrolledUp(t *testing.T) {
	m := newModel(context.Background(), &fakeRuntime{}, Options{})
	m.width = 80
	m.height = 12
	for i := 0; i < 40; i++ {
		m.items = append(m.items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: "history " + strconv.Itoa(i)})
	}
	m.layout()
	m.syncViewport(true)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = updated.(model)
	offsetBefore := m.viewport.YOffset
	if offsetBefore <= 0 {
		t.Fatalf("expected scroll offset before new output, got %d", offsetBefore)
	}

	m.applyAgentEvent(agent.Event{
		Type:    agent.EventTypeAssistantMessageComplete,
		Message: message(conversation.RoleAssistant, "new output"),
	})

	if m.viewport.AtBottom() {
		t.Fatal("expected viewport to remain off bottom after new output while scrolled up")
	}
	if m.viewport.YOffset != offsetBefore {
		t.Fatalf("expected viewport offset to stay %d, got %d", offsetBefore, m.viewport.YOffset)
	}
}

func TestMouseWheelScrollsTranscript(t *testing.T) {
	m := newModel(context.Background(), &fakeRuntime{}, Options{})
	m.width = 80
	m.height = 12
	for i := 0; i < 40; i++ {
		m.items = append(m.items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: "history " + strconv.Itoa(i)})
	}
	m.layout()
	m.syncViewport(true)

	if !m.viewport.AtBottom() {
		t.Fatal("expected viewport to start at bottom")
	}

	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelUp,
	})
	m = updated.(model)
	if m.viewport.AtBottom() {
		t.Fatal("expected mouse wheel up to move away from bottom")
	}

	offsetAfterUp := m.viewport.YOffset
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	})
	m = updated.(model)
	if m.viewport.YOffset <= offsetAfterUp {
		t.Fatalf("expected wheel down to move viewport back toward bottom, before=%d after=%d", offsetAfterUp, m.viewport.YOffset)
	}
}

func TestRenderToolItemDoesNotExceedViewportWidth(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	item := transcriptItem{
		Kind:   kindTool,
		Prefix: "tool[shell]",
		Meta:   "completed",
		Text: renderToolGroup(
			"completed",
			`{"command":"sed -n '1,50p' docs/invariants.md","working_dir":"."}`,
			"The root architecture doc defines package-level boundaries. Concrete subsystem behavior is documented separately.\n\nThis is additional wrapped output that should stay inside the card width.",
			false,
		),
	}

	rendered := renderToolItem(theme, item, 80)
	for _, line := range strings.Split(rendered, "\n") {
		if lipgloss.Width(line) > 80 {
			t.Fatalf("expected rendered line width <= 80, got %d for line %q", lipgloss.Width(line), line)
		}
	}
}
