package conversation

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type ContentType string

const (
	ContentTypeText               ContentType = "text"
	ContentTypeToolRequest        ContentType = "tool_request"
	ContentTypeToolResponse       ContentType = "tool_response"
	ContentTypeSystemNotification ContentType = "system_notification"
)

type Message struct {
	ID        string    `json:"id"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	Content   []Content `json:"content"`
}

type Content struct {
	Type               ContentType                `json:"type"`
	Text               *TextContent               `json:"text,omitempty"`
	ToolRequest        *ToolRequestContent        `json:"tool_request,omitempty"`
	ToolResponse       *ToolResponseContent       `json:"tool_response,omitempty"`
	SystemNotification *SystemNotificationContent `json:"system_notification,omitempty"`
}

type TextContent struct {
	Text string `json:"text"`
}

type ToolRequestContent struct {
	ID         string          `json:"id"`
	ProviderID string          `json:"provider_id,omitempty"`
	Name       string          `json:"name"`
	Arguments  json.RawMessage `json:"arguments,omitempty"`
}

type ToolResponseContent struct {
	ID         string          `json:"id"`
	IsError    bool            `json:"is_error"`
	Content    []ToolResult    `json:"content,omitempty"`
	Structured json.RawMessage `json:"structured,omitempty"`
}

type ToolResult struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type SystemNotificationContent struct {
	Level   string          `json:"level"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type Conversation struct {
	Messages []Message `json:"messages"`
}

func NewConversation() Conversation {
	return Conversation{Messages: []Message{}}
}

func NewMessage(role Role, content ...Content) Message {
	return Message{
		ID:        newID("msg"),
		Role:      role,
		CreatedAt: time.Now().UTC(),
		Content:   content,
	}
}

func Text(text string) Content {
	return Content{
		Type: ContentTypeText,
		Text: &TextContent{Text: text},
	}
}

func ToolRequest(id, name string, arguments json.RawMessage) Content {
	return ToolRequestWithProviderID(id, "", name, arguments)
}

func ToolRequestWithProviderID(id, providerID, name string, arguments json.RawMessage) Content {
	return Content{
		Type: ContentTypeToolRequest,
		ToolRequest: &ToolRequestContent{
			ID:         id,
			ProviderID: providerID,
			Name:       name,
			Arguments:  arguments,
		},
	}
}

func ToolResponse(id string, isError bool, content []ToolResult, structured json.RawMessage) Content {
	return Content{
		Type: ContentTypeToolResponse,
		ToolResponse: &ToolResponseContent{
			ID:         id,
			IsError:    isError,
			Content:    content,
			Structured: structured,
		},
	}
}

func SystemNotification(level, message string, data json.RawMessage) Content {
	return Content{
		Type: ContentTypeSystemNotification,
		SystemNotification: &SystemNotificationContent{
			Level:   level,
			Message: message,
			Data:    data,
		},
	}
}

func (c *Conversation) Append(message Message) error {
	if err := message.Validate(); err != nil {
		return err
	}
	c.Messages = append(c.Messages, message)
	return nil
}

func (c Conversation) Clone() Conversation {
	out := make([]Message, len(c.Messages))
	copy(out, c.Messages)
	return Conversation{Messages: out}
}

func (c Conversation) Validate() error {
	for i := range c.Messages {
		if err := c.Messages[i].Validate(); err != nil {
			return fmt.Errorf("message %d: %w", i, err)
		}
	}
	return nil
}

func (m Message) Validate() error {
	if m.ID == "" {
		return errors.New("id is required")
	}
	if m.Role == "" {
		return errors.New("role is required")
	}
	if len(m.Content) == 0 {
		return errors.New("content is required")
	}
	for i := range m.Content {
		if err := m.Content[i].Validate(); err != nil {
			return fmt.Errorf("content %d: %w", i, err)
		}
	}
	return nil
}

func (c Content) Validate() error {
	switch c.Type {
	case ContentTypeText:
		if c.Text == nil || c.Text.Text == "" {
			return errors.New("text content requires text")
		}
	case ContentTypeToolRequest:
		if c.ToolRequest == nil || c.ToolRequest.ID == "" || c.ToolRequest.Name == "" {
			return errors.New("tool request requires id and name")
		}
	case ContentTypeToolResponse:
		if c.ToolResponse == nil || c.ToolResponse.ID == "" {
			return errors.New("tool response requires id")
		}
	case ContentTypeSystemNotification:
		if c.SystemNotification == nil || c.SystemNotification.Message == "" {
			return errors.New("system notification requires message")
		}
	default:
		return fmt.Errorf("unsupported content type %q", c.Type)
	}
	return nil
}
