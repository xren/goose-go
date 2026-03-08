package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"goose-go/internal/conversation"
)

type SQLiteStore struct {
	db *sql.DB
}

type sessionQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type sessionExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &SQLiteStore{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) CreateSession(ctx context.Context, params CreateParams) (Session, error) {
	now := time.Now().UTC().Unix()
	session := Session{
		ID:           newSessionID(),
		Name:         params.Name,
		WorkingDir:   params.WorkingDir,
		Type:         params.Type,
		CreatedAt:    now,
		UpdatedAt:    now,
		MessageCount: 0,
		Conversation: conversation.NewConversation(),
	}

	if session.Type == "" {
		session.Type = TypeUser
	}

	payload, err := json.Marshal(session.Conversation)
	if err != nil {
		return Session{}, fmt.Errorf("marshal conversation: %w", err)
	}

	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO sessions (id, name, working_dir, type, created_at, updated_at, message_count, conversation_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID,
		session.Name,
		session.WorkingDir,
		session.Type,
		session.CreatedAt,
		session.UpdatedAt,
		session.MessageCount,
		string(payload),
	)
	if err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}

	return session, nil
}

func (s *SQLiteStore) GetSession(ctx context.Context, id string) (Session, error) {
	return getSession(ctx, s.db, id)
}

func (s *SQLiteStore) AddMessage(ctx context.Context, sessionID string, message conversation.Message) (Session, error) {
	if err := message.Validate(); err != nil {
		return Session{}, err
	}

	return s.withImmediateTx(ctx, func(conn *sql.Conn) (Session, error) {
		session, err := getSession(ctx, conn, sessionID)
		if err != nil {
			return Session{}, err
		}

		if err := session.Conversation.Append(message); err != nil {
			return Session{}, err
		}

		return persistConversation(ctx, conn, session)
	})
}

func (s *SQLiteStore) ReplaceConversation(ctx context.Context, sessionID string, conv conversation.Conversation) (Session, error) {
	if err := conv.Validate(); err != nil {
		return Session{}, err
	}

	return s.withImmediateTx(ctx, func(conn *sql.Conn) (Session, error) {
		session, err := getSession(ctx, conn, sessionID)
		if err != nil {
			return Session{}, err
		}

		session.Conversation = conv
		return persistConversation(ctx, conn, session)
	})
}

func (s *SQLiteStore) ReplayConversation(ctx context.Context, sessionID string) (conversation.Conversation, error) {
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return conversation.Conversation{}, err
	}
	return session.Conversation.Clone(), nil
}

func (s *SQLiteStore) withImmediateTx(ctx context.Context, fn func(conn *sql.Conn) (Session, error)) (Session, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("acquire sqlite conn: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return Session{}, fmt.Errorf("begin immediate tx: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
	}()

	session, err := fn(conn)
	if err != nil {
		return Session{}, err
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return Session{}, fmt.Errorf("commit tx: %w", err)
	}

	committed = true
	return session, nil
}

func getSession(ctx context.Context, queryer sessionQueryer, id string) (Session, error) {
	row := queryer.QueryRowContext(
		ctx,
		`SELECT id, name, working_dir, type, created_at, updated_at, message_count, conversation_json
		 FROM sessions WHERE id = ?`,
		id,
	)

	var session Session
	var convJSON string
	err := row.Scan(
		&session.ID,
		&session.Name,
		&session.WorkingDir,
		&session.Type,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.MessageCount,
		&convJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, fmt.Errorf("get session: %w", err)
	}

	if err := json.Unmarshal([]byte(convJSON), &session.Conversation); err != nil {
		return Session{}, fmt.Errorf("decode conversation: %w", err)
	}
	if err := session.Conversation.Validate(); err != nil {
		return Session{}, fmt.Errorf("validate conversation: %w", err)
	}

	return session, nil
}

func persistConversation(ctx context.Context, execer sessionExecer, session Session) (Session, error) {
	now := time.Now().UTC().Unix()
	session.UpdatedAt = now
	session.MessageCount = len(session.Conversation.Messages)

	payload, err := json.Marshal(session.Conversation)
	if err != nil {
		return Session{}, fmt.Errorf("marshal conversation: %w", err)
	}

	result, err := execer.ExecContext(
		ctx,
		`UPDATE sessions
		 SET updated_at = ?, message_count = ?, conversation_json = ?
		 WHERE id = ?`,
		session.UpdatedAt,
		session.MessageCount,
		string(payload),
		session.ID,
	)
	if err != nil {
		return Session{}, fmt.Errorf("update session: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return Session{}, fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return Session{}, ErrSessionNotFound
	}

	return session, nil
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			working_dir TEXT NOT NULL,
			type TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			message_count INTEGER NOT NULL DEFAULT 0,
			conversation_json TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	return nil
}
