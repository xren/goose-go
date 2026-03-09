package app

import (
	"context"
	"errors"
	"fmt"

	"goose-go/internal/compaction"
	"goose-go/internal/conversation"
	"goose-go/internal/session"
)

type ContextSnapshot struct {
	SessionID        string
	WorkingDir       string
	Provider         string
	Model            string
	SystemPrompt     string
	LatestCompaction *session.Compaction
	ActiveMessages   []conversation.Message
	EstimatedTokens  int
	HasSession       bool
}

func (r *Runtime) ContextSnapshot(ctx context.Context, sessionID string) (ContextSnapshot, error) {
	snapshot := ContextSnapshot{
		SessionID:    sessionID,
		WorkingDir:   r.workingDir,
		Provider:     r.provider,
		Model:        r.model,
		SystemPrompt: r.systemPrompt,
	}
	if sessionID == "" {
		return snapshot, nil
	}

	record, err := r.store.GetSession(ctx, sessionID)
	if err != nil {
		return ContextSnapshot{}, fmt.Errorf("load session context: %w", err)
	}
	snapshot.HasSession = true
	snapshot.SessionID = record.ID
	if record.WorkingDir != "" {
		snapshot.WorkingDir = record.WorkingDir
	}
	if record.Provider != "" {
		snapshot.Provider = record.Provider
	}
	if record.Model != "" {
		snapshot.Model = record.Model
	}

	latest, err := r.store.GetLatestCompaction(ctx, record.ID)
	if err != nil && !errors.Is(err, session.ErrCompactionNotFound) {
		return ContextSnapshot{}, fmt.Errorf("load latest compaction: %w", err)
	}
	if err == nil {
		copy := latest
		snapshot.LatestCompaction = &copy
	}

	activeMessages, err := compaction.BuildActiveMessages(record.Conversation.Messages, latest)
	if err != nil {
		return ContextSnapshot{}, fmt.Errorf("build active messages: %w", err)
	}
	snapshot.ActiveMessages = activeMessages
	snapshot.EstimatedTokens = compaction.EstimateConversationTokens(activeMessages)

	return snapshot, nil
}
