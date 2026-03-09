package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"goose-go/internal/agent"
	"goose-go/internal/conversation"
)

func (m *model) applyAgentEvent(event agent.Event) tea.Cmd {
	m.applyTrace(event)

	switch event.Type {
	case agent.EventTypeRunStarted:
		m.status = "running"
		return nil
	case agent.EventTypeUserMessagePersisted:
		if event.Message != nil {
			cmd := m.printMessageCmd(*event.Message)
			if m.contextPanel.Open {
				return tea.Batch(cmd, m.refreshContextCmd())
			}
			return cmd
		}
		if m.contextPanel.Open {
			return m.refreshContextCmd()
		}
		return nil
	case agent.EventTypeProviderTextDelta:
		m.liveAssistant += event.Delta
		return nil
	case agent.EventTypeAssistantMessageComplete:
		m.liveAssistant = ""
		if event.Message != nil {
			return m.printMessageCmd(*event.Message)
		}
		return nil
	case agent.EventTypeToolCallDetected:
		if event.ToolCall != nil {
			upsertToolGroup(&m.activeTools, *event.ToolCall, "requested")
			if idx := findToolGroup(m.activeTools, event.ToolCall.ID); idx >= 0 {
				return m.printItemsCmd([]transcriptItem{m.activeTools[idx]})
			}
		}
		return nil
	case agent.EventTypeToolExecutionStarted:
		if event.ToolCall != nil {
			markToolGroupRunning(&m.activeTools, *event.ToolCall)
		}
		return nil
	case agent.EventTypeToolMessagePersisted:
		if event.ToolResult == nil {
			if m.contextPanel.Open {
				return m.refreshContextCmd()
			}
			return nil
		}
		if event.ToolCall != nil && findToolGroup(m.activeTools, event.ToolCall.ID) < 0 {
			upsertToolGroup(&m.activeTools, *event.ToolCall, "requested")
		}
		response := conversation.ToolResponseContent{
			ID:      event.ToolResult.ToolCallID,
			IsError: event.ToolResult.IsError,
			Content: event.ToolResult.Content,
		}
		upsertToolResult(&m.activeTools, response)
		if idx := findToolGroup(m.activeTools, response.ID); idx >= 0 {
			item := m.activeTools[idx]
			m.activeTools = removeToolGroup(m.activeTools, response.ID)
			cmd := m.printItemsCmd([]transcriptItem{item})
			if m.contextPanel.Open {
				return tea.Batch(cmd, m.refreshContextCmd())
			}
			return cmd
		}
		if m.contextPanel.Open {
			return m.refreshContextCmd()
		}
		return nil
	case agent.EventTypeCompactionStarted:
		return m.printSystemCmd(fmt.Sprintf("compacting context (%s, %d tokens)", event.CompactionReason, event.TokensBefore))
	case agent.EventTypeCompactionCompleted:
		cmd := m.printSystemCmd(fmt.Sprintf("compaction complete (%s)", event.CompactionReason))
		if m.contextPanel.Open {
			return tea.Batch(cmd, m.refreshContextCmd())
		}
		return cmd
	case agent.EventTypeCompactionFailed:
		return m.printErrorCmd(fmt.Sprintf("compaction failed (%s)", event.CompactionReason))
	case agent.EventTypeApprovalRequired:
		m.status = "awaiting approval"
		m.running = false
		m.cancelRun = nil
		m.liveAssistant = ""
		m.approval = approvalViewState{Request: event.ApprovalRequest}
		return nil
	case agent.EventTypeApprovalResolved:
		m.approval.Busy = false
		m.approval.Err = ""
		m.approval.Request = nil
		return nil
	case agent.EventTypeRunCompleted:
		m.liveAssistant = ""
		if event.Result != nil && event.Result.Status == agent.StatusAwaitingApproval {
			m.finishRun("awaiting approval")
		} else {
			m.finishRun(runtimeResultStatus(event.Result))
		}
		return nil
	case agent.EventTypeRunInterrupted:
		m.liveAssistant = ""
		m.finishRun("interrupted")
		return m.printSystemCmd("interrupted")
	case agent.EventTypeRunFailed:
		m.liveAssistant = ""
		m.finishRun("failed")
		return m.printErrorCmd(errorText(event.Err))
	default:
		return nil
	}
}

func removeToolGroup(items []transcriptItem, callID string) []transcriptItem {
	index := findToolGroup(items, callID)
	if index < 0 {
		return items
	}
	out := make([]transcriptItem, 0, len(items)-1)
	out = append(out, items[:index]...)
	out = append(out, items[index+1:]...)
	return out
}

func waitWith(cmd tea.Cmd, async <-chan tea.Msg) tea.Cmd {
	if cmd == nil {
		return waitForAsync(async)
	}
	return tea.Batch(cmd, waitForAsync(async))
}
