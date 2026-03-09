package tui

import (
	"fmt"

	"goose-go/internal/agent"
	"goose-go/internal/conversation"
)

func (m *model) applyAgentEvent(event agent.Event) {
	m.applyTrace(event)

	switch event.Type {
	case agent.EventTypeRunStarted:
		m.status = "running"
	case agent.EventTypeUserMessagePersisted:
		if event.Message != nil {
			appendMessageItems(&m.items, *event.Message)
		}
	case agent.EventTypeProviderTextDelta:
		m.upsertLiveAssistant(event.Delta)
	case agent.EventTypeAssistantMessageComplete:
		m.clearLiveAssistant()
		if event.Message != nil {
			appendMessageItems(&m.items, *event.Message)
		}
	case agent.EventTypeToolCallDetected:
		if event.ToolCall != nil {
			upsertToolGroup(&m.items, *event.ToolCall, "requested")
		}
	case agent.EventTypeToolExecutionStarted:
		if event.ToolCall != nil {
			markToolGroupRunning(&m.items, *event.ToolCall)
		}
	case agent.EventTypeToolMessagePersisted:
		if event.ToolResult != nil {
			if event.ToolCall != nil && findToolGroup(m.items, event.ToolCall.ID) < 0 {
				upsertToolGroup(&m.items, *event.ToolCall, "requested")
			}
			response := conversation.ToolResponseContent{
				ID:      event.ToolResult.ToolCallID,
				IsError: event.ToolResult.IsError,
				Content: event.ToolResult.Content,
			}
			upsertToolResult(&m.items, response)
		}
	case agent.EventTypeCompactionStarted:
		m.items = append(m.items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: fmt.Sprintf("compacting context (%s, %d tokens)", event.CompactionReason, event.TokensBefore)})
	case agent.EventTypeCompactionCompleted:
		m.items = append(m.items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: fmt.Sprintf("compaction complete (%s)", event.CompactionReason)})
	case agent.EventTypeCompactionFailed:
		m.items = append(m.items, transcriptItem{Kind: kindError, Prefix: "system", Text: fmt.Sprintf("compaction failed (%s)", event.CompactionReason)})
	case agent.EventTypeApprovalRequired:
		m.status = "awaiting approval"
		m.running = false
		m.cancelRun = nil
		m.approval = approvalViewState{Request: event.ApprovalRequest}
	case agent.EventTypeApprovalResolved:
		m.approval.Busy = false
		m.approval.Err = ""
		m.approval.Request = nil
	case agent.EventTypeRunCompleted:
		if event.Result != nil && event.Result.Status == agent.StatusAwaitingApproval {
			m.finishRun("awaiting approval")
		} else {
			m.finishRun(runtimeResultStatus(event.Result))
		}
	case agent.EventTypeRunInterrupted:
		m.items = append(m.items, transcriptItem{Kind: kindSystem, Prefix: "system", Text: "interrupted"})
		m.finishRun("interrupted")
	case agent.EventTypeRunFailed:
		m.items = append(m.items, transcriptItem{Kind: kindError, Prefix: "error", Text: errorText(event.Err)})
		m.finishRun("failed")
	}
	m.layout()
	m.syncViewport(false)
}

func (m *model) upsertLiveAssistant(delta string) {
	if len(m.items) > 0 && m.items[len(m.items)-1].Kind == kindLiveBuffer {
		m.items[len(m.items)-1].Text += delta
		return
	}
	m.items = append(m.items, transcriptItem{Kind: kindLiveBuffer, Prefix: "assistant", Text: delta})
}

func (m *model) clearLiveAssistant() {
	if len(m.items) > 0 && m.items[len(m.items)-1].Kind == kindLiveBuffer {
		m.items = m.items[:len(m.items)-1]
	}
}
