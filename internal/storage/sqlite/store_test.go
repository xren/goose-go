package sqlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"

	"goose-go/internal/conversation"
	"goose-go/internal/session"
)

func TestStoreSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	created, err := store.CreateSession(ctx, session.CreateParams{
		Name:       "test",
		WorkingDir: t.TempDir(),
		Provider:   "openai-codex",
		Model:      "gpt-5-codex",
		Type:       session.TypeUser,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	loaded, err := store.GetSession(ctx, created.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}

	if loaded.Name != "test" {
		t.Fatalf("expected name test, got %q", loaded.Name)
	}
	if loaded.Provider != "openai-codex" || loaded.Model != "gpt-5-codex" {
		t.Fatalf("expected persisted provider/model, got provider=%q model=%q", loaded.Provider, loaded.Model)
	}

	msg := conversation.NewMessage(
		conversation.RoleUser,
		conversation.Text("hello"),
		conversation.ToolRequest("tool_1", "shell", json.RawMessage(`{"command":"pwd"}`)),
	)

	updated, err := store.AddMessage(ctx, created.ID, msg)
	if err != nil {
		t.Fatalf("add message: %v", err)
	}

	if updated.MessageCount != 1 {
		t.Fatalf("expected 1 message, got %d", updated.MessageCount)
	}

	replayed, err := store.ReplayConversation(ctx, created.ID)
	if err != nil {
		t.Fatalf("replay conversation: %v", err)
	}

	if len(replayed.Messages) != 1 {
		t.Fatalf("expected 1 replayed message, got %d", len(replayed.Messages))
	}

	replacement := conversation.NewConversation()
	if err := replacement.Append(conversation.NewMessage(conversation.RoleAssistant, conversation.Text("done"))); err != nil {
		t.Fatalf("append replacement message: %v", err)
	}

	replaced, err := store.ReplaceConversation(ctx, created.ID, replacement)
	if err != nil {
		t.Fatalf("replace conversation: %v", err)
	}

	if replaced.MessageCount != 1 {
		t.Fatalf("expected 1 replaced message, got %d", replaced.MessageCount)
	}

	final, err := store.GetSession(ctx, created.ID)
	if err != nil {
		t.Fatalf("reload session: %v", err)
	}

	if got := final.Conversation.Messages[0].Content[0].Text.Text; got != "done" {
		t.Fatalf("expected done, got %q", got)
	}
}

func TestStoreMissingSession(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	if _, err := store.GetSession(ctx, "missing"); err != session.ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestStoreAddMessageConcurrent(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	created, err := store.CreateSession(ctx, session.CreateParams{
		Name:       "concurrent",
		WorkingDir: t.TempDir(),
		Provider:   "openai-codex",
		Model:      "gpt-5-codex",
		Type:       session.TypeUser,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	const writers = 8

	start := make(chan struct{})
	errCh := make(chan error, writers)
	var wg sync.WaitGroup

	for i := range writers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start

			msg := conversation.NewMessage(
				conversation.RoleUser,
				conversation.Text("message"),
				conversation.ToolRequest("tool_1", "shell", json.RawMessage(`{"command":"pwd"}`)),
			)

			if _, err := store.AddMessage(ctx, created.ID, msg); err != nil {
				errCh <- err
			}
		}(i)
	}

	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent add message: %v", err)
		}
	}

	replayed, err := store.ReplayConversation(ctx, created.ID)
	if err != nil {
		t.Fatalf("replay conversation: %v", err)
	}

	if got := len(replayed.Messages); got != writers {
		t.Fatalf("expected %d replayed messages, got %d", writers, got)
	}
}

func TestStoreAppliesSchemaVersion(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	version, err := store.userVersion(ctx)
	if err != nil {
		t.Fatalf("read user version: %v", err)
	}

	if version != 3 {
		t.Fatalf("expected schema version 3, got %d", version)
	}
}

