package agent

import (
	"goose-go/internal/conversation"
	"goose-go/internal/session"
	"goose-go/internal/tools"
)

type EventType string

const (
	EventTypeRunStarted                EventType = "run_started"
	EventTypeUserMessagePersisted      EventType = "user_message_persisted"
	EventTypeTurnStarted               EventType = "turn_started"
	EventTypeProviderTextDelta         EventType = "provider_text_delta"
	EventTypeAssistantMessageComplete  EventType = "assistant_message_complete"
	EventTypeAssistantMessagePersisted EventType = "assistant_message_persisted"
	EventTypeCompactionStarted         EventType = "compaction_started"
	EventTypeCompactionCompleted       EventType = "compaction_completed"
	EventTypeCompactionFailed          EventType = "compaction_failed"
	EventTypeToolCallDetected          EventType = "tool_call_detected"
	EventTypeApprovalRequired          EventType = "approval_required"
	EventTypeApprovalResolved          EventType = "approval_resolved"
	EventTypeToolExecutionStarted      EventType = "tool_execution_started"
	EventTypeToolExecutionFinished     EventType = "tool_execution_finished"
	EventTypeToolMessagePersisted      EventType = "tool_message_persisted"
	EventTypeRunCompleted              EventType = "run_completed"
	EventTypeRunInterrupted            EventType = "run_interrupted"
	EventTypeRunFailed                 EventType = "run_failed"
)

type Event struct {
	Type             EventType                 `json:"type"`
	SessionID        string                    `json:"session_id,omitempty"`
	Turn             int                       `json:"turn,omitempty"`
	Delta            string                    `json:"delta,omitempty"`
	Message          *conversation.Message     `json:"message,omitempty"`
	Compaction       *session.Compaction       `json:"compaction,omitempty"`
	CompactionReason session.CompactionTrigger `json:"compaction_reason,omitempty"`
	TokensBefore     int                       `json:"tokens_before,omitempty"`
	ToolCall         *tools.Call               `json:"tool_call,omitempty"`
	ToolResult       *tools.Result             `json:"tool_result,omitempty"`
	ApprovalRequest  *ApprovalRequest          `json:"approval_request,omitempty"`
	ApprovalDecision ApprovalDecision          `json:"approval_decision,omitempty"`
	Result           *Result                   `json:"result,omitempty"`
	Err              error                     `json:"-"`
}

func resultOrNil(result Result) *Result {
	if result.Session.ID == "" && result.Status == "" && result.Turns == 0 && result.PendingApprovalFor == nil {
		return nil
	}
	copy := result
	return &copy
}
