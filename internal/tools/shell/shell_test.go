package shell

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"goose-go/internal/tools"
)

func TestShellRunSuccess(t *testing.T) {
	tool := New()

	result, err := tool.Run(context.Background(), tools.Call{
		ID:        "call_1",
		Name:      "shell",
		Arguments: json.RawMessage(`{"command":"printf pong"}`),
	})
	if err != nil {
		t.Fatalf("run shell: %v", err)
	}

	if result.ToolCallID != "call_1" {
		t.Fatalf("expected tool call id call_1, got %q", result.ToolCallID)
	}
	if result.IsError {
		t.Fatal("expected success result")
	}
	if got := result.Content[0].Text; got != "pong" {
		t.Fatalf("expected pong output, got %q", got)
	}
}

func TestShellRunFailureExitCode(t *testing.T) {
	tool := New()

	result, err := tool.Run(context.Background(), tools.Call{
		ID:        "call_1",
		Name:      "shell",
		Arguments: json.RawMessage(`{"command":"printf nope && exit 7"}`),
	})
	if err != nil {
		t.Fatalf("run shell: %v", err)
	}

	if !result.IsError {
		t.Fatal("expected error result")
	}

	var structured StructuredResult
	if err := json.Unmarshal(result.Structured, &structured); err != nil {
		t.Fatalf("decode structured result: %v", err)
	}
	if structured.ExitCode != 7 {
		t.Fatalf("expected exit code 7, got %d", structured.ExitCode)
	}
}

func TestShellRunWorkingDir(t *testing.T) {
	tool := New()
	dir := t.TempDir()

	result, err := tool.Run(context.Background(), tools.Call{
		ID:   "call_1",
		Name: "shell",
		Arguments: mustJSON(t, Arguments{
			Command:    "pwd",
			WorkingDir: dir,
		}),
	})
	if err != nil {
		t.Fatalf("run shell: %v", err)
	}

	if got := strings.TrimSpace(result.Content[0].Text); filepath.Clean(got) != dir {
		t.Fatalf("expected working dir %q, got %q", dir, got)
	}
}

func TestShellRunRejectsInvalidArguments(t *testing.T) {
	tool := New()

	_, err := tool.Run(context.Background(), tools.Call{
		ID:        "call_1",
		Name:      "shell",
		Arguments: json.RawMessage(`{"working_dir":"/tmp"}`),
	})
	if err == nil {
		t.Fatal("expected invalid argument error")
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}
