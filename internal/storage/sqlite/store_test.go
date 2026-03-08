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

	if version != 1 {
		t.Fatalf("expected schema version 1, got %d", version)
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
