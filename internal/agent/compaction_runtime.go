package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"goose-go/internal/compaction"
	"goose-go/internal/conversation"
	"goose-go/internal/session"
)

func (a *Agent) prepareActiveMessages(ctx context.Context, record session.Session, turn int, emit func(Event)) ([]conversation.Message, error) {
	latest, err := a.latestCompaction(ctx, record.ID)
	if err != nil {
		return nil, err
	}

	activeMessages, err := compaction.BuildActiveMessages(record.Conversation.Messages, latest)
	if err != nil {
		return nil, fmt.Errorf("build active messages: %w", err)
	}

	totalTokens := compaction.EstimateConversationTokens(activeMessages)
	if !compaction.ShouldCompact(totalTokens, a.config.contextWindow(), a.config.Compaction) {
		return activeMessages, nil
	}

	return a.compactForReason(ctx, record, turn, session.CompactionTriggerThreshold, emit)
}

func (a *Agent) compactForReason(
	ctx context.Context,
	record session.Session,
	turn int,
	reason session.CompactionTrigger,
	emit func(Event),
) ([]conversation.Message, error) {
	latest, err := a.latestCompaction(ctx, record.ID)
	if err != nil {
		return nil, err
	}

	candidateMessages, previousSummary, err := compactionCandidates(record.Conversation.Messages, latest)
	if err != nil {
		return nil, err
	}

	tokensBefore := compaction.EstimateConversationTokens(candidateMessages)
	effectiveWindow := a.config.contextWindow()
	if latest.ID != "" {
		summaryTokens := compaction.EstimateMessageTokens(compaction.SummaryMessage(latest))
		tokensBefore += summaryTokens
		effectiveWindow -= summaryTokens
	}

	preparation, err := compaction.Prepare(candidateMessages, effectiveWindow, a.config.Compaction)
	if err != nil {
		return nil, fmt.Errorf("prepare compaction: %w", err)
	}
	preparation.TokensBefore = tokensBefore
	if !preparation.NeedsCompaction && reason == session.CompactionTriggerOverflow {
		preparation, err = forceCompactionPreparation(candidateMessages, tokensBefore, a.config.Compaction.KeepRecentTokens)
		if err != nil {
			return nil, fmt.Errorf("force overflow compaction: %w", err)
		}
	}
	if preparation.NeedsCompaction && len(preparation.MessagesToSummarize) == 0 {
		preparation, err = forceCompactionPreparation(candidateMessages, tokensBefore, a.config.Compaction.KeepRecentTokens)
		if err != nil {
			return nil, fmt.Errorf("force compaction reduction: %w", err)
		}
	}
	if !preparation.NeedsCompaction || len(preparation.MessagesToSummarize) == 0 {
		return compaction.BuildActiveMessages(record.Conversation.Messages, latest)
	}

	emit(Event{Type: EventTypeCompactionStarted, SessionID: record.ID, Turn: turn, CompactionReason: reason, TokensBefore: preparation.TokensBefore})

	summarizer, err := compaction.NewSummarizer(a.provider, a.config.Model)
	if err != nil {
		emit(Event{Type: EventTypeCompactionFailed, SessionID: record.ID, Turn: turn, CompactionReason: reason, TokensBefore: preparation.TokensBefore, Err: err})
		return nil, err
	}

	summaryResult, err := summarizer.Summarize(ctx, record.ID, compaction.SummaryRequest{
		Messages:        preparation.MessagesToSummarize,
		PreviousSummary: previousSummary,
	})
	if err != nil {
		emit(Event{Type: EventTypeCompactionFailed, SessionID: record.ID, Turn: turn, CompactionReason: reason, TokensBefore: preparation.TokensBefore, Err: err})
		return nil, err
	}

	artifact, err := a.store.AppendCompaction(ctx, record.ID, session.CompactionParams{
		Summary:            summaryResult.Summary,
		FirstKeptMessageID: preparation.FirstKeptMessageID,
		TokensBefore:       preparation.TokensBefore,
		Trigger:            reason,
	})
	if err != nil {
		emit(Event{Type: EventTypeCompactionFailed, SessionID: record.ID, Turn: turn, CompactionReason: reason, TokensBefore: preparation.TokensBefore, Err: err})
		return nil, fmt.Errorf("persist compaction: %w", err)
	}

	activeMessages, err := compaction.BuildActiveMessages(record.Conversation.Messages, artifact)
	if err != nil {
		emit(Event{Type: EventTypeCompactionFailed, SessionID: record.ID, Turn: turn, CompactionReason: reason, TokensBefore: preparation.TokensBefore, Err: err})
		return nil, fmt.Errorf("rebuild compacted context: %w", err)
	}

	emit(Event{Type: EventTypeCompactionCompleted, SessionID: record.ID, Turn: turn, CompactionReason: reason, TokensBefore: preparation.TokensBefore, Compaction: &artifact})
	return activeMessages, nil
}

func (a *Agent) latestCompaction(ctx context.Context, sessionID string) (session.Compaction, error) {
	artifact, err := a.store.GetLatestCompaction(ctx, sessionID)
	if err == nil {
		return artifact, nil
	}
	if errors.Is(err, session.ErrCompactionNotFound) {
		return session.Compaction{}, nil
	}
	return session.Compaction{}, fmt.Errorf("get latest compaction: %w", err)
}

func compactionCandidates(messages []conversation.Message, latest session.Compaction) ([]conversation.Message, string, error) {
	if latest.ID == "" {
		return messages, "", nil
	}

	firstKeptIndex := -1
	for i, message := range messages {
		if message.ID == latest.FirstKeptMessageID {
			firstKeptIndex = i
			break
		}
	}
	if firstKeptIndex == -1 {
		return nil, "", fmt.Errorf("latest compaction first kept message %q not found", latest.FirstKeptMessageID)
	}
	return messages[firstKeptIndex:], latest.Summary, nil
}

func forceCompactionPreparation(messages []conversation.Message, tokensBefore int, keepRecentTokens int) (compaction.Preparation, error) {
	if len(messages) <= 1 {
		return compaction.Preparation{}, errors.New("compaction cannot reduce context further")
	}

	cutPoint, err := compaction.FindCutPoint(messages, keepRecentTokens)
	if err != nil {
		return compaction.Preparation{}, err
	}
	if cutPoint.FirstKeptIndex <= 0 {
		cutPoint.FirstKeptIndex = forceKeptIndex(messages)
		cutPoint.FirstKeptMessageID = messages[cutPoint.FirstKeptIndex].ID
	}

	return compaction.Preparation{
		NeedsCompaction:     true,
		TokensBefore:        tokensBefore,
		FirstKeptMessageID:  cutPoint.FirstKeptMessageID,
		MessagesToSummarize: append([]conversation.Message(nil), messages[:cutPoint.FirstKeptIndex]...),
		KeptMessages:        append([]conversation.Message(nil), messages[cutPoint.FirstKeptIndex:]...),
	}, nil
}

func forceKeptIndex(messages []conversation.Message) int {
	for i := len(messages) - 1; i > 0; i-- {
		if messages[i].Role == conversation.RoleUser {
			return i
		}
	}
	return len(messages) - 1
}

func isContextOverflowError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context window") ||
		strings.Contains(msg, "context length") ||
		strings.Contains(msg, "context limit") ||
		strings.Contains(msg, "too large for model") ||
		strings.Contains(msg, "maximum context length")
}
