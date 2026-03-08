package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"goose-go/internal/conversation"
)

type Provider interface {
	Stream(ctx context.Context, req Request) (<-chan Event, error)
}

type Request struct {
	SessionID    string
	SystemPrompt string
	Messages     []conversation.Message
	Tools        []ToolDefinition
	Model        ModelConfig
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type ModelConfig struct {
	Provider        string   `json:"provider"`
	Model           string   `json:"model"`
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens int      `json:"max_output_tokens,omitempty"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type EventType string

const (
	EventTypeTextDelta       EventType = "text_delta"
	EventTypeMessageComplete EventType = "message_complete"
	EventTypeUsage           EventType = "usage"
	EventTypeDone            EventType = "done"
	EventTypeError           EventType = "error"
)

type Event struct {
	Type    EventType             `json:"type"`
	Delta   string                `json:"delta,omitempty"`
	Message *conversation.Message `json:"message,omitempty"`
	Usage   *Usage                `json:"usage,omitempty"`
	Err     error                 `json:"-"`
}

func (r Request) Validate() error {
	if err := r.Model.Validate(); err != nil {
		return fmt.Errorf("model: %w", err)
	}

	for i, message := range r.Messages {
		if err := message.Validate(); err != nil {
			return fmt.Errorf("message %d: %w", i, err)
		}
	}

	for i, tool := range r.Tools {
		if err := tool.Validate(); err != nil {
			return fmt.Errorf("tool %d: %w", i, err)
		}
	}

	return nil
}

func (t ToolDefinition) Validate() error {
	if t.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func (m ModelConfig) Validate() error {
	if m.Provider == "" {
		return errors.New("provider is required")
	}
	if m.Model == "" {
		return errors.New("model is required")
	}
	if m.Temperature != nil && *m.Temperature < 0 {
		return errors.New("temperature must be >= 0")
	}
	if m.MaxOutputTokens < 0 {
		return errors.New("max_output_tokens must be >= 0")
	}
	return nil
}

func (e Event) Validate() error {
	switch e.Type {
	case EventTypeTextDelta:
		if e.Delta == "" {
			return errors.New("text_delta requires delta")
		}
	case EventTypeMessageComplete:
		if e.Message == nil {
			return errors.New("message_complete requires message")
		}
		if err := e.Message.Validate(); err != nil {
			return fmt.Errorf("message: %w", err)
		}
	case EventTypeUsage:
		if e.Usage == nil {
			return errors.New("usage requires usage payload")
		}
	case EventTypeDone:
	case EventTypeError:
		if e.Err == nil {
			return errors.New("error requires err")
		}
	default:
		return fmt.Errorf("unsupported event type %q", e.Type)
	}
	return nil
}
