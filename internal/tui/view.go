package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	parts := make([]string, 0, 8)
	if panel := m.approvalPanel(); panel != "" {
		parts = append(parts, panel)
	}
	if panel := m.sessionPickerPanel(); panel != "" {
		parts = append(parts, panel)
	}
	if panel := m.modelPickerPanel(); panel != "" {
		parts = append(parts, panel)
	}
	if panel := m.themePickerPanel(); panel != "" {
		parts = append(parts, panel)
	}
	if preview := m.previewView(); preview != "" {
		parts = append(parts, preview)
	}
	parts = append(parts, m.composerView())
	parts = append(parts, m.statusView())
	if m.errorText != "" {
		parts = append(parts, m.errorTextStyle().Render(m.errorText))
	}
	parts = append(parts, m.headerView(), m.metaView(), m.footerStyle().Render(m.footerText()))
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *model) layout() {
	if m.width > 0 {
		m.input.Width = max(12, m.width-4)
	}
}

func (m model) previewView() string {
	if strings.TrimSpace(m.liveAssistant) == "" {
		return ""
	}
	width := m.transcriptWidth()
	body := renderMarkdownText(m.theme, m.liveAssistant, max(20, width-2), lipgloss.NewStyle().Foreground(m.theme.AssistantText))
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(m.theme.Border).
		Padding(0, 1)
	if width > 0 {
		style = style.Width(width)
	}
	return style.Render(body)
}

func (m model) headerView() string {
	sessionText := fmt.Sprintf(" session: %s ", fallback(m.sessionID, "new"))
	modelProvider, modelName := m.runtime.ProviderModel()
	right := fmt.Sprintf(" %s / %s ", modelProvider, modelName)
	leftStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Text).Background(m.theme.Accent)
	rightStyle := lipgloss.NewStyle().Foreground(m.theme.Text).Background(m.theme.Border)
	if m.width <= 0 {
		return lipgloss.JoinHorizontal(lipgloss.Top, leftStyle.Render(sessionText), rightStyle.Render(right))
	}
	remaining := max(0, m.width-lipgloss.Width(sessionText)-lipgloss.Width(right))
	fill := lipgloss.NewStyle().Background(m.theme.Border).Render(strings.Repeat(" ", remaining))
	return leftStyle.Render(sessionText) + fill + rightStyle.Render(right)
}

func (m model) metaView() string {
	cwd := fallback(m.workingDir, "-")
	left := fmt.Sprintf(" cwd: %s ", cwd)
	right := fmt.Sprintf(" theme: %s ", m.theme.Name)
	style := lipgloss.NewStyle().Foreground(m.theme.FooterText).Background(m.theme.SelectedBG)
	if m.width <= 0 {
		return style.Render(left + right)
	}
	remaining := max(0, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return style.Render(left + strings.Repeat(" ", remaining) + right)
}

func (m model) statusView() string {
	color := m.theme.StatusIdle
	switch m.status {
	case "running", "starting", "loading session", "switching model", "resolving approval", "interrupting":
		color = m.theme.StatusRunning
	case "awaiting approval", "select model", "select session", "select theme":
		color = m.theme.StatusWaiting
	case "failed":
		color = m.theme.StatusError
	case "completed":
		color = m.theme.Success
	case "interrupted":
		color = m.theme.Warning
	}
	return lipgloss.NewStyle().Foreground(color).Padding(0, 1).Render(fmt.Sprintf("status: %s", m.status))
}

func (m model) errorTextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(m.theme.Error).Padding(0, 1)
}

func (m model) footerStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(m.theme.FooterMuted).Padding(0, 1)
}

func (m model) composerActive() bool {
	return !m.running && !m.sessions.Open && !m.picker.Open && !m.themes.Open && m.approval.Request == nil
}

func (m model) composerView() string {
	input := m.input
	active := m.composerActive()
	if active {
		input.Focus()
		input.PromptStyle = lipgloss.NewStyle().Foreground(m.theme.BorderActive).Bold(true)
		input.TextStyle = lipgloss.NewStyle().Foreground(m.theme.Text)
		input.PlaceholderStyle = lipgloss.NewStyle().Foreground(m.theme.PanelHint)
		input.Cursor.Style = lipgloss.NewStyle().Foreground(m.theme.BorderActive)
	} else {
		input.Blur()
		input.PromptStyle = lipgloss.NewStyle().Foreground(m.theme.Muted)
		input.TextStyle = lipgloss.NewStyle().Foreground(m.theme.FooterText)
		input.PlaceholderStyle = lipgloss.NewStyle().Foreground(m.theme.Dim)
		input.Cursor.Style = lipgloss.NewStyle().Foreground(m.theme.Muted)
	}

	bg := m.theme.UserBG
	fg := m.theme.FooterText
	if active {
		bg = m.theme.SelectedBG
		fg = m.theme.Text
	}

	style := lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Padding(0, 1)
	if m.width > 0 {
		style = style.Width(m.width)
	}
	return style.Render(input.View())
}

func (m model) sessionPickerMetrics() (visibleItems int, panelLines int) {
	if !m.sessions.Open {
		return 0, 0
	}
	if m.height <= 0 {
		visibleItems = min(max(1, len(m.sessions.Items)), 8)
	} else {
		controlLines := 5
		panelReserve := 0
		if m.approval.Request != nil {
			panelReserve += 7
			if m.approval.Err != "" {
				panelReserve++
			}
		}
		maxPanelLines := m.height - controlLines - panelReserve
		if maxPanelLines < 6 {
			maxPanelLines = 6
		}
		visibleItems = max(1, (maxPanelLines-3)/2)
	}
	visibleItems = min(visibleItems, max(1, len(m.sessions.Items)))
	panelLines = max(6, visibleItems*2+3)
	if m.sessions.Err != "" {
		panelLines++
	}
	if m.sessions.Busy {
		panelLines++
	}
	return visibleItems, panelLines
}

func pickerWindow(selected int, total int, visible int) (start int, end int) {
	if total <= 0 || visible <= 0 {
		return 0, 0
	}
	if total <= visible {
		return 0, total
	}
	start = selected - visible/2
	if start < 0 {
		start = 0
	}
	if start > total-visible {
		start = total - visible
	}
	end = start + visible
	return start, end
}
