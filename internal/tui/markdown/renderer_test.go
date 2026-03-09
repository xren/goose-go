package markdown

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	tuitheme "goose-go/internal/tui/theme"
)

func TestRenderInlineStylesMarkdown(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	rendered := RenderInline(theme, "Use **goose-go** and `make check`", 80, lipgloss.NewStyle().Foreground(theme.AssistantText))
	if strings.Contains(rendered, "**goose-go**") {
		t.Fatalf("expected bold markdown markers to be consumed, got %q", rendered)
	}
	if strings.Contains(rendered, "`make check`") {
		t.Fatalf("expected code markdown markers to be consumed, got %q", rendered)
	}
	if !strings.Contains(rendered, "goose-go") || !strings.Contains(rendered, "make check") {
		t.Fatalf("expected rendered content to preserve text, got %q", rendered)
	}
}

func TestRenderInlineHonorsWidth(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	rendered := RenderInline(theme, "Here is **goose-go** with `averyveryverylonginlinecodesegment` and [docs](https://example.com)", 24, lipgloss.NewStyle().Foreground(theme.AssistantText))
	for _, line := range strings.Split(rendered, "\n") {
		if lipgloss.Width(line) > 24 {
			t.Fatalf("expected rendered line width <= 24, got %d for %q", lipgloss.Width(line), line)
		}
	}
}

func TestRenderInlineStylesLinksWithoutURLNoise(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	rendered := RenderInline(theme, "See [architecture](https://example.com)", 80, lipgloss.NewStyle().Foreground(theme.SystemText))
	if strings.Contains(rendered, "https://example.com") {
		t.Fatalf("expected inline link to render label only, got %q", rendered)
	}
	if !strings.Contains(rendered, "architecture") {
		t.Fatalf("expected inline link label to survive, got %q", rendered)
	}
}

func TestRenderSupportsFencedCodeBlock(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	input := "Before\n\n```go\nfmt.Println(\"hello\")\n```\n\nAfter"
	rendered := Render(theme, input, 40, lipgloss.NewStyle().Foreground(theme.AssistantText))
	if !strings.Contains(rendered, "fmt.Println(\"hello\")") {
		t.Fatalf("expected fenced code content to render, got %q", rendered)
	}
	for _, line := range strings.Split(rendered, "\n") {
		if lipgloss.Width(line) > 40 {
			t.Fatalf("expected rendered line width <= 40, got %d for %q", lipgloss.Width(line), line)
		}
	}
}

func TestRenderSupportsList(t *testing.T) {
	theme, err := tuitheme.Resolve("dark")
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	input := "- first item with `code`\n- second item"
	rendered := Render(theme, input, 32, lipgloss.NewStyle().Foreground(theme.AssistantText))
	if !strings.Contains(rendered, "- first item") {
		t.Fatalf("expected first list item marker, got %q", rendered)
	}
	if !strings.Contains(rendered, "- second item") {
		t.Fatalf("expected second list item marker, got %q", rendered)
	}
	for _, line := range strings.Split(rendered, "\n") {
		if lipgloss.Width(line) > 32 {
			t.Fatalf("expected rendered line width <= 32, got %d for %q", lipgloss.Width(line), line)
		}
	}
}
