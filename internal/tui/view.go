package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	header := m.headerView()
	cwd := m.metaView()
	status := m.statusView()
	if m.errorText != "" {
		status += "\n" + m.errorTextStyle().Render(m.errorText)
	}
	parts := []string{header, cwd, status, m.viewport.View()}
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
	parts = append(parts, m.input.View(), m.footerStyle().Render(m.footerText()))
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *model) layout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	headerLines := 3
	footerLines := 2
	composerLines := 1
	approvalLines := 0
	if m.approval.Request != nil {
		approvalLines = 7
		if m.approval.Err != "" {
			approvalLines++
		}
	}
	pickerLines := 0
	if m.picker.Open {
		pickerLines = len(m.picker.Items) + 3
		if m.picker.Err != "" {
			pickerLines++
		}
		if pickerLines < 6 {
			pickerLines = 6
		}
	}
	sessionLines := 0
	if m.sessions.Open {
		sessionLines = len(m.sessions.Items)*2 + 3
		if m.sessions.Err != "" {
			sessionLines++
		}
		if sessionLines < 6 {
			sessionLines = 6
		}
	}
	themeLines := 0
	if m.themes.Open {
		themeLines = len(m.themes.Items) + 4
		if themeLines < 6 {
			themeLines = 6
		}
	}
	bodyHeight := m.height - headerLines - footerLines - composerLines - approvalLines - pickerLines - sessionLines - themeLines
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	m.viewport.Width = m.width
	m.viewport.Height = bodyHeight
	m.syncViewport(false)
}

func (m *model) syncViewport(forceBottom bool) {
	wasAtBottom := m.viewport.AtBottom()
	m.viewport.SetContent(renderItems(m.theme, m.items, m.viewport.Width, m.debug))
	if forceBottom || wasAtBottom {
		m.viewport.GotoBottom()
	}
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
