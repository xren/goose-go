package markdown

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

type token struct {
	Text       string
	Style      lipgloss.Style
	Newline    bool
	Whitespace bool
}

func renderWrappedSpans(spans []span, width int) string {
	if width <= 0 {
		var b strings.Builder
		for _, s := range spans {
			b.WriteString(s.Style.Render(s.Text))
		}
		return b.String()
	}

	tokens := tokenize(spans)
	lines := []string{""}
	lineWidths := []int{0}

	appendToken := func(tok token) {
		idx := len(lines) - 1
		lines[idx] += tok.Style.Render(tok.Text)
		lineWidths[idx] += lipgloss.Width(tok.Text)
	}

	newLine := func() {
		lines = append(lines, "")
		lineWidths = append(lineWidths, 0)
	}

	for _, tok := range tokens {
		if tok.Newline {
			newLine()
			continue
		}
		if tok.Text == "" {
			continue
		}

		segments := []string{tok.Text}
		if !tok.Whitespace && lipgloss.Width(tok.Text) > width {
			segments = splitToken(tok.Text, width)
		}

		for _, segment := range segments {
			segWidth := lipgloss.Width(segment)
			idx := len(lines) - 1
			if tok.Whitespace && lineWidths[idx] == 0 {
				continue
			}
			if !tok.Whitespace && lineWidths[idx] > 0 && lineWidths[idx]+segWidth > width {
				newLine()
				idx = len(lines) - 1
			}
			if tok.Whitespace && lineWidths[idx]+segWidth > width {
				continue
			}
			appendToken(token{Text: segment, Style: tok.Style})
		}
	}

	return strings.Join(lines, "\n")
}

func tokenize(spans []span) []token {
	out := make([]token, 0, len(spans)*2)
	for _, s := range spans {
		for _, part := range splitWhitespace(s.Text) {
			switch {
			case part == "\n":
				out = append(out, token{Newline: true})
			case strings.TrimSpace(part) == "":
				out = append(out, token{Text: part, Style: s.Style, Whitespace: true})
			default:
				out = append(out, token{Text: part, Style: s.Style})
			}
		}
	}
	return out
}

func splitWhitespace(input string) []string {
	if input == "" {
		return nil
	}
	var out []string
	var buf strings.Builder
	flush := func() {
		if buf.Len() == 0 {
			return
		}
		out = append(out, buf.String())
		buf.Reset()
	}
	for _, r := range input {
		switch r {
		case '\n':
			flush()
			out = append(out, "\n")
		case ' ', '\t':
			if buf.Len() > 0 {
				last, _ := utf8.DecodeLastRuneInString(buf.String())
				if last == ' ' || last == '\t' {
					buf.WriteRune(r)
					continue
				}
				flush()
			}
			buf.WriteRune(r)
		default:
			if buf.Len() > 0 {
				last, _ := utf8.DecodeLastRuneInString(buf.String())
				if last == ' ' || last == '\t' {
					flush()
				}
			}
			buf.WriteRune(r)
		}
	}
	flush()
	return out
}

func splitToken(input string, width int) []string {
	if width <= 0 || lipgloss.Width(input) <= width {
		return []string{input}
	}
	var out []string
	var current strings.Builder
	currentWidth := 0
	for _, r := range input {
		rw := lipgloss.Width(string(r))
		if currentWidth > 0 && currentWidth+rw > width {
			out = append(out, current.String())
			current.Reset()
			currentWidth = 0
		}
		current.WriteRune(r)
		currentWidth += rw
	}
	if current.Len() > 0 {
		out = append(out, current.String())
	}
	return out
}
