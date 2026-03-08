package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"goose-go/internal/conversation"
)

func TestRegistryRegisterAndDefinitions(t *testing.T) {
	registry := NewRegistry()

	if err := registry.Register(toolStub{
		definition: Definition{Name: "b", Description: "second"},
	}); err != nil {
		t.Fatalf("register tool b: %v", err)
	}
	if err := registry.Register(toolStub{
		definition: Definition{Name: "a", Description: "first"},
	}); err != nil {
		t.Fatalf("register tool a: %v", err)
	}

	defs := registry.Definitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(defs))
	}
	if defs[0].Name != "a" || defs[1].Name != "b" {
		t.Fatalf("expected sorted definitions, got %#v", defs)
	}
}

func TestRegistryRejectsDuplicateTool(t *testing.T) {
	registry := NewRegistry()
	tool := toolStub{definition: Definition{Name: "shell"}}

	if err := registry.Register(tool); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	if err := registry.Register(tool); !errors.Is(err, ErrDuplicateTool) {
		t.Fatalf("expected duplicate tool error, got %v", err)
	}
}

func TestRegistryExecute(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(toolStub{
		definition: Definition{Name: "shell"},
		result: Result{
			ToolCallID: "call_1",
			Content:    []conversation.ToolResult{{Type: "text", Text: "ok"}},
		},
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	result, err := registry.Execute(context.Background(), Call{
		ID:        "call_1",
		Name:      "shell",
		Arguments: json.RawMessage(`{"command":"pwd"}`),
	})
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}
	if result.ToolCallID != "call_1" {
		t.Fatalf("expected tool call id call_1, got %q", result.ToolCallID)
	}
}

func TestRegistryExecuteUnknownTool(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Execute(context.Background(), Call{
		ID:   "call_1",
		Name: "missing",
	})
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("expected tool not found, got %v", err)
	}
}

func TestResultToConversationContent(t *testing.T) {
	result := Result{
		ToolCallID: "call_1",
		IsError:    true,
		Content:    []conversation.ToolResult{{Type: "text", Text: "boom"}},
		Structured: json.RawMessage(`{"exit_code":1}`),
	}

	content := result.ToConversationContent()
	if content.Type != conversation.ContentTypeToolResponse {
		t.Fatalf("expected tool response content, got %q", content.Type)
	}
	if content.ToolResponse.ID != "call_1" {
		t.Fatalf("expected tool response id call_1, got %q", content.ToolResponse.ID)
	}
}

type toolStub struct {
	definition Definition
	result     Result
	err        error
}

func (t toolStub) Definition() Definition {
	return t.definition
}

func (t toolStub) Run(context.Context, Call) (Result, error) {
	return t.result, t.err
}
