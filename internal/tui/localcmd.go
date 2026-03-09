package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"goose-go/internal/app"
	tuitheme "goose-go/internal/tui/theme"
)

func (m *model) handleLocalCommand(prompt string) (bool, tea.Cmd) {
	trimmed := strings.TrimSpace(prompt)
	switch trimmed {
	case "/help":
		m.status = "idle"
		return true, m.printItemsCmd([]transcriptItem{
			transcriptItem{Kind: kindSystem, Prefix: "system", Text: "/help"},
			transcriptItem{Kind: kindSystem, Prefix: "system", Text: "commands:\n/model\n/theme\n/sessions\n/session\n/context\n/debug\n/new\n/help"},
		})
	case "/context":
		m.contextPanel.Open = !m.contextPanel.Open
		m.contextPanel.Err = ""
		if !m.contextPanel.Open {
			m.contextPanel.Busy = false
			m.layout()
			return true, nil
		}
		m.status = "idle"
		m.layout()
		return true, m.refreshContextCmd()
	case "/session":
		providerName, modelName := m.runtime.ProviderModel()
		sessionID := fallback(m.sessionID, "new")
		cwd := fallback(m.workingDir, "-")
		debugMode := "off"
		if m.debug {
			debugMode = "on"
		}
		m.status = "idle"
		return true, m.printItemsCmd([]transcriptItem{
			transcriptItem{Kind: kindSystem, Prefix: "system", Text: "/session"},
			transcriptItem{Kind: kindSystem, Prefix: "system", Text: fmt.Sprintf("session: %s\ncwd: %s\nprovider: %s\nmodel: %s\ntheme: %s\ndebug: %s", sessionID, cwd, providerName, modelName, m.theme.Name, debugMode)},
		})
	case "/debug":
		m.debug = !m.debug
		m.status = "idle"
		mode := "off"
		if m.debug {
			mode = "on"
		}
		return true, m.printItemsCmd([]transcriptItem{
			transcriptItem{Kind: kindSystem, Prefix: "system", Text: "/debug"},
			transcriptItem{Kind: kindSystem, Prefix: "system", Text: fmt.Sprintf("debug mode: %s", mode)},
		})
	case "/new":
		m.status = "idle"
		m.errorText = ""
		m.sessionID = ""
		m.workingDir = m.runtime.WorkingDir()
		m.liveAssistant = ""
		m.activeTools = nil
		m.approval = approvalViewState{}
		m.picker = modelPickerState{}
		m.sessions = sessionPickerState{}
		m.layout()
		cmd := m.printItemsCmd([]transcriptItem{
			{Kind: kindSystem, Prefix: "system", Text: "/new"},
			{Kind: kindSystem, Prefix: "system", Text: "started a new session"},
		})
		if m.contextPanel.Open {
			return true, tea.Batch(cmd, m.refreshContextCmd())
		}
		return true, cmd
	case "/theme":
		m.themes = themePickerState{
			Open:     true,
			Items:    tuitheme.Available(),
			Selected: selectedThemeIndex(tuitheme.Available(), m.theme.Name),
		}
		m.status = "select theme"
		m.layout()
		return true, nil
	}

	providerName, modelName := m.runtime.ProviderModel()
	cmd, ok := app.LocalCommand(prompt, providerName, modelName)
	if !ok {
		return false, nil
	}
	m.status = "idle"
	return true, m.printItemsCmd([]transcriptItem{
		transcriptItem{Kind: kindSystem, Prefix: "system", Text: prompt},
		transcriptItem{Kind: kindSystem, Prefix: "system", Text: cmd.Output},
	})
}
