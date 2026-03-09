package findfiles

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goose-go/internal/tools"
)

func TestFindFilesRunSuccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, path := range []string{"one.txt", "sub/two.txt", "sub/three.go"} {
		full := filepath.Join(dir, path)
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	result, err := New().Run(context.Background(), tools.Call{
		ID:                "call_1",
		Name:              "find_files",
		DefaultWorkingDir: dir,
		Arguments:         json.RawMessage(`{"path":".","pattern":"*.txt"}`),
	})
	if err != nil {
		t.Fatalf("run find_files: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "one.txt") || !strings.Contains(result.Content[0].Text, "two.txt") {
		t.Fatalf("expected txt matches, got %q", result.Content[0].Text)
	}
}

func TestFindFilesRequiresPattern(t *testing.T) {
	_, err := New().Run(context.Background(), tools.Call{
		ID:        "call_1",
		Name:      "find_files",
		Arguments: json.RawMessage(`{}`),
	})
	if err == nil {
		t.Fatal("expected missing pattern error")
	}
}
