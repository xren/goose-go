package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) approvalPanel() string {
	if m.approval.Request == nil {
		return ""
	}
	req := m.approval.Request
	lines := []string{
		m.panelTitleStyle().Render("Approval required"),
		fmt.Sprintf("tool: %s", req.ToolCall.Name),
		fmt.Sprintf("args: %s", compactArgs(req.ToolCall.Arguments)),
	}
	if m.sessionID != "" {
		lines = append(lines, fmt.Sprintf("session: %s", m.sessionID))
	}
	if strings.TrimSpace(m.workingDir) != "" {
		lines = append(lines, fmt.Sprintf("cwd: %s", m.workingDir))
	}
	if m.approval.Err != "" {
		lines = append(lines, m.errorTextStyle().Render(m.approval.Err))
	}
	if m.approval.Busy {
		lines = append(lines, "resolving...")
	} else {
		lines = append(lines, "a/y approve  d/n deny")
	}
	return m.panelStyle(m.theme.Warning).Width(max(40, min(m.width, 120))).Render(strings.Join(lines, "\n"))
}

func (m model) modelPickerPanel() string {
	if !m.picker.Open {
		return ""
	}
	lines := []string{m.panelTitleStyle().Render("Select model")}
	if len(m.picker.Items) == 0 {
		lines = append(lines, m.panelHintStyle().Render("No models available"))
	} else {
		for i, item := range m.picker.Items {
			cursor := "  "
			if i == m.picker.Selected {
				cursor = m.panelCursorStyle().Render("> ")
			}
			line := fmt.Sprintf("%s%s (%s)", cursor, item.Model.DisplayName, item.Model.ID)
			if !item.Available {
				reason := item.Reason
				if reason == "" {
					reason = "unavailable"
				}
				line = m.panelHintStyle().Render(fmt.Sprintf("%s - %s", line, reason))
			}
			lines = append(lines, line)
		}
	}
	if m.picker.Err != "" {
		lines = append(lines, m.errorTextStyle().Render(m.picker.Err))
	}
	if m.picker.Busy {
		lines = append(lines, m.panelHintStyle().Render("switching model..."))
	}
	return m.panelStyle(m.theme.PanelBorder).Width(max(40, min(m.width, 120))).Render(strings.Join(lines, "\n"))
}

func (m model) sessionPickerPanel() string {
	if !m.sessions.Open {
		return ""
	}
	lines := []string{m.panelTitleStyle().Render("Recent sessions")}
	if len(m.sessions.Items) == 0 {
		lines = append(lines, m.panelHintStyle().Render("No sessions found"))
	} else {
		for i, item := range m.sessions.Items {
			cursor := "  "
			if i == m.sessions.Selected {
				cursor = m.panelCursorStyle().Render("> ")
			}
			line := fmt.Sprintf("%s%s (%s)", cursor, item.Name, item.ID)
			meta := fmt.Sprintf("%s • %s/%s • %d msgs", item.WorkingDir, item.Provider, item.Model, item.MessageCount)
			lines = append(lines, line, m.panelHintStyle().Render("    "+meta))
		}
	}
	if m.sessions.Err != "" {
		lines = append(lines, m.errorTextStyle().Render(m.sessions.Err))
	}
	if m.sessions.Busy {
		lines = append(lines, m.panelHintStyle().Render("loading session..."))
	}
	return m.panelStyle(m.theme.PanelBorder).Width(max(50, min(m.width, 140))).Render(strings.Join(lines, "\n"))
}

func (m model) themePickerPanel() string {
	if !m.themes.Open {
		return ""
	}
	lines := []string{m.panelTitleStyle().Render("Select theme")}
	for i, item := range m.themes.Items {
		cursor := "  "
		if i == m.themes.Selected {
			cursor = m.panelCursorStyle().Render("> ")
		}
		line := fmt.Sprintf("%s%s", cursor, item)
		if string(item) == m.theme.Name {
			line = line + " current"
		}
		lines = append(lines, line)
	}
	lines = append(lines, m.panelHintStyle().Render("enter choose  esc close"))
	return m.panelStyle(m.theme.PanelBorder).Width(max(36, min(m.width, 100))).Render(strings.Join(lines, "\n"))
}

func (m model) footerText() string {
	if m.sessions.Open {
		return "up/down select  enter open  esc close"
	}
	if m.picker.Open {
		return "up/down select  enter choose  esc close"
	}
	if m.themes.Open {
		return "up/down select  enter choose  esc close"
	}
	if m.approval.Request != nil {
		return "a/y approve  d/n deny  esc/ctrl+c interrupt"
	}
	return "wheel/pgup/pgdown scroll  home/end jump  enter submit  ctrl+r sessions  /debug toggle  /help commands  esc/ctrl+c interrupt  ctrl+d quit"
}

func (m model) panelStyle(border lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Foreground(m.theme.Text)
}

func (m model) panelTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(m.theme.PanelTitle)
}

func (m model) panelHintStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(m.theme.PanelHint)
}

func (m model) panelCursorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
}
