package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"goose-go/internal/app"
	"goose-go/internal/conversation"
)

func (m model) controlSurfaceWidth() int {
	if !m.contextPanel.Open || m.width <= 0 {
		return m.width
	}
	return max(32, m.width-m.contextPanelWidth()-1)
}

func (m model) contextPanelWidth() int {
	if m.width <= 0 {
		return 48
	}
	width := m.width * 2 / 5
	width = max(36, min(width, 64))
	if m.width-width-1 < 32 {
		width = max(24, m.width-33)
	}
	if width >= m.width {
		width = max(24, m.width/2)
	}
	return width
}

func (m *model) syncContextViewport() {
	if !m.contextPanel.Open {
		return
	}
	width := max(20, m.contextPanelWidth()-4)
	height := 12
	if m.height > 0 {
		height = max(8, min(20, m.height-6))
	}
	m.contextPanel.Viewport.Width = width
	m.contextPanel.Viewport.Height = height
	m.contextPanel.Viewport.SetContent(renderWrappedText(m.contextPanelText(), width, lipgloss.NewStyle()))
}

func (m *model) refreshContextCmd() tea.Cmd {
	if !m.contextPanel.Open {
		return nil
	}
	m.contextPanel.Busy = true
	m.contextPanel.Err = ""
	return loadContextCmd(m.ctx, m.runtime, m.sessionID)
}

func (m model) contextPanelView() string {
	if !m.contextPanel.Open {
		return ""
	}
	lines := []string{m.panelTitleStyle().Render("Current context")}
	if m.contextPanel.Busy {
		lines = append(lines, m.panelHintStyle().Render("loading context..."))
	}
	if m.contextPanel.Err != "" {
		lines = append(lines, m.errorTextStyle().Render(m.contextPanel.Err))
	}
	lines = append(lines, m.contextPanel.Viewport.View())
	lines = append(lines, m.panelHintStyle().Render("up/down scroll  pgup/pgdown jump  /context close"))
	return m.panelStyle(m.theme.PanelBorder).
		Width(m.contextPanelWidth()).
		Render(strings.Join(lines, "\n"))
}

func (m model) contextPanelText() string {
	return formatContextSnapshot(m.contextPanel.Snapshot)
}

func formatContextSnapshot(snapshot app.ContextSnapshot) string {
	lines := []string{
		"Session",
		fmt.Sprintf("session: %s", fallback(snapshot.SessionID, "new")),
		fmt.Sprintf("cwd: %s", fallback(snapshot.WorkingDir, "-")),
		fmt.Sprintf("provider: %s", fallback(snapshot.Provider, "-")),
		fmt.Sprintf("model: %s", fallback(snapshot.Model, "-")),
		fmt.Sprintf("estimated tokens: %d", snapshot.EstimatedTokens),
		"",
		"Latest compaction",
	}
	if snapshot.LatestCompaction == nil {
		lines = append(lines, "none")
	} else {
		lines = append(lines,
			fmt.Sprintf("trigger: %s", snapshot.LatestCompaction.Trigger),
			fmt.Sprintf("tokens before: %d", snapshot.LatestCompaction.TokensBefore),
			fmt.Sprintf("first kept message: %s", snapshot.LatestCompaction.FirstKeptMessageID),
			fmt.Sprintf("created: %s", time.Unix(snapshot.LatestCompaction.CreatedAt, 0).UTC().Format(time.RFC3339)),
		)
	}

	lines = append(lines,
		"",
		"System prompt",
		fallback(strings.TrimSpace(snapshot.SystemPrompt), "(none)"),
		"",
		"Active messages",
	)
	if len(snapshot.ActiveMessages) == 0 {
		lines = append(lines, "no conversation yet")
	} else {
		for _, message := range snapshot.ActiveMessages {
			lines = append(lines, formatContextMessage(message))
		}
	}

	return strings.Join(lines, "\n")
}

func formatContextMessage(message conversation.Message) string {
	lines := []string{fmt.Sprintf("[%s] %s", strings.ToUpper(string(message.Role)), message.ID)}
	for _, item := range message.Content {
		switch item.Type {
		case conversation.ContentTypeText:
			if item.Text != nil {
				lines = append(lines, item.Text.Text)
			}
		case conversation.ContentTypeToolRequest:
			if item.ToolRequest != nil {
				lines = append(lines, fmt.Sprintf("tool_request(%s): %s", item.ToolRequest.Name, compactJSON(item.ToolRequest.Arguments)))
			}
		case conversation.ContentTypeToolResponse:
			if item.ToolResponse != nil {
				lines = append(lines, formatToolResponse(item.ToolResponse))
			}
		case conversation.ContentTypeSystemNotification:
			if item.SystemNotification != nil {
				lines = append(lines, fmt.Sprintf("system_notification(%s): %s", item.SystemNotification.Level, item.SystemNotification.Message))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func formatToolResponse(response *conversation.ToolResponseContent) string {
	if response == nil {
		return ""
	}
	parts := make([]string, 0, len(response.Content))
	for _, item := range response.Content {
		if item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	label := "tool_response"
	if response.IsError {
		label = "tool_response(error)"
	}
	if len(parts) > 0 {
		return label + ": " + strings.Join(parts, "\n")
	}
	if len(response.Structured) > 0 {
		return label + "_structured: " + compactJSON(response.Structured)
	}
	return label + ": <empty>"
}

func compactJSON(data json.RawMessage) string {
	if len(data) == 0 {
		return "{}"
	}
	var out bytes.Buffer
	if err := json.Compact(&out, data); err != nil {
		return string(data)
	}
	return out.String()
}