func TestStoreCompactionLifecycle(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	created, err := store.CreateSession(ctx, session.CreateParams{
		Name:       "compaction",
		WorkingDir: t.TempDir(),
		Provider:   "openai-codex",
		Model:      "gpt-5-codex",
		Type:       session.TypeTerminal,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	first, err := store.AppendCompaction(ctx, created.ID, session.CompactionParams{
		Summary:            "initial summary",
		FirstKeptMessageID: "msg_1",
		TokensBefore:       1234,
		Trigger:            session.CompactionTriggerThreshold,
	})
	if err != nil {
		t.Fatalf("append first compaction: %v", err)
	}

	second, err := store.AppendCompaction(ctx, created.ID, session.CompactionParams{
		Summary:            "latest summary",
		FirstKeptMessageID: "msg_2",
		TokensBefore:       2345,
		Trigger:            session.CompactionTriggerOverflow,
	})
	if err != nil {
		t.Fatalf("append second compaction: %v", err)
	}

	if first.ID == second.ID {
		t.Fatalf("expected unique compaction ids")
	}

	latest, err := store.GetLatestCompaction(ctx, created.ID)
	if err != nil {
		t.Fatalf("get latest compaction: %v", err)
	}

	if latest.ID != second.ID {
		t.Fatalf("expected latest compaction %q, got %q", second.ID, latest.ID)
	}
	if latest.Trigger != session.CompactionTriggerOverflow {
		t.Fatalf("expected overflow trigger, got %q", latest.Trigger)
	}
}

func TestStoreLatestCompactionNotFound(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	created, err := store.CreateSession(ctx, session.CreateParams{
		Name:       "no-compaction",
		WorkingDir: t.TempDir(),
		Provider:   "openai-codex",
		Model:      "gpt-5-codex",
		Type:       session.TypeTerminal,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if _, err := store.GetLatestCompaction(ctx, created.ID); err != session.ErrCompactionNotFound {
		t.Fatalf("expected ErrCompactionNotFound, got %v", err)
	}
}

func TestStoreCompactionPreservesConversationHistory(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	created, err := store.CreateSession(ctx, session.CreateParams{
		Name:       "history",
		WorkingDir: t.TempDir(),
		Provider:   "openai-codex",
		Model:      "gpt-5-codex",
		Type:       session.TypeTerminal,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	original := conversation.NewMessage(conversation.RoleUser, conversation.Text("keep this"))
	if _, err := store.AddMessage(ctx, created.ID, original); err != nil {
		t.Fatalf("add message: %v", err)
	}

	if _, err := store.AppendCompaction(ctx, created.ID, session.CompactionParams{
		Summary:            "summary",
		FirstKeptMessageID: created.ID,
		TokensBefore:       99,
		Trigger:            session.CompactionTriggerManual,
	}); err != nil {
		t.Fatalf("append compaction: %v", err)
	}

	replayed, err := store.ReplayConversation(ctx, created.ID)
	if err != nil {
		t.Fatalf("replay conversation: %v", err)
	}

	if got := len(replayed.Messages); got != 1 {
		t.Fatalf("expected 1 message after compaction artifact, got %d", got)
	}
	if got := replayed.Messages[0].Content[0].Text.Text; got != "keep this" {
		t.Fatalf("expected preserved conversation content, got %q", got)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "sessions.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func TestStoreUpdateSessionSelection(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	created, err := store.CreateSession(ctx, session.CreateParams{
		Name:       "selection",
		WorkingDir: t.TempDir(),
		Provider:   "openai-codex",
		Model:      "gpt-5-codex",
		Type:       session.TypeTerminal,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	updated, err := store.UpdateSessionSelection(ctx, created.ID, "openai-codex", "gpt-5.3-codex")
	if err != nil {
		t.Fatalf("update selection: %v", err)
	}
	if updated.Model != "gpt-5.3-codex" {
		t.Fatalf("expected updated model, got %q", updated.Model)
	}
}
