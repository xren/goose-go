package session

import (
	"context"
	"errors"

	"github.com/google/uuid"

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

type Store interface {
	CreateSession(ctx context.Context, params CreateParams) (Session, error)
	GetSession(ctx context.Context, id string) (Session, error)
	AddMessage(ctx context.Context, sessionID string, message conversation.Message) (Session, error)
	ReplaceConversation(ctx context.Context, sessionID string, conv conversation.Conversation) (Session, error)
	ReplayConversation(ctx context.Context, sessionID string) (conversation.Conversation, error)
}

var ErrSessionNotFound = errors.New("session not found")

func newSessionID() string {
	return "sess_" + uuid.NewString()
}
