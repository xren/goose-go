package session

import (
	"context"
	"errors"

	"goose-go/internal/conversation"
)

type Type string

const (
	TypeUser     Type = "user"
	TypeTerminal Type = "terminal"
)

type Session struct {
	ID           string                    `json:"id"`
	Name         string                    `json:"name"`
	WorkingDir   string                    `json:"working_dir"`
	Type         Type                      `json:"type"`
	CreatedAt    int64                     `json:"created_at"`
	UpdatedAt    int64                     `json:"updated_at"`
	MessageCount int                       `json:"message_count"`
	Conversation conversation.Conversation `json:"conversation"`
}

type CreateParams struct {
	Name       string
	WorkingDir string
	Type       Type
}

type Summary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	WorkingDir   string `json:"working_dir"`
	Type         Type   `json:"type"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	MessageCount int    `json:"message_count"`
}

type CompactionTrigger string

const (
	CompactionTriggerManual    CompactionTrigger = "manual"
	CompactionTriggerThreshold CompactionTrigger = "threshold"
	CompactionTriggerOverflow  CompactionTrigger = "overflow"
)

type Compaction struct {
	ID                 string            `json:"id"`
	SessionID          string            `json:"session_id"`
	Summary            string            `json:"summary"`
	FirstKeptMessageID string            `json:"first_kept_message_id"`
	TokensBefore       int               `json:"tokens_before"`
	Trigger            CompactionTrigger `json:"trigger"`
	CreatedAt          int64             `json:"created_at"`
}

type CompactionParams struct {
	Summary            string
	FirstKeptMessageID string
	TokensBefore       int
	Trigger            CompactionTrigger
}

type Store interface {
	CreateSession(ctx context.Context, params CreateParams) (Session, error)
	GetSession(ctx context.Context, id string) (Session, error)
	ListSessions(ctx context.Context) ([]Summary, error)
	AddMessage(ctx context.Context, sessionID string, message conversation.Message) (Session, error)
	ReplaceConversation(ctx context.Context, sessionID string, conv conversation.Conversation) (Session, error)
	ReplayConversation(ctx context.Context, sessionID string) (conversation.Conversation, error)
	AppendCompaction(ctx context.Context, sessionID string, params CompactionParams) (Compaction, error)
	GetLatestCompaction(ctx context.Context, sessionID string) (Compaction, error)
}

var ErrSessionNotFound = errors.New("session not found")
var ErrCompactionNotFound = errors.New("compaction not found")
