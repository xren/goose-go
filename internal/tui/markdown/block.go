package markdown

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/yuin/goldmark/ast"

	tuitheme "goose-go/internal/tui/theme"
)

func renderBlock(theme tuitheme.Palette, node ast.Node, source []byte, width int, base lipgloss.Style) string {
	switch n := node.(type) {
	case *ast.Paragraph:
		return renderWrappedSpans(renderChildren(theme, n, source, base), width)
	case *ast.TextBlock:
		return renderWrappedSpans(renderChildren(theme, n, source, base), width)
	case *ast.Heading:
		style := base.Foreground(theme.MarkdownBold).Bold(true)
		return renderWrappedSpans(renderChildren(theme, n, source, style), width)
	case *ast.FencedCodeBlock:
		return renderCodeBlock(theme, blockLines(n, source), width, base)
	case *ast.CodeBlock:
		return renderCodeBlock(theme, blockLines(n, source), width, base)
	case *ast.List:
		return renderList(theme, n, source, width, base)
	case *ast.Blockquote:
		innerWidth := width
		if innerWidth > 0 {
			innerWidth = max(8, width-2)
		}
		parts := renderChildBlocks(theme, n, source, innerWidth, base.Foreground(theme.SystemText))
		return prefixLines(strings.Join(parts, "\n\n"), "> ", "  ")
	default:
		parts := renderChildBlocks(theme, node, source, width, base)
		return strings.Join(parts, "\n\n")
	}
}

func renderChildBlocks(theme tuitheme.Palette, node ast.Node, source []byte, width int, base lipgloss.Style) []string {
	var blocks []string
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		rendered := renderBlock(theme, child, source, width, base)
		if strings.TrimSpace(rendered) == "" {
			continue
		}
		blocks = append(blocks, rendered)
	}
	return blocks
}

func renderList(theme tuitheme.Palette, list *ast.List, source []byte, width int, base lipgloss.Style) string {
	items := make([]string, 0, list.ChildCount())
	index := list.Start
	for child := list.FirstChild(); child != nil; child = child.NextSibling() {
		item, ok := child.(*ast.ListItem)
		if !ok {
			continue
		}
		marker := "- "
		if list.IsOrdered() {
			if index == 0 {
				index = 1
			}
			marker = fmt.Sprintf("%d. ", index)
			index++
		}
		bodyWidth := width
		if bodyWidth > 0 {
			bodyWidth = max(8, width-lipgloss.Width(marker))
		}
		body := strings.Join(renderChildBlocks(theme, item, source, bodyWidth, base), "\n\n")
		items = append(items, prefixLines(body, marker, strings.Repeat(" ", lipgloss.Width(marker))))
	}
	return strings.Join(items, "\n")
}

func renderCodeBlock(theme tuitheme.Palette, code string, width int, base lipgloss.Style) string {
	style := base.Foreground(theme.MarkdownCodeFG).Background(theme.MarkdownCodeBG)
	lines := strings.Split(strings.TrimRight(code, "\n"), "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	rendered := make([]string, 0, len(lines))
	contentWidth := width
	if contentWidth > 0 {
		contentWidth = max(8, width-2)
	}
	for _, line := range lines {
		rendered = append(rendered, renderCodeLine(style, line, contentWidth))
	}
	return strings.Join(rendered, "\n")
}

func renderCodeLine(style lipgloss.Style, line string, width int) string {
	if width <= 0 {
		return style.Render(" " + line)
	}
	segments := splitToken(line, max(1, width))
	if len(segments) == 0 {
		segments = []string{""}
	}
	out := make([]string, 0, len(segments))
	for _, segment := range segments {
		padded := " " + segment
		remaining := max(0, width+1-lipgloss.Width(padded))
		padded += strings.Repeat(" ", remaining)
		out = append(out, style.Render(padded))
	}
	return strings.Join(out, "\n")
}

func blockLines(node ast.Node, source []byte) string {
	var b strings.Builder
	lines := node.Lines()
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		b.Write(segment.Value(source))
	}
	return b.String()
}

func prefixLines(input string, first string, rest string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		if i == 0 {
			lines[i] = first + line
		} else {
			lines[i] = rest + line
		}
	}
	return strings.Join(lines, "\n")
}
