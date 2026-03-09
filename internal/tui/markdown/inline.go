package markdown

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/yuin/goldmark/ast"

	tuitheme "goose-go/internal/tui/theme"
)

type span struct {
	Text  string
	Style lipgloss.Style
}

func renderChildren(theme tuitheme.Palette, node ast.Node, source []byte, current lipgloss.Style) []span {
	var spans []span
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		spans = append(spans, renderNode(theme, child, source, current)...)
	}
	return coalesceSpans(spans)
}

func renderNode(theme tuitheme.Palette, node ast.Node, source []byte, current lipgloss.Style) []span {
	switch n := node.(type) {
	case *ast.Text:
		text := string(n.Segment.Value(source))
		if n.HardLineBreak() || n.SoftLineBreak() {
			text += "\n"
		}
		return []span{{Text: text, Style: current}}
	case *ast.Emphasis:
		style := current
		if n.Level == 2 {
			style = style.Foreground(theme.MarkdownBold).Bold(true)
		} else {
			style = style.Foreground(theme.MarkdownItalic).Italic(true)
		}
		return renderChildren(theme, n, source, style)
	case *ast.CodeSpan:
		text := strings.TrimRight(codeSpanText(n, source), "\n")
		style := current.Foreground(theme.MarkdownCodeFG)
		if theme.MarkdownCodeBG != lipgloss.Color("") {
			style = style.Background(theme.MarkdownCodeBG)
		}
		return []span{{Text: text, Style: style}}
	case *ast.Link:
		style := current.Foreground(theme.MarkdownLink).Underline(true)
		return renderChildren(theme, n, source, style)
	case *ast.AutoLink:
		style := current.Foreground(theme.MarkdownLink).Underline(true)
		return []span{{Text: string(n.Label(source)), Style: style}}
	default:
		return renderChildren(theme, node, source, current)
	}
}

func codeSpanText(node *ast.CodeSpan, source []byte) string {
	var b strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		text, ok := child.(*ast.Text)
		if !ok {
			continue
		}
		b.Write(text.Segment.Value(source))
	}
	return b.String()
}

func coalesceSpans(spans []span) []span {
	if len(spans) == 0 {
		return nil
	}
	out := make([]span, 0, len(spans))
	for _, s := range spans {
		if s.Text == "" {
			continue
		}
		if len(out) > 0 && out[len(out)-1].Style.String() == s.Style.String() {
			out[len(out)-1].Text += s.Text
			continue
		}
		out = append(out, s)
	}
	return out
}
