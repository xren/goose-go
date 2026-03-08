package conversation

import (
	"encoding/json"
	"testing"
)

func TestConversationAppendAndValidate(t *testing.T) {
	conv := NewConversation()

	msg := NewMessage(
		RoleAssistant,
		Text("ready"),
		ToolRequest("tool_1", "shell", json.RawMessage(`{"command":"pwd"}`)),
		ToolResponse("tool_1", false, []ToolResult{{Type: "text", Text: "ok"}}, nil),
	)

	if err := conv.Append(msg); err != nil {
		t.Fatalf("append message: %v", err)
	}

	if len(conv.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(conv.Messages))
	}

	if err := conv.Validate(); err != nil {
		t.Fatalf("validate conversation: %v", err)
	}
}

func TestContentValidationRejectsInvalidShape(t *testing.T) {
	msg := NewMessage(RoleUser, Content{Type: ContentTypeToolRequest})

	if err := msg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
