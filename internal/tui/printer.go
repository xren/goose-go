package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"goose-go/internal/conversation"
)

type transcriptPrinter interface {
	Cmd(blocks ...string) tea.Cmd
}

type bubbleTranscriptPrinter struct{}

func (bubbleTranscriptPrinter) Cmd(blocks ...string) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(blocks)*2)
	for i, block := range blocks {
		block = strings.TrimRight(block, "\n")
		if strings.TrimSpace(block) == "" {
			continue
		}
		cmds = append(cmds, tea.Println(block))
		if i < len(blocks)-1 {
			cmds = append(cmds, tea.Println(""))
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Sequence(cmds...)
}

func (m model) transcriptWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 80
}

func (m model) renderedBlocks(items []transcriptItem) []string {
	if len(items) == 0 {
		return nil
	}
	blocks := make([]string, 0, len(items))
	width := m.transcriptWidth()
	for _, item := range items {
		blocks = append(blocks, renderItem(m.theme, item, width, m.debug))
	}
	return blocks
}

func (m model) printItemsCmd(items []transcriptItem) tea.Cmd {
	if len(items) == 0 {
		return nil
	}
	return m.printer.Cmd(m.renderedBlocks(items)...)
}

func (m model) printMessageCmd(message conversation.Message) tea.Cmd {
	items := make([]transcriptItem, 0, len(message.Content))
	appendMessageItems(&items, message)
	return m.printItemsCmd(items)
}

func (m model) printSystemCmd(text string) tea.Cmd {
	return m.printItemsCmd([]transcriptItem{{Kind: kindSystem, Prefix: "system", Text: text}})
}

func (m model) printErrorCmd(text string) tea.Cmd {
	return m.printItemsCmd([]transcriptItem{{Kind: kindError, Prefix: "error", Text: text}})
}

func (m model) printConversationCmd(conv conversation.Conversation) tea.Cmd {
	return m.printItemsCmd(buildTranscriptFromConversation(conv))
}
