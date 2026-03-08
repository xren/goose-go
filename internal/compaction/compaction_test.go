package compaction

import (
	"encoding/json"
	"strings"
	"testing"

	"goose-go/internal/conversation"
	"goose-go/internal/session"
)

func TestEstimateConversationTokens(t *testing.T) {
	messages := []conversation.Message{
		conversation.NewMessage(conversation.RoleUser, conversation.Text(strings.Repeat("a", 80))),
		conversation.NewMessage(conversation.RoleAssistant, conversation.Text(strings.Repeat("b", 40))),
	}

	got := EstimateConversationTokens(messages)
	if got <= 0 {
		t.Fatalf("expected positive token estimate, got %d", got)
	}
}

func TestPrepareNoCompactionNeeded(t *testing.T) {
	settings := DefaultSettings()
	messages := []conversation.Message{
		conversation.NewMessage(conversation.RoleUser, conversation.Text("short prompt")),
	}

	preparation, err := Prepare(messages, 100000, settings)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if preparation.NeedsCompaction {
		t.Fatalf("expected no compaction needed")
	}
	if got := len(preparation.KeptMessages); got != 1 {
		t.Fatalf("expected kept messages, got %d", got)
	}
}

func TestPrepareSelectsUserTurnBoundary(t *testing.T) {
	settings := Settings{
		Enabled:          true,
		ReserveTokens:    10,
		KeepRecentTokens: 8,
	}

	messages := []conversation.Message{
		conversation.NewMessage(conversation.RoleUser, conversation.Text(strings.Repeat("a", 100))),
		conversation.NewMessage(conversation.RoleAssistant, conversation.Text(strings.Repeat("b", 20))),
		conversation.NewMessage(conversation.RoleUser, conversation.Text(strings.Repeat("c", 20))),
		conversation.NewMessage(conversation.RoleAssistant, conversation.Text(strings.Repeat("d", 20))),
	}

	preparation, err := Prepare(messages, 40, settings)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if !preparation.NeedsCompaction {
		t.Fatalf("expected compaction to be needed")
	}
	if got, want := preparation.FirstKeptMessageID, messages[2].ID; got != want {
		t.Fatalf("expected first kept message %q, got %q", want, got)
	}
	if got := len(preparation.MessagesToSummarize); got != 2 {
		t.Fatalf("expected 2 summarized messages, got %d", got)
	}
}

func TestBuildActiveMessagesFromCompaction(t *testing.T) {
	messages := []conversation.Message{
		conversation.NewMessage(conversation.RoleUser, conversation.Text("first")),
		conversation.NewMessage(conversation.RoleAssistant, conversation.Text("second")),
		conversation.NewMessage(conversation.RoleUser, conversation.Text("third")),
	}

	artifact := session.Compaction{
		ID:                 "cmp_1",
		SessionID:          "sess_1",
		Summary:            "summary body",
		FirstKeptMessageID: messages[2].ID,
		TokensBefore:       123,
		Trigger:            session.CompactionTriggerThreshold,
	}

	active, err := BuildActiveMessages(messages, artifact)
	if err != nil {
		t.Fatalf("build active messages: %v", err)
	}

	if got := len(active); got != 2 {
		t.Fatalf("expected 2 active messages, got %d", got)
	}
	if active[0].Role != conversation.RoleAssistant {
		t.Fatalf("expected synthetic summary assistant message, got %q", active[0].Role)
	}
	if got := active[1].ID; got != messages[2].ID {
		t.Fatalf("expected kept message %q, got %q", messages[2].ID, got)
	}
}

func TestBuildActiveMessagesMissingFirstKept(t *testing.T) {
	_, err := BuildActiveMessages([]conversation.Message{
		conversation.NewMessage(conversation.RoleUser, conversation.Text("first")),
	}, session.Compaction{
		ID:                 "cmp_1",
		FirstKeptMessageID: "missing",
	})
	if err == nil {
		t.Fatalf("expected missing first kept message error")
	}
}

func TestSerializeForSummarization(t *testing.T) {
	args := json.RawMessage(`{"command":"pwd"}`)
	message := conversation.NewMessage(
		conversation.RoleAssistant,
		conversation.Text("working"),
		conversation.ToolRequest("tool_1", "shell", args),
	)

	got := SerializeForSummarization([]conversation.Message{message})
	if !strings.Contains(got, "[ASSISTANT]:") {
		t.Fatalf("expected assistant role in serialization, got %q", got)
	}
	if !strings.Contains(got, `tool_request(shell): {"command":"pwd"}`) {
		t.Fatalf("expected compact tool request in serialization, got %q", got)
	}
}
