package theme

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Name string

const (
	Dark  Name = "dark"
	Light Name = "light"
)

type Palette struct {
	Name string

	Accent       lipgloss.Color
	Muted        lipgloss.Color
	Dim          lipgloss.Color
	Text         lipgloss.Color
	Success      lipgloss.Color
	Warning      lipgloss.Color
	Error        lipgloss.Color
	Border       lipgloss.Color
	BorderActive lipgloss.Color
	SelectedBG   lipgloss.Color

	UserBG        lipgloss.Color
	UserText      lipgloss.Color
	AssistantText lipgloss.Color
	SystemText    lipgloss.Color
	NoticeText    lipgloss.Color

	ToolPendingBG lipgloss.Color
	ToolRunningBG lipgloss.Color
	ToolSuccessBG lipgloss.Color
	ToolErrorBG   lipgloss.Color
	ToolTitle     lipgloss.Color
	ToolOutput    lipgloss.Color

	PanelTitle    lipgloss.Color
	PanelHint     lipgloss.Color
	PanelBorder   lipgloss.Color
	PanelSelected lipgloss.Color

	FooterText    lipgloss.Color
	FooterMuted   lipgloss.Color
	StatusIdle    lipgloss.Color
	StatusRunning lipgloss.Color
	StatusWaiting lipgloss.Color
	StatusError   lipgloss.Color
}

var builtins = map[Name]Palette{
	Dark: {
		Name:          "dark",
		Accent:        lipgloss.Color("#8abeb7"),
		Muted:         lipgloss.Color("#808080"),
		Dim:           lipgloss.Color("#666666"),
		Text:          lipgloss.Color("#e5e5e5"),
		Success:       lipgloss.Color("#b5bd68"),
		Warning:       lipgloss.Color("#f0c674"),
		Error:         lipgloss.Color("#cc6666"),
		Border:        lipgloss.Color("#5f87ff"),
		BorderActive:  lipgloss.Color("#00d7ff"),
		SelectedBG:    lipgloss.Color("#3a3a4a"),
		UserBG:        lipgloss.Color("#343541"),
		UserText:      lipgloss.Color("#f3f3f3"),
		AssistantText: lipgloss.Color("#e9e9e9"),
		SystemText:    lipgloss.Color("#bdbdbd"),
		NoticeText:    lipgloss.Color("#8abeb7"),
		ToolPendingBG: lipgloss.Color("#282832"),
		ToolRunningBG: lipgloss.Color("#243241"),
		ToolSuccessBG: lipgloss.Color("#283228"),
		ToolErrorBG:   lipgloss.Color("#3c2828"),
		ToolTitle:     lipgloss.Color("#f3f3f3"),
		ToolOutput:    lipgloss.Color("#d6d6d6"),
		PanelTitle:    lipgloss.Color("#00d7ff"),
		PanelHint:     lipgloss.Color("#808080"),
		PanelBorder:   lipgloss.Color("#5f87ff"),
		PanelSelected: lipgloss.Color("#3a3a4a"),
		FooterText:    lipgloss.Color("#d0d0d0"),
		FooterMuted:   lipgloss.Color("#808080"),
		StatusIdle:    lipgloss.Color("#808080"),
		StatusRunning: lipgloss.Color("#8abeb7"),
		StatusWaiting: lipgloss.Color("#f0c674"),
		StatusError:   lipgloss.Color("#cc6666"),
	},
	Light: {
		Name:          "light",
		Accent:        lipgloss.Color("#5a8080"),
		Muted:         lipgloss.Color("#6c6c6c"),
		Dim:           lipgloss.Color("#767676"),
		Text:          lipgloss.Color("#1f1f1f"),
		Success:       lipgloss.Color("#588458"),
		Warning:       lipgloss.Color("#9a7326"),
		Error:         lipgloss.Color("#aa5555"),
		Border:        lipgloss.Color("#547da7"),
		BorderActive:  lipgloss.Color("#5a8080"),
		SelectedBG:    lipgloss.Color("#d0d0e0"),
		UserBG:        lipgloss.Color("#e8e8e8"),
		UserText:      lipgloss.Color("#1f1f1f"),
		AssistantText: lipgloss.Color("#111111"),
		SystemText:    lipgloss.Color("#4a4a4a"),
		NoticeText:    lipgloss.Color("#5a8080"),
		ToolPendingBG: lipgloss.Color("#e8e8f0"),
		ToolRunningBG: lipgloss.Color("#dfe9f4"),
		ToolSuccessBG: lipgloss.Color("#e8f0e8"),
		ToolErrorBG:   lipgloss.Color("#f0e8e8"),
		ToolTitle:     lipgloss.Color("#111111"),
		ToolOutput:    lipgloss.Color("#333333"),
		PanelTitle:    lipgloss.Color("#547da7"),
		PanelHint:     lipgloss.Color("#6c6c6c"),
		PanelBorder:   lipgloss.Color("#547da7"),
		PanelSelected: lipgloss.Color("#d0d0e0"),
		FooterText:    lipgloss.Color("#333333"),
		FooterMuted:   lipgloss.Color("#6c6c6c"),
		StatusIdle:    lipgloss.Color("#6c6c6c"),
		StatusRunning: lipgloss.Color("#5a8080"),
		StatusWaiting: lipgloss.Color("#9a7326"),
		StatusError:   lipgloss.Color("#aa5555"),
	},
}

func Available() []Name {
	return []Name{Dark, Light}
}

func Resolve(raw string) (Palette, error) {
	name := Name(strings.TrimSpace(strings.ToLower(raw)))
	if name == "" {
		name = Dark
	}
	p, ok := builtins[name]
	if !ok {
		return Palette{}, fmt.Errorf("unknown theme %q", raw)
	}
	return p, nil
}
