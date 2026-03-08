package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"goose-go/internal/conversation"
	"goose-go/internal/tools"
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
			upsertToolGroup(items, tools.Call{
				ID:        content.ToolRequest.ID,
				Name:      content.ToolRequest.Name,
				Arguments: content.ToolRequest.Arguments,
			}, "requested")
		case conversation.ContentTypeToolResponse:
			if content.ToolResponse == nil {
				continue
			}
			upsertToolResult(items, *content.ToolResponse)
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

func upsertToolGroup(items *[]transcriptItem, call tools.Call, status string) {
	index := findToolGroup(*items, call.ID)
	group := transcriptItem{
		Kind:   kindTool,
		Key:    call.ID,
		Prefix: fmt.Sprintf("tool[%s]", call.Name),
		Text:   renderToolGroup(status, compactArgs(call.Arguments), "", false),
		Meta:   status,
	}
	if index >= 0 {
		(*items)[index] = group
		return
	}
	*items = append(*items, group)
}

func markToolGroupRunning(items *[]transcriptItem, call tools.Call) {
	index := findToolGroup(*items, call.ID)
	if index < 0 {
		upsertToolGroup(items, call, "running")
		return
	}
	item := (*items)[index]
	item.Meta = "running"
	item.Text = renderToolGroup("running", extractToolArgs(item.Text), extractToolOutput(item.Text), strings.Contains(item.Text, "status: error"))
	(*items)[index] = item
}

func upsertToolResult(items *[]transcriptItem, response conversation.ToolResponseContent) {
	index := findToolGroup(*items, response.ID)
	output := joinToolResults(response.Content)
	status := "completed"
	if response.IsError {
		status = "error"
	}
	if index < 0 {
		prefix := "tool"
		text := renderToolGroup(status, "{}", output, response.IsError)
		*items = append(*items, transcriptItem{Kind: kindTool, Key: response.ID, Prefix: prefix, Text: text, Meta: status})
		return
	}
	item := (*items)[index]
	item.Meta = status
	item.Text = renderToolGroup(status, extractToolArgs(item.Text), output, response.IsError)
	(*items)[index] = item
}

func joinToolResults(results []conversation.ToolResult) string {
	parts := make([]string, 0, len(results))
	for _, result := range results {
		if strings.TrimSpace(result.Text) == "" {
			continue
		}
		parts = append(parts, result.Text)
	}
	return strings.Join(parts, "\n")
}

func renderToolGroup(status string, args string, output string, isError bool) string {
	lines := []string{fmt.Sprintf("status: %s", status)}
	if args == "" {
		args = "{}"
	}
	lines = append(lines, "args: "+args)
	if strings.TrimSpace(output) != "" {
		label := "output:"
		if isError {
			label = "error:"
		}
		lines = append(lines, label, output)
	}
	return strings.Join(lines, "\n")
}

func findToolGroup(items []transcriptItem, callID string) int {
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].Key == callID {
			return i
		}
	}
	return -1
}

func extractToolArgs(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "args: ") {
			return strings.TrimPrefix(line, "args: ")
		}
	}
	return "{}"
}

func extractToolOutput(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line == "output:" || line == "error:" {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return ""
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
