package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"goose-go/internal/conversation"
	"goose-go/internal/tools"
	"goose-go/internal/tui/markdown"
	tuitheme "goose-go/internal/tui/theme"
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

func renderItems(theme tuitheme.Palette, items []transcriptItem, width int, showToolDetails bool) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, renderItem(theme, item, width, showToolDetails))
	}
	return strings.Join(lines, "\n")
}

func renderItem(theme tuitheme.Palette, item transcriptItem, width int, showToolDetails bool) string {
	text := strings.TrimRight(item.Text, "\n")
	switch item.Kind {
	case kindUser:
		return renderUserText(text, width, theme)
	case kindAssistant, kindLiveBuffer:
		return renderMarkdownText(theme, text, width, lipgloss.NewStyle().Foreground(theme.AssistantText))
	case kindSystem:
		return renderLabeledMarkdownBlock(
			theme,
			"system>",
			text,
			width,
			lipgloss.NewStyle().Foreground(theme.NoticeText).Bold(true),
			lipgloss.NewStyle().Foreground(theme.SystemText),
		)
	case kindError:
		return renderLabeledBlock(
			"error>",
			text,
			width,
			lipgloss.NewStyle().Foreground(theme.Error).Bold(true),
			lipgloss.NewStyle().Foreground(theme.Error),
		)
	case kindTool:
		return renderToolItem(theme, item, width, showToolDetails)
	default:
		prefix := item.Prefix
		if prefix == "" {
			prefix = string(item.Kind)
		}
		return fmt.Sprintf("%s> %s", prefix, text)
	}
}

func renderUserText(text string, width int, theme tuitheme.Palette) string {
	style := lipgloss.NewStyle().
		Foreground(theme.UserText).
		Background(theme.UserBG)

	if width <= 0 {
		return style.Padding(0, 2).Render(text)
	}

	const padX = 2
	innerWidth := max(8, width-(padX*2))
	wrapped := lipgloss.NewStyle().Width(innerWidth).Render(text)

	padded := make([]string, 0, len(strings.Split(wrapped, "\n")))
	lines := strings.Split(wrapped, "\n")
	for _, line := range lines {
		visible := strings.Repeat(" ", padX) + line
		remaining := max(0, width-lipgloss.Width(visible))
		visible += strings.Repeat(" ", remaining)
		padded = append(padded, style.Render(visible))
	}
	return strings.Join(padded, "\n")
}

func renderWrappedText(text string, width int, style lipgloss.Style) string {
	if width <= 0 {
		return style.Render(text)
	}
	return style.Width(width).Render(text)
}

func renderMarkdownText(theme tuitheme.Palette, text string, width int, base lipgloss.Style) string {
	return markdown.Render(theme, text, width, base)
}

func renderLabeledBlock(label string, text string, width int, labelStyle lipgloss.Style, bodyStyle lipgloss.Style) string {
	if width <= 0 {
		return labelStyle.Render(label) + bodyStyle.Render(" "+text)
	}

	labelWidth := min(12, max(8, lipgloss.Width(label)+1))
	bodyWidth := max(8, width-labelWidth)
	left := labelStyle.Width(labelWidth).Render(label)
	right := bodyStyle.Width(bodyWidth).Render(" " + text)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func renderLabeledMarkdownBlock(theme tuitheme.Palette, label string, text string, width int, labelStyle lipgloss.Style, bodyBase lipgloss.Style) string {
	if width <= 0 {
		return labelStyle.Render(label) + " " + markdown.Render(theme, text, 0, bodyBase)
	}

	labelWidth := min(12, max(8, lipgloss.Width(label)+1))
	bodyWidth := max(8, width-labelWidth)
	rendered := markdown.Render(theme, text, max(8, bodyWidth-1), bodyBase)

	lines := strings.Split(rendered, "\n")
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		leftLabel := ""
		if i == 0 {
			leftLabel = label
		}
		left := labelStyle.Width(labelWidth).Render(leftLabel)
		right := lipgloss.NewStyle().Width(bodyWidth).Render(" " + line)
		out = append(out, lipgloss.JoinHorizontal(lipgloss.Top, left, right))
	}
	return strings.Join(out, "\n")
}

