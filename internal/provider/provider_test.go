package provider

import (
	"encoding/json"
	"errors"
	"testing"

	"goose-go/internal/conversation"
)

func TestRequestValidate(t *testing.T) {
	temp := 0.3
	req := Request{
		SessionID:    "sess_123",
		SystemPrompt: "You are helpful.",
		Messages: []conversation.Message{
			conversation.NewMessage(conversation.RoleUser, conversation.Text("hello")),
		},
		Tools: []ToolDefinition{
			{
				Name:        "shell",
				Description: "Run a shell command",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
		},
		Model: ModelConfig{
			Provider:        "openai-codex",
			Model:           "gpt-5-codex",
			Temperature:     &temp,
			MaxOutputTokens: 512,
		},
	}

	if err := req.Validate(); err != nil {
		t.Fatalf("validate request: %v", err)
	}
}

func TestRequestValidateRejectsInvalidMessage(t *testing.T) {
	req := Request{
		Messages: []conversation.Message{
			{ID: "msg_1", Role: conversation.RoleUser},
		},
		Model: ModelConfig{
			Provider: "openai-codex",
			Model:    "gpt-5-codex",
		},
	}

	if err := req.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestModelConfigValidateRejectsInvalidValues(t *testing.T) {
	temp := -0.1
	config := ModelConfig{
		Provider:        "openai-codex",
		Model:           "gpt-5-codex",
		Temperature:     &temp,
		MaxOutputTokens: -1,
	}

	if err := config.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestEventValidate(t *testing.T) {
	tests := []struct {
		name  string
		event Event
	}{
		{
			name: "text delta",
			event: Event{
				Type:  EventTypeTextDelta,
				Delta: "hel",
			},
		},
		{
			name: "message complete",
			event: Event{
				Type: EventTypeMessageComplete,
				Message: ptr(conversation.NewMessage(
					conversation.RoleAssistant,
					conversation.Text("hello"),
				)),
			},
		},
		{
			name: "usage",
			event: Event{
				Type:  EventTypeUsage,
				Usage: &Usage{InputTokens: 10, OutputTokens: 20, TotalTokens: 30},
			},
		},
		{
			name: "done",
			event: Event{
				Type: EventTypeDone,
			},
		},
		{
			name: "error",
			event: Event{
				Type: EventTypeError,
				Err:  errors.New("boom"),
			},
		},
	}

	for _, test := range tests {
		if err := test.event.Validate(); err != nil {
			t.Fatalf("%s: validate event: %v", test.name, err)
		}
	}
}

func TestEventValidateRejectsMissingPayload(t *testing.T) {
	event := Event{Type: EventTypeUsage}

	if err := event.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func ptr[T any](value T) *T {
	return &value
}
