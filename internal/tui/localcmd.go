package tui

import (
	"fmt"
	"strings"

	"goose-go/internal/app"
)

func (m *model) handleLocalCommand(prompt string) bool {
	trimmed := strings.TrimSpace(prompt)
	switch trimmed {
	case "/help":
		m.status = "idle"
		m.items = append(m.items,
			transcriptItem{Kind: kindSystem, Prefix: "system", Text: "/help"},
			transcriptItem{Kind: kindSystem, Prefix: "system", Text: "commands:\n/model\n/sessions\n/session\n/new\n/help"},
		)
		return true
	case "/session":
		providerName, modelName := m.runtime.ProviderModel()
		sessionID := fallback(m.sessionID, "new")
		cwd := fallback(m.workingDir, "-")
		m.status = "idle"
		m.items = append(m.items,
			transcriptItem{Kind: kindSystem, Prefix: "system", Text: "/session"},
			transcriptItem{Kind: kindSystem, Prefix: "system", Text: fmt.Sprintf("session: %s\ncwd: %s\nprovider: %s\nmodel: %s", sessionID, cwd, providerName, modelName)},
		)
		return true
	case "/new":
		m.status = "idle"
		m.errorText = ""
		m.sessionID = ""
		m.workingDir = m.runtime.WorkingDir()
		m.items = []transcriptItem{
			{Kind: kindSystem, Prefix: "system", Text: "/new"},
			{Kind: kindSystem, Prefix: "system", Text: "started a new session"},
		}
		m.approval = approvalViewState{}
		m.picker = modelPickerState{}
		m.sessions = sessionPickerState{}
		m.layout()
		return true
	}

	providerName, modelName := m.runtime.ProviderModel()
	cmd, ok := app.LocalCommand(prompt, providerName, modelName)
	if !ok {
		return false
	}
	m.status = "idle"
	m.items = append(m.items,
		transcriptItem{Kind: kindSystem, Prefix: "system", Text: prompt},
		transcriptItem{Kind: kindSystem, Prefix: "system", Text: cmd.Output},
	)
	return true
}