func renderToolItem(theme tuitheme.Palette, item transcriptItem, width int, showToolDetails bool) string {
	status := strings.TrimSpace(item.Meta)
	headerColor := theme.Muted
	bodyColor := theme.ToolOutput
	switch status {
	case "running":
		headerColor = theme.NoticeText
	case "completed":
		headerColor = theme.Muted
	case "error":
		headerColor = theme.Error
		bodyColor = theme.Error
	}

	headline := summarizeToolItem(item)
	if showToolDetails {
		headline = item.Prefix + " • " + status
	}
	header := renderWrappedText(
		headline,
		width,
		lipgloss.NewStyle().Foreground(headerColor),
	)
	if !showToolDetails {
		return header
	}

	bodyWidth := width
	if bodyWidth > 0 {
		bodyWidth = max(12, width-2)
	}
	bodyText := indentBlock(strings.TrimSpace(item.Text), "  ")
	body := renderWrappedText(bodyText, bodyWidth, lipgloss.NewStyle().Foreground(bodyColor))
	return header + "\n" + body
}

func indentBlock(text string, indent string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

func summarizeToolItem(item transcriptItem) string {
	switch item.Prefix {
	case "tool[shell]", "tool":
		return summarizeShellTool(extractToolArgs(item.Text), item.Meta)
	default:
		if strings.TrimSpace(item.Meta) == "" {
			return "Tool activity"
		}
		status := strings.TrimSpace(item.Meta)
		return strings.ToUpper(status[:1]) + status[1:]
	}
}

func summarizeShellTool(args string, status string) string {
	type shellArgs struct {
		Command    string `json:"command"`
		WorkingDir string `json:"working_dir"`
	}
	var parsed shellArgs
	if err := json.Unmarshal([]byte(args), &parsed); err != nil {
		return fallbackToolStatus(status, "Running shell command")
	}
	command := strings.TrimSpace(parsed.Command)
	if command == "" {
		return fallbackToolStatus(status, "Running shell command")
	}

	fields := strings.Fields(command)
	if len(fields) == 0 {
		return fallbackToolStatus(status, "Running shell command")
	}

	switch fields[0] {
	case "cat", "head", "tail":
		if target := lastPathToken(fields[1:]); target != "" {
			return "Reading [" + target + "]"
		}
	case "sed":
		if target := lastPathToken(fields[1:]); target != "" {
			return "Reading [" + target + "]"
		}
	case "grep", "rg":
		if target := lastPathToken(fields[1:]); target != "" {
			return "Searching [" + target + "]"
		}
	case "ls":
		if target := lastPathToken(fields[1:]); target != "" {
			return "Listing [" + target + "]"
		}
		if parsed.WorkingDir != "" {
			return "Listing [" + parsed.WorkingDir + "]"
		}
		return "Listing directory"
	case "pwd":
		return "Checking working directory"
	case "git":
		if len(fields) > 1 && fields[1] == "status" {
			return "Inspecting repository state"
		}
	case "go":
		if len(fields) > 1 && fields[1] == "test" {
			return "Running tests"
		}
	}

	return fallbackToolStatus(status, "Running ["+truncateCommand(command, 48)+"]")
}

func fallbackToolStatus(status string, fallback string) string {
	switch strings.TrimSpace(status) {
	case "completed":
		return fallback
	case "error":
		return "Failed: " + fallback
	default:
		return fallback
	}
}

func lastPathToken(tokens []string) string {
	for i := len(tokens) - 1; i >= 0; i-- {
		token := strings.Trim(tokens[i], "\"'")
		if token == "" || strings.HasPrefix(token, "-") {
			continue
		}
		return token
	}
	return ""
}

func truncateCommand(command string, limit int) string {
	if limit <= 0 || len(command) <= limit {
		return command
	}
	return strings.TrimSpace(command[:limit-3]) + "..."
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
