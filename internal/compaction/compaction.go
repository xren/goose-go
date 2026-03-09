package compaction

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"goose-go/internal/conversation"
	"goose-go/internal/session"
)

const (
	DefaultReserveTokens    = 16384
	DefaultKeepRecentTokens = 20000
)

type Settings struct {
	Enabled          bool `json:"enabled"`
	ReserveTokens    int  `json:"reserve_tokens"`
	KeepRecentTokens int  `json:"keep_recent_tokens"`
}

type CutPoint struct {
	FirstKeptMessageID string `json:"first_kept_message_id"`
	FirstKeptIndex     int    `json:"first_kept_index"`
}

type Preparation struct {
	NeedsCompaction     bool                   `json:"needs_compaction"`
	TokensBefore        int                    `json:"tokens_before"`
	FirstKeptMessageID  string                 `json:"first_kept_message_id,omitempty"`
	MessagesToSummarize []conversation.Message `json:"messages_to_summarize,omitempty"`
	KeptMessages        []conversation.Message `json:"kept_messages,omitempty"`
}

func DefaultSettings() Settings {
	return Settings{
		Enabled:          true,
		ReserveTokens:    DefaultReserveTokens,
		KeepRecentTokens: DefaultKeepRecentTokens,
	}
}

func (s Settings) Validate() error {
	if s.ReserveTokens < 0 {
		return errors.New("reserve_tokens must be >= 0")
	}
	if s.KeepRecentTokens < 0 {
		return errors.New("keep_recent_tokens must be >= 0")
	}
	return nil
}

func EstimateMessageTokens(message conversation.Message) int {
	chars := 0
	for _, item := range message.Content {
		switch item.Type {
		case conversation.ContentTypeText:
			if item.Text != nil {
				chars += len(item.Text.Text)
			}
		case conversation.ContentTypeToolRequest:
			if item.ToolRequest != nil {
				chars += len(item.ToolRequest.Name)
				chars += len(item.ToolRequest.Arguments)
			}
		case conversation.ContentTypeToolResponse:
			if item.ToolResponse != nil {
				for _, result := range item.ToolResponse.Content {
					chars += len(result.Type)
					chars += len(result.Text)
				}
				chars += len(item.ToolResponse.Structured)
			}
		case conversation.ContentTypeSystemNotification:
			if item.SystemNotification != nil {
				chars += len(item.SystemNotification.Level)
				chars += len(item.SystemNotification.Message)
				chars += len(item.SystemNotification.Data)
			}
		}
	}
	if chars == 0 {
		return 0
	}
	return (chars + 3) / 4
}

func EstimateConversationTokens(messages []conversation.Message) int {
	total := 0
	for _, message := range messages {
		total += EstimateMessageTokens(message)
	}
	return total
}

func ShouldCompact(totalTokens, contextWindow int, settings Settings) bool {
	if !settings.Enabled || contextWindow <= 0 {
		return false
	}
	return totalTokens > contextWindow-settings.ReserveTokens
}

func FindCutPoint(messages []conversation.Message, keepRecentTokens int) (CutPoint, error) {
	if len(messages) == 0 {
		return CutPoint{}, errors.New("messages are required")
	}
	if keepRecentTokens <= 0 {
		return CutPoint{
			FirstKeptMessageID: messages[0].ID,
			FirstKeptIndex:     0,
		}, nil
	}

	accumulated := 0
	firstKeptIndex := 0
	foundBoundary := false

	for i := len(messages) - 1; i >= 0; i-- {
		accumulated += EstimateMessageTokens(messages[i])
		firstKeptIndex = i
		if accumulated >= keepRecentTokens {
			firstKeptIndex = nearestTurnBoundary(messages, i)
			foundBoundary = true
			break
		}
	}

	if !foundBoundary {
		firstKeptIndex = 0
	}

	return CutPoint{
		FirstKeptMessageID: messages[firstKeptIndex].ID,
		FirstKeptIndex:     firstKeptIndex,
	}, nil
}

