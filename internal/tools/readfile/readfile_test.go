package readfile

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goose-go/internal/tools"
)

func TestReadFileRunSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := New().Run(context.Background(), tools.Call{
		ID:                "call_1",
		Name:              "read_file",
		DefaultWorkingDir: dir,
		Arguments:         json.RawMessage(`{"path":"note.txt"}`),
	})
	if err != nil {
		t.Fatalf("run read_file: %v", err)
	}
	if result.ToolCallID != "call_1" {
		t.Fatalf("expected tool call id call_1, got %q", result.ToolCallID)
	}
	if got := result.Content[0].Text; got != "hello world" {
		t.Fatalf("expected file contents, got %q", got)
	}

	var structured StructuredResult
	if err := json.Unmarshal(result.Structured, &structured); err != nil {
		t.Fatalf("decode structured result: %v", err)
	}
	if structured.Path != filepath.Clean(path) {
		t.Fatalf("expected path %q, got %q", path, structured.Path)
	}
	if structured.Truncated {
		t.Fatal("expected untruncated result")
	}
}

func TestReadFileRunTruncatesLargeFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("a", 32)), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := New().Run(context.Background(), tools.Call{
		ID:                "call_1",
		Name:              "read_file",
		DefaultWorkingDir: dir,
		Arguments:         json.RawMessage(`{"path":"large.txt","max_bytes":8}`),
	})
	if err != nil {
		t.Fatalf("run read_file: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "[truncated]") {
		t.Fatalf("expected truncated marker, got %q", result.Content[0].Text)
	}

	var structured StructuredResult
	if err := json.Unmarshal(result.Structured, &structured); err != nil {
		t.Fatalf("decode structured result: %v", err)
	}
	if !structured.Truncated {
		t.Fatal("expected truncated structured result")
	}
	if structured.ReadBytes != 8 {
		t.Fatalf("expected read bytes 8, got %d", structured.ReadBytes)
	}
}

func TestReadFileRunRejectsMissingPath(t *testing.T) {
	_, err := New().Run(context.Background(), tools.Call{
		ID:        "call_1",
		Name:      "read_file",
		Arguments: json.RawMessage(`{}`),
	})
	if err == nil {
		t.Fatal("expected missing path error")
	}
}

func TestReadFileRunRejectsDirectories(t *testing.T) {
	dir := t.TempDir()
	_, err := New().Run(context.Background(), tools.Call{
		ID:                "call_1",
		Name:              "read_file",
		DefaultWorkingDir: dir,
		Arguments:         json.RawMessage(`{"path":"."}`),
	})
	if err == nil {
		t.Fatal("expected directory error")
	}
}
