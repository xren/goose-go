package sqlite

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
	"goose-go/internal/session"
)

type Store struct {
	db *sql.DB
}

type sessionQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type sessionExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type migration struct {
	version int
	up      string
}

var migrations = []migration{
	{
		version: 1,
		up: `
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
		`,
	},
}

var _ session.Store = (*Store)(nil)

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateSession(ctx context.Context, params session.CreateParams) (session.Session, error) {
	now := time.Now().UTC().Unix()
	record := session.Session{
		ID:           newSessionID(),
		Name:         params.Name,
		WorkingDir:   params.WorkingDir,
		Type:         params.Type,
		CreatedAt:    now,
		UpdatedAt:    now,
		MessageCount: 0,
		Conversation: conversation.NewConversation(),
	}

	if record.Type == "" {
		record.Type = session.TypeUser
	}

	payload, err := json.Marshal(record.Conversation)
	if err != nil {
		return session.Session{}, fmt.Errorf("marshal conversation: %w", err)
	}

	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO sessions (id, name, working_dir, type, created_at, updated_at, message_count, conversation_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID,
		record.Name,
		record.WorkingDir,
		record.Type,
		record.CreatedAt,
		record.UpdatedAt,
		record.MessageCount,
		string(payload),
	)
	if err != nil {
		return session.Session{}, fmt.Errorf("insert session: %w", err)
	}

	return record, nil
}

func (s *Store) GetSession(ctx context.Context, id string) (session.Session, error) {
	return getSession(ctx, s.db, id)
}

func (s *Store) ListSessions(ctx context.Context) ([]session.Summary, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, name, working_dir, type, created_at, updated_at, message_count
		 FROM sessions
		 ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summaries []session.Summary
	for rows.Next() {
		var item session.Summary
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.WorkingDir,
			&item.Type,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.MessageCount,
		); err != nil {
			return nil, fmt.Errorf("scan session summary: %w", err)
		}
		summaries = append(summaries, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session summaries: %w", err)
	}
	return summaries, nil
}

func (s *Store) AddMessage(ctx context.Context, sessionID string, message conversation.Message) (session.Session, error) {
	if err := message.Validate(); err != nil {
		return session.Session{}, err
	}

	return s.withImmediateTx(ctx, func(conn *sql.Conn) (session.Session, error) {
		record, err := getSession(ctx, conn, sessionID)
		if err != nil {
			return session.Session{}, err
		}

		if err := record.Conversation.Append(message); err != nil {
			return session.Session{}, err
		}

		return persistConversation(ctx, conn, record)
	})
}

func (s *Store) ReplaceConversation(ctx context.Context, sessionID string, conv conversation.Conversation) (session.Session, error) {
	if err := conv.Validate(); err != nil {
		return session.Session{}, err
	}

	return s.withImmediateTx(ctx, func(conn *sql.Conn) (session.Session, error) {
		record, err := getSession(ctx, conn, sessionID)
		if err != nil {
			return session.Session{}, err
		}

		record.Conversation = conv
		return persistConversation(ctx, conn, record)
	})
}

func (s *Store) ReplayConversation(ctx context.Context, sessionID string) (conversation.Conversation, error) {
	record, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return conversation.Conversation{}, err
	}
	return record.Conversation.Clone(), nil
}

func (s *Store) withImmediateTx(ctx context.Context, fn func(conn *sql.Conn) (session.Session, error)) (session.Session, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return session.Session{}, fmt.Errorf("acquire sqlite conn: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return session.Session{}, fmt.Errorf("begin immediate tx: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
	}()

	record, err := fn(conn)
	if err != nil {
		return session.Session{}, err
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return session.Session{}, fmt.Errorf("commit tx: %w", err)
	}

	committed = true
	return record, nil
}

func getSession(ctx context.Context, queryer sessionQueryer, id string) (session.Session, error) {
	row := queryer.QueryRowContext(
		ctx,
		`SELECT id, name, working_dir, type, created_at, updated_at, message_count, conversation_json
		 FROM sessions WHERE id = ?`,
		id,
	)

	var record session.Session
	var convJSON string
	err := row.Scan(
		&record.ID,
		&record.Name,
		&record.WorkingDir,
		&record.Type,
		&record.CreatedAt,
		&record.UpdatedAt,
		&record.MessageCount,
		&convJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return session.Session{}, session.ErrSessionNotFound
		}
		return session.Session{}, fmt.Errorf("get session: %w", err)
	}

	if err := json.Unmarshal([]byte(convJSON), &record.Conversation); err != nil {
		return session.Session{}, fmt.Errorf("decode conversation: %w", err)
	}
	if err := record.Conversation.Validate(); err != nil {
		return session.Session{}, fmt.Errorf("validate conversation: %w", err)
	}

	return record, nil
}

func persistConversation(ctx context.Context, execer sessionExecer, record session.Session) (session.Session, error) {
	now := time.Now().UTC().Unix()
	record.UpdatedAt = now
	record.MessageCount = len(record.Conversation.Messages)

	payload, err := json.Marshal(record.Conversation)
	if err != nil {
		return session.Session{}, fmt.Errorf("marshal conversation: %w", err)
	}

	result, err := execer.ExecContext(
		ctx,
		`UPDATE sessions
		 SET updated_at = ?, message_count = ?, conversation_json = ?
		 WHERE id = ?`,
		record.UpdatedAt,
		record.MessageCount,
		string(payload),
		record.ID,
	)
	if err != nil {
		return session.Session{}, fmt.Errorf("update session: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return session.Session{}, fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return session.Session{}, session.ErrSessionNotFound
	}

	return record, nil
}

func (s *Store) migrate(ctx context.Context) error {
	currentVersion, err := s.userVersion(ctx)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if migration.version <= currentVersion {
			continue
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration tx: %w", err)
		}

		if _, err := tx.ExecContext(ctx, migration.up); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", migration.version, err)
		}

		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", migration.version)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("set user_version %d: %w", migration.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", migration.version, err)
		}

		currentVersion = migration.version
	}

	return nil
}

func (s *Store) userVersion(ctx context.Context) (int, error) {
	row := s.db.QueryRowContext(ctx, "PRAGMA user_version")

	var version int
	if err := row.Scan(&version); err != nil {
		return 0, fmt.Errorf("read user_version: %w", err)
	}

	return version, nil
}

func newSessionID() string {
	return "sess_" + newUUID()
}
