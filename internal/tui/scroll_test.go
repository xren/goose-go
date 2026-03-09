package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	tuitheme "goose-go/internal/tui/theme"
)

func TestRenderToolItemDoesNotExceedViewportWidth(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	item := transcriptItem{
		Kind:   kindTool,
		Prefix: "tool[shell]",
		Meta:   "completed",
		Text: renderToolGroup(
			"completed",
			`{"command":"sed -n '1,50p' docs/invariants.md","working_dir":"."}`,
			"The root architecture doc defines package-level boundaries. Concrete subsystem behavior is documented separately.\n\nThis is additional wrapped output that should stay inside the card width.",
			false,
		),
	}

	rendered := renderToolItem(theme, item, 80, true)
	for _, line := range strings.Split(rendered, "\n") {
		if lipgloss.Width(line) > 80 {
			t.Fatalf("expected rendered line width <= 80, got %d for line %q", lipgloss.Width(line), line)
		}
	}
}

func TestRenderToolItemCompactModeHidesDetailedOutput(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	item := transcriptItem{
		Kind:   kindTool,
		Prefix: "tool[shell]",
		Meta:   "completed",
		Text: renderToolGroup(
			"completed",
			`{"command":"sed -n '1,50p' docs/invariants.md","working_dir":"."}`,
			"# Invariants\n\nThese rules are non-negotiable.",
			false,
		),
	}

	rendered := renderToolItem(theme, item, 80, false)
	if strings.Contains(rendered, "output:") {
		t.Fatalf("expected compact rendering to hide output label, got %q", rendered)
	}
	if !strings.Contains(rendered, "Reading [docs/invariants.md]") {
		t.Fatalf("expected compact rendering summary, got %q", rendered)
	}
}

func TestRenderToolItemDebugModeShowsIndentedDetails(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	item := transcriptItem{
		Kind:   kindTool,
		Prefix: "tool[shell]",
		Meta:   "completed",
		Text: renderToolGroup(
			"completed",
			`{"command":"pwd","working_dir":"."}`,
			"/Users/rex/projects/goose-go",
			false,
		),
	}

	rendered := renderToolItem(theme, item, 80, true)
	if !strings.Contains(rendered, "tool[shell] • completed") {
		t.Fatalf("expected debug headline, got %q", rendered)
	}
	if !strings.Contains(rendered, "  status: completed") {
		t.Fatalf("expected indented details, got %q", rendered)
	}
}

func TestRenderAssistantItemDoesNotExceedViewportWidth(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	item := transcriptItem{
		Kind:   kindAssistant,
		Prefix: "assistant",
		Text:   "Here is a long response that should wrap inside the transcript width instead of overflowing past the terminal width and forcing the output to render a single oversized line.",
	}

	rendered := renderItem(theme, item, 80, false)
	for _, line := range strings.Split(rendered, "\n") {
		if lipgloss.Width(line) > 80 {
			t.Fatalf("expected rendered line width <= 80, got %d for line %q", lipgloss.Width(line), line)
		}
	}
}

func TestRenderAssistantItemConsumesInlineMarkdownMarkers(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	item := transcriptItem{
		Kind: kindAssistant,
		Text: "Use **goose-go** and `make check`.",
	}

	rendered := renderItem(theme, item, 80, false)
	if strings.Contains(rendered, "**goose-go**") {
		t.Fatalf("expected bold markdown markers to be consumed, got %q", rendered)
	}
	if strings.Contains(rendered, "`make check`") {
		t.Fatalf("expected code markdown markers to be consumed, got %q", rendered)
	}
	if !strings.Contains(rendered, "goose-go") || !strings.Contains(rendered, "make check") {
		t.Fatalf("expected rendered text to retain markdown content, got %q", rendered)
	}
}

func TestRenderSystemItemConsumesInlineMarkdownMarkers(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	item := transcriptItem{
		Kind: kindSystem,
		Text: "See **docs** in [architecture](https://example.com).",
	}

	rendered := renderItem(theme, item, 80, false)
	if strings.Contains(rendered, "**docs**") {
		t.Fatalf("expected bold markdown markers to be consumed, got %q", rendered)
	}
	if strings.Contains(rendered, "https://example.com") {
		t.Fatalf("expected link URL to stay out of transcript text, got %q", rendered)
	}
	if !strings.Contains(rendered, "architecture") {
		t.Fatalf("expected link label to remain, got %q", rendered)
	}
}

func TestRenderAssistantItemSupportsFencedCodeAndLists(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	item := transcriptItem{
		Kind: kindAssistant,
		Text: "Steps:\n\n- first item\n- second item\n\n```go\nfmt.Println(\"hello\")\n```",
	}

	rendered := renderItem(theme, item, 48, false)
	if !strings.Contains(rendered, "- first item") {
		t.Fatalf("expected rendered list item, got %q", rendered)
	}
	if !strings.Contains(rendered, "fmt.Println(\"hello\")") {
		t.Fatalf("expected rendered fenced code block, got %q", rendered)
	}
	for _, line := range strings.Split(rendered, "\n") {
		if lipgloss.Width(line) > 48 {
			t.Fatalf("expected rendered line width <= 48, got %d for line %q", lipgloss.Width(line), line)
		}
	}
}

func TestRenderUserAndAssistantItemsDoNotShowRolePrefixes(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}

	userRendered := renderItem(theme, transcriptItem{Kind: kindUser, Text: "hello"}, 80, false)
	if strings.Contains(userRendered, "user>") {
		t.Fatalf("expected user rendering without prefix, got %q", userRendered)
	}

	assistantRendered := renderItem(theme, transcriptItem{Kind: kindAssistant, Text: "world"}, 80, false)
	if strings.Contains(assistantRendered, "assistant>") {
		t.Fatalf("expected assistant rendering without prefix, got %q", assistantRendered)
	}
}

func TestRenderUserItemFillsWidth(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	rendered := renderItem(theme, transcriptItem{
		Kind: kindUser,
		Text: "this should render as a full-width gray bubble with padding and wrapping across the transcript width",
	}, 64, false)

	for _, line := range strings.Split(rendered, "\n") {
		if lipgloss.Width(line) != 64 {
			t.Fatalf("expected user bubble line width 64, got %d for %q", lipgloss.Width(line), line)
		}
	}
}
