package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"goose-go/internal/conversation"
)

func buildTranscriptFromConversation(conv conversation.Conversation) []transcriptItem {
	items := make([]transcriptItem, 0, len(conv.Messages))
	for _, message := range conv.Messages {
		appendMessageItems(&items, message)
	}
	return items
}

func appendMessageItems(items *[]transcriptItem, message conversation.Message) {
	for _, content := range message.Content {
		switch content.Type {
		case conversation.ContentTypeText:
			if content.Text == nil {
				continue
			}
			prefix := string(message.Role)
			kind := kindSystem
			switch message.Role {
			case conversation.RoleUser:
				kind = kindUser
			case conversation.RoleAssistant:
				kind = kindAssistant
			case conversation.RoleTool:
				kind = kindTool
			}
			*items = append(*items, transcriptItem{Kind: kind, Prefix: prefix, Text: content.Text.Text})
		case conversation.ContentTypeToolRequest:
			if content.ToolRequest == nil {
				continue
			}
			*items = append(*items, transcriptItem{Kind: kindSystem, Prefix: "assistant requested tool", Text: fmt.Sprintf("%s %s", content.ToolRequest.Name, compactArgs(content.ToolRequest.Arguments))})
		case conversation.ContentTypeToolResponse:
			if content.ToolResponse == nil {
				continue
			}
			for _, result := range content.ToolResponse.Content {
				*items = append(*items, transcriptItem{Kind: kindTool, Prefix: "tool", Text: result.Text})
			}
		case conversation.ContentTypeSystemNotification:
			if content.SystemNotification == nil {
				continue
			}
			*items = append(*items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: content.SystemNotification.Message})
		}
	}
}

func renderItems(items []transcriptItem, width int) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		prefix := item.Prefix
		if prefix == "" {
			prefix = string(item.Kind)
		}
		text := strings.TrimRight(item.Text, "\n")
		parts := strings.Split(text, "\n")
		for i, part := range parts {
			if i == 0 {
				lines = append(lines, fmt.Sprintf("%s> %s", prefix, part))
				continue
			}
			lines = append(lines, fmt.Sprintf("%s  %s", strings.Repeat(" ", len(prefix)), part))
		}
		if len(parts) == 0 {
			lines = append(lines, prefix+"> ")
		}
	}
	content := strings.Join(lines, "\n")
	if width > 0 {
		return lipgloss.NewStyle().Width(width).Render(content)
	}
	return content
}

func compactArgs(raw json.RawMessage) string {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return "{}"
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return string(raw)
	}
	return string(data)
}
