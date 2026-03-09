package tui

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"goose-go/internal/conversation"
	"goose-go/internal/session"
)

func TestSessionPickerLoadsSelectedSession(t *testing.T) {
	msgTime := time.Now().UTC()
	runtime := &fakeRuntime{
		sessionSummaries: []session.Summary{
			{ID: "sess_a", Name: "A", WorkingDir: "/tmp/a", Provider: "openai-codex", Model: "gpt-5-codex", MessageCount: 1},
			{ID: "sess_b", Name: "B", WorkingDir: "/tmp/b", Provider: "openai-codex", Model: "gpt-5.3-codex", MessageCount: 2},
		},
		replay: session.Session{
			ID:         "sess_b",
			WorkingDir: "/tmp/b",
			Provider:   "openai-codex",
			Model:      "gpt-5.3-codex",
			Conversation: conversation.Conversation{Messages: []conversation.Message{
				{ID: "m1", Role: conversation.RoleUser, CreatedAt: msgTime, Content: []conversation.Content{conversation.Text("resume me")}},
			}},
		},
	}
	m := newModel(context.Background(), runtime, Options{})
	m.sessions = sessionPickerState{Open: true, Items: runtime.sessionSummaries, Selected: 1}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if cmd == nil {
		t.Fatal("expected load selected session command")
	}
	updated, _ = m.Update(cmd())
	m = updated.(model)

	if m.sessions.Open {
		t.Fatal("expected sessions picker to close after load")
	}
	if m.sessionID != "sess_b" {
		t.Fatalf("expected session id sess_b, got %q", m.sessionID)
	}
	if !containsText(m.items, "user", "resume me") {
		t.Fatalf("expected replayed transcript, got %#v", m.items)
	}
}

func TestSessionPickerLoadRestoresViewportHeight(t *testing.T) {
	msgTime := time.Now().UTC()
	summaries := make([]session.Summary, 12)
	for i := range summaries {
		summaries[i] = session.Summary{
			ID:         "sess_" + strconv.Itoa(i),
			Name:       "Session " + strconv.Itoa(i),
			WorkingDir: "/tmp/project",
			Provider:   "openai-codex",
			Model:      "gpt-5-codex",
		}
	}

	runtime := &fakeRuntime{
		sessionSummaries: summaries,
		replay: session.Session{
			ID:         "sess_3",
			WorkingDir: "/tmp/project",
			Provider:   "openai-codex",
			Model:      "gpt-5-codex",
			Conversation: conversation.Conversation{Messages: []conversation.Message{
				{ID: "m1", Role: conversation.RoleUser, CreatedAt: msgTime, Content: []conversation.Content{conversation.Text("resume me")}},
			}},
		},
	}

	m := newModel(context.Background(), runtime, Options{})
	m.width = 120
	m.height = 40
	m.sessions = sessionPickerState{Open: true, Items: summaries, Selected: 3}
	m.layout()
	heightWithPicker := m.viewport.Height

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if cmd == nil {
		t.Fatal("expected load selected session command")
	}
	updated, _ = m.Update(cmd())
	m = updated.(model)

	if m.sessions.Open {
		t.Fatal("expected sessions picker to close after load")
	}
	if m.viewport.Height <= heightWithPicker {
		t.Fatalf("expected viewport height to grow after picker closes, before=%d after=%d", heightWithPicker, m.viewport.Height)
	}
}

func TestSessionPickerPanelShowsWindowedItems(t *testing.T) {
	summaries := make([]session.Summary, 20)
	for i := range summaries {
		summaries[i] = session.Summary{
			ID:           "sess_" + strconv.Itoa(i),
			Name:         "Session " + strconv.Itoa(i),
			WorkingDir:   "/tmp/project",
			Provider:     "openai-codex",
			Model:        "gpt-5-codex",
			MessageCount: i + 1,
		}
	}

	m := newModel(context.Background(), &fakeRuntime{}, Options{})
	m.width = 120
	m.height = 20
	m.sessions = sessionPickerState{Open: true, Items: summaries, Selected: 10}
	m.layout()

	panel := m.sessionPickerPanel()
	visibleItems, _ := m.sessionPickerMetrics()
	if got := strings.Count(panel, " (sess_"); got != visibleItems {
		t.Fatalf("expected %d visible items in panel, got %d\npanel:\n%s", visibleItems, got, panel)
	}
	if !strings.Contains(panel, "showing ") {
		t.Fatalf("expected windowed session hint, got:\n%s", panel)
	}
	if !strings.Contains(panel, "enter open  esc close") {
		t.Fatalf("expected inline session picker action hint, got:\n%s", panel)
	}
}
