package markdown

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	tuitheme "goose-go/internal/tui/theme"
)

// Render parses markdown and returns ANSI-styled terminal text for assistant/system transcript content.
func Render(theme tuitheme.Palette, input string, width int, base lipgloss.Style) string {
	if strings.TrimSpace(input) == "" {
		return base.Render(input)
	}

	source := []byte(input)
	doc := goldmark.New().Parser().Parse(text.NewReader(source))
	return renderDocument(theme, doc, source, width, base)
}

// RenderInline parses inline markdown and returns ANSI-styled terminal text.
// Layout ownership stays with the caller via the width parameter.
func RenderInline(theme tuitheme.Palette, input string, width int, base lipgloss.Style) string {
	if strings.TrimSpace(input) == "" {
		return base.Render(input)
	}

	source := []byte(input)
	doc := goldmark.New().Parser().Parse(text.NewReader(source))
	spans := renderChildren(theme, doc, source, base)
	if len(spans) == 0 {
		return renderWrappedPlain(base, input, width)
	}
	return renderWrappedSpans(spans, width)
}

func renderDocument(theme tuitheme.Palette, doc ast.Node, source []byte, width int, base lipgloss.Style) string {
	blocks := make([]string, 0, doc.ChildCount())
	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		rendered := renderBlock(theme, child, source, width, base)
		if strings.TrimSpace(rendered) == "" {
			continue
		}
		blocks = append(blocks, rendered)
	}
	if len(blocks) == 0 {
		return RenderInline(theme, string(source), width, base)
	}
	return strings.Join(blocks, "\n\n")
}

func renderWrappedPlain(style lipgloss.Style, input string, width int) string {
	if width <= 0 {
		return style.Render(input)
	}
	return renderWrappedSpans([]span{{Text: input, Style: style}}, width)
}
