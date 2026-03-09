package grep

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goose-go/internal/tools"
)

func TestGrepRunSuccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("alpha\nbeta\nalphabet\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := New().Run(context.Background(), tools.Call{
		ID:                "call_1",
		Name:              "grep",
		DefaultWorkingDir: dir,
		Arguments:         json.RawMessage(`{"path":".","pattern":"alpha"}`),
	})
	if err != nil {
		t.Fatalf("run grep: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "note.txt:1: alpha") || !strings.Contains(result.Content[0].Text, "note.txt:3: alphabet") {
		t.Fatalf("expected matches, got %q", result.Content[0].Text)
	}
}

func TestGrepRejectsInvalidPattern(t *testing.T) {
	_, err := New().Run(context.Background(), tools.Call{
		ID:        "call_1",
		Name:      "grep",
		Arguments: json.RawMessage(`{"pattern":"["}`),
	})
	if err == nil {
		t.Fatal("expected invalid pattern error")
	}
}
