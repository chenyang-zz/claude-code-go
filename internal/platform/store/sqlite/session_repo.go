package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// SessionRepository persists the minimum normalized session snapshots in SQLite.
type SessionRepository struct {
	// DB provides the opened SQLite handle and applied schema.
	DB *DB
}

// NewSessionRepository builds a repository from one opened SQLite database.
func NewSessionRepository(db *DB) *SessionRepository {
	return &SessionRepository{DB: db}
}

// Save upserts the latest normalized session snapshot.
func (r *SessionRepository) Save(ctx context.Context, session coresession.Session) error {
	if r == nil || r.DB == nil || r.DB.SQL == nil {
		return fmt.Errorf("sqlite session repository is not initialized")
	}

	messagesJSON, err := json.Marshal(session.Messages)
	if err != nil {
		return fmt.Errorf("marshal session messages: %w", err)
	}

	_, err = r.DB.SQL.ExecContext(
		ctx,
		`INSERT INTO sessions (id, updated_at, messages_json)
VALUES (?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	updated_at = excluded.updated_at,
	messages_json = excluded.messages_json`,
		session.ID,
		session.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(messagesJSON),
	)
	if err != nil {
		return fmt.Errorf("save session %s: %w", session.ID, err)
	}

	logger.DebugCF("sqlite_session_repo", "saved session snapshot", map[string]any{
		"session_id":    session.ID,
		"message_count": len(session.Messages),
		"path":          r.DB.Path,
	})
	return nil
}

// Load restores one previously saved session snapshot by identifier.
func (r *SessionRepository) Load(ctx context.Context, id string) (coresession.Session, error) {
	if r == nil || r.DB == nil || r.DB.SQL == nil {
		return coresession.Session{}, fmt.Errorf("sqlite session repository is not initialized")
	}

	row := r.DB.SQL.QueryRowContext(
		ctx,
		`SELECT updated_at, messages_json FROM sessions WHERE id = ?`,
		id,
	)

	var updatedAtText string
	var messagesJSON string
	if err := row.Scan(&updatedAtText, &messagesJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return coresession.Session{}, coresession.ErrSessionNotFound
		}
		return coresession.Session{}, fmt.Errorf("load session %s: %w", id, err)
	}

	updatedAt, err := time.Parse(time.RFC3339Nano, updatedAtText)
	if err != nil {
		return coresession.Session{}, fmt.Errorf("parse session %s updated_at: %w", id, err)
	}

	var messages []message.Message
	if err := json.Unmarshal([]byte(messagesJSON), &messages); err != nil {
		return coresession.Session{}, fmt.Errorf("decode session %s messages: %w", id, err)
	}

	logger.DebugCF("sqlite_session_repo", "loaded session snapshot", map[string]any{
		"session_id":    id,
		"message_count": len(messages),
		"path":          r.DB.Path,
	})
	return coresession.Session{
		ID:        id,
		Messages:  messages,
		UpdatedAt: updatedAt,
	}, nil
}
