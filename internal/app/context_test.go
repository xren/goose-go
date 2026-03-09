package app

import (
	"context"
	"strings"
	"testing"

	"goose-go/internal/conversation"
	"goose-go/internal/session"
	sqlitestore "goose-go/internal/storage/sqlite"
)

func TestContextSnapshotWithoutSessionReturnsPromptOnly(t *testing.T) {
	store, err := sqlitestore.Open(t.TempDir() + "/sessions.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	runtime := &Runtime{
		store:        store,
		workingDir:   "/tmp/project",
		provider:     "openai-codex",
		model:        "gpt-5-codex",
		systemPrompt: "You are helpful.",
	}

	snapshot, err := runtime.ContextSnapshot(context.Background(), "")
	if err != nil {
		t.Fatalf("context snapshot: %v", err)
	}
	if snapshot.HasSession {
		t.Fatal("expected no persisted session")
	}
	if snapshot.SystemPrompt != "You are helpful." {
		t.Fatalf("expected system prompt, got %q", snapshot.SystemPrompt)
	}
	if len(snapshot.ActiveMessages) != 0 {
		t.Fatalf("expected no active messages, got %d", len(snapshot.ActiveMessages))
	}
}

func TestContextSnapshotReturnsActiveMessagesWithoutCompaction(t *testing.T) {
	store, err := sqlitestore.Open(t.TempDir() + "/sessions.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	record, err := store.CreateSession(context.Background(), session.CreateParams{
		Name:       "test",
		WorkingDir: "/tmp/project",
		Provider:   "openai-codex",
		Model:      "gpt-5-codex",
		Type:       session.TypeTerminal,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	record, err = store.AddMessage(context.Background(), record.ID, conversation.NewMessage(conversation.RoleUser, conversation.Text("hello")))
	if err != nil {
		t.Fatalf("add user message: %v", err)
	}
	record, err = store.AddMessage(context.Background(), record.ID, conversation.NewMessage(conversation.RoleAssistant, conversation.Text("world")))
	if err != nil {
		t.Fatalf("add assistant message: %v", err)
	}

	runtime := &Runtime{
		store:        store,
		workingDir:   "/tmp/project",
		provider:     "openai-codex",
		model:        "gpt-5-codex",
		systemPrompt: "You are helpful.",
	}

	snapshot, err := runtime.ContextSnapshot(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("context snapshot: %v", err)
	}
	if !snapshot.HasSession || snapshot.SessionID != record.ID {
		t.Fatalf("expected session metadata, got %+v", snapshot)
	}
	if len(snapshot.ActiveMessages) != 2 {
		t.Fatalf("expected 2 active messages, got %d", len(snapshot.ActiveMessages))
	}
	if snapshot.EstimatedTokens <= 0 {
		t.Fatalf("expected positive token estimate, got %d", snapshot.EstimatedTokens)
	}
}

func TestContextSnapshotUsesLatestCompaction(t *testing.T) {
	store, err := sqlitestore.Open(t.TempDir() + "/sessions.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	record, err := store.CreateSession(context.Background(), session.CreateParams{
		Name:       "test",
		WorkingDir: "/tmp/project",
		Provider:   "openai-codex",
		Model:      "gpt-5-codex",
		Type:       session.TypeTerminal,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	record, err = store.AddMessage(context.Background(), record.ID, conversation.NewMessage(conversation.RoleUser, conversation.Text("first")))
	if err != nil {
		t.Fatalf("add first message: %v", err)
	}
	second := conversation.NewMessage(conversation.RoleUser, conversation.Text("second"))
	record, err = store.AddMessage(context.Background(), record.ID, second)
	if err != nil {
		t.Fatalf("add second message: %v", err)
	}
	artifact, err := store.AppendCompaction(context.Background(), record.ID, session.CompactionParams{
		Summary:            "summary text",
		FirstKeptMessageID: second.ID,
		TokensBefore:       99,
		Trigger:            session.CompactionTriggerThreshold,
	})
	if err != nil {
		t.Fatalf("append compaction: %v", err)
	}

	runtime := &Runtime{
		store:        store,
		workingDir:   "/tmp/project",
		provider:     "openai-codex",
		model:        "gpt-5-codex",
		systemPrompt: "You are helpful.",
	}

	snapshot, err := runtime.ContextSnapshot(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("context snapshot: %v", err)
	}
	if snapshot.LatestCompaction == nil || snapshot.LatestCompaction.ID != artifact.ID {
		t.Fatalf("expected latest compaction metadata, got %+v", snapshot.LatestCompaction)
	}
	if len(snapshot.ActiveMessages) != 2 {
		t.Fatalf("expected summary message plus kept message, got %d", len(snapshot.ActiveMessages))
	}
	if snapshot.ActiveMessages[0].Role != conversation.RoleAssistant {
		t.Fatalf("expected summary message first, got %+v", snapshot.ActiveMessages[0])
	}
	if !strings.Contains(snapshot.ActiveMessages[0].Content[0].Text.Text, "summary text") {
		t.Fatalf("expected summary content, got %+v", snapshot.ActiveMessages[0])
	}
}
