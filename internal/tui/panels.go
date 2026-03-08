package tui

import (
	"fmt"
	"strings"
)

func (m model) approvalPanel() string {
	if m.approval.Request == nil {
		return ""
	}
	req := m.approval.Request
	lines := []string{
		approvalTitleStyle.Render("Approval required"),
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
		lines = append(lines, approvalErrorStyle.Render(m.approval.Err))
	}
	if m.approval.Busy {
		lines = append(lines, "resolving...")
	} else {
		lines = append(lines, "a/y approve  d/n deny")
	}
	return approvalPanelStyle.Width(max(40, min(m.width, 120))).Render(strings.Join(lines, "\n"))
}

func (m model) modelPickerPanel() string {
	if !m.picker.Open {
		return ""
	}
	lines := []string{pickerTitleStyle.Render("Select model")}
	if len(m.picker.Items) == 0 {
		lines = append(lines, pickerDimStyle.Render("No models available"))
	} else {
		for i, item := range m.picker.Items {
			cursor := "  "
			if i == m.picker.Selected {
				cursor = pickerCursorStyle.Render("> ")
			}
			line := fmt.Sprintf("%s%s (%s)", cursor, item.Model.DisplayName, item.Model.ID)
			if !item.Available {
				reason := item.Reason
				if reason == "" {
					reason = "unavailable"
				}
				line = pickerDimStyle.Render(fmt.Sprintf("%s - %s", line, reason))
			}
			lines = append(lines, line)
		}
	}
	if m.picker.Err != "" {
		lines = append(lines, approvalErrorStyle.Render(m.picker.Err))
	}
	if m.picker.Busy {
		lines = append(lines, pickerDimStyle.Render("switching model..."))
	}
	return pickerPanelStyle.Width(max(40, min(m.width, 120))).Render(strings.Join(lines, "\n"))
}

func (m model) sessionPickerPanel() string {
	if !m.sessions.Open {
		return ""
	}
	lines := []string{sessionTitleStyle.Render("Recent sessions")}
	if len(m.sessions.Items) == 0 {
		lines = append(lines, pickerDimStyle.Render("No sessions found"))
	} else {
		for i, item := range m.sessions.Items {
			cursor := "  "
			if i == m.sessions.Selected {
				cursor = pickerCursorStyle.Render("> ")
			}
			line := fmt.Sprintf("%s%s (%s)", cursor, item.Name, item.ID)
			meta := fmt.Sprintf("%s • %s/%s • %d msgs", item.WorkingDir, item.Provider, item.Model, item.MessageCount)
			lines = append(lines, line, pickerDimStyle.Render("    "+meta))
		}
	}
	if m.sessions.Err != "" {
		lines = append(lines, approvalErrorStyle.Render(m.sessions.Err))
	}
	if m.sessions.Busy {
		lines = append(lines, pickerDimStyle.Render("loading session..."))
	}
	return sessionPanelStyle.Width(max(50, min(m.width, 140))).Render(strings.Join(lines, "\n"))
}

func (m model) footerText() string {
	if m.sessions.Open {
		return "up/down select  enter open  esc close"
	}
	if m.picker.Open {
		return "up/down select  enter choose  esc close"
	}
	if m.approval.Request != nil {
		return "a/y approve  d/n deny  esc/ctrl+c interrupt"
	}
	return "enter submit  ctrl+r sessions  /help commands  esc/ctrl+c interrupt  ctrl+d quit"
}