func Prepare(messages []conversation.Message, contextWindow int, settings Settings) (Preparation, error) {
	if err := settings.Validate(); err != nil {
		return Preparation{}, err
	}
	if len(messages) == 0 {
		return Preparation{}, nil
	}

	totalTokens := EstimateConversationTokens(messages)
	if !ShouldCompact(totalTokens, contextWindow, settings) {
		return Preparation{
			NeedsCompaction: false,
			TokensBefore:    totalTokens,
			KeptMessages:    cloneMessages(messages),
		}, nil
	}

	cutPoint, err := FindCutPoint(messages, settings.KeepRecentTokens)
	if err != nil {
		return Preparation{}, err
	}

	return Preparation{
		NeedsCompaction:     true,
		TokensBefore:        totalTokens,
		FirstKeptMessageID:  cutPoint.FirstKeptMessageID,
		MessagesToSummarize: cloneMessages(messages[:cutPoint.FirstKeptIndex]),
		KeptMessages:        cloneMessages(messages[cutPoint.FirstKeptIndex:]),
	}, nil
}

func BuildActiveMessages(messages []conversation.Message, artifact session.Compaction) ([]conversation.Message, error) {
	if artifact.ID == "" {
		return cloneMessages(messages), nil
	}
	if artifact.FirstKeptMessageID == "" {
		return nil, errors.New("compaction first_kept_message_id is required")
	}

	firstKeptIndex := -1
	for i, message := range messages {
		if message.ID == artifact.FirstKeptMessageID {
			firstKeptIndex = i
			break
		}
	}
	if firstKeptIndex == -1 {
		return nil, fmt.Errorf("first kept message %q not found", artifact.FirstKeptMessageID)
	}

	active := []conversation.Message{SummaryMessage(artifact)}
	active = append(active, cloneMessages(messages[firstKeptIndex:])...)
	return active, nil
}

func SummaryMessage(artifact session.Compaction) conversation.Message {
	return conversation.NewMessage(
		conversation.RoleAssistant,
		conversation.Text(renderSummaryText(artifact)),
	)
}

func SerializeForSummarization(messages []conversation.Message) string {
	lines := make([]string, 0, len(messages))
	for _, message := range messages {
		role := strings.ToUpper(string(message.Role))
		var parts []string
		for _, item := range message.Content {
			switch item.Type {
			case conversation.ContentTypeText:
				if item.Text != nil && item.Text.Text != "" {
					parts = append(parts, item.Text.Text)
				}
			case conversation.ContentTypeToolRequest:
				if item.ToolRequest != nil {
					parts = append(parts, fmt.Sprintf("tool_request(%s): %s", item.ToolRequest.Name, compactJSON(item.ToolRequest.Arguments)))
				}
			case conversation.ContentTypeToolResponse:
				if item.ToolResponse != nil {
					textParts := make([]string, 0, len(item.ToolResponse.Content))
					for _, result := range item.ToolResponse.Content {
						if result.Text != "" {
							textParts = append(textParts, result.Text)
						}
					}
					if len(textParts) > 0 {
						parts = append(parts, "tool_response: "+strings.Join(textParts, "\n"))
					} else if len(item.ToolResponse.Structured) > 0 {
						parts = append(parts, "tool_response_structured: "+compactJSON(item.ToolResponse.Structured))
					}
				}
			case conversation.ContentTypeSystemNotification:
				if item.SystemNotification != nil {
					parts = append(parts, fmt.Sprintf("system_notification(%s): %s", item.SystemNotification.Level, item.SystemNotification.Message))
				}
			}
		}
		if len(parts) == 0 {
			lines = append(lines, fmt.Sprintf("[%s]: <empty>", role))
			continue
		}
		lines = append(lines, fmt.Sprintf("[%s]: %s", role, strings.Join(parts, "\n")))
	}
	return strings.Join(lines, "\n")
}

func nearestTurnBoundary(messages []conversation.Message, index int) int {
	for i := index; i >= 0; i-- {
		if messages[i].Role == conversation.RoleUser {
			return i
		}
	}
	return index
}

func cloneMessages(messages []conversation.Message) []conversation.Message {
	out := make([]conversation.Message, len(messages))
	copy(out, messages)
	return out
}

func renderSummaryText(artifact session.Compaction) string {
	return fmt.Sprintf(
		"Compacted session summary (%s, %d tokens before):\n\n%s",
		artifact.Trigger,
		artifact.TokensBefore,
		artifact.Summary,
	)
}

func compactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return string(raw)
	}
	buf, err := json.Marshal(decoded)
	if err != nil {
		return string(raw)
	}
	return string(buf)
}
