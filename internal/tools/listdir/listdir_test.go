package listdir

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goose-go/internal/tools"
)

func TestListDirRunSuccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	result, err := New().Run(context.Background(), tools.Call{
		ID:                "call_1",
		Name:              "list_dir",
		DefaultWorkingDir: dir,
		Arguments:         json.RawMessage(`{"path":"."}`),
	})
	if err != nil {
		t.Fatalf("run list_dir: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "a.txt") || !strings.Contains(result.Content[0].Text, "subdir/") {
		t.Fatalf("expected directory entries, got %q", result.Content[0].Text)
	}
}

func TestListDirRunTruncates(t *testing.T) {
	dir := t.TempDir()
	for i := range 3 {
		if err := os.WriteFile(filepath.Join(dir, string(rune('a'+i))+".txt"), []byte("x"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	result, err := New().Run(context.Background(), tools.Call{
		ID:                "call_1",
		Name:              "list_dir",
		DefaultWorkingDir: dir,
		Arguments:         json.RawMessage(`{"path":".","max_entries":2}`),
	})
	if err != nil {
		t.Fatalf("run list_dir: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "[truncated]") {
		t.Fatalf("expected truncated marker, got %q", result.Content[0].Text)
	}
}
