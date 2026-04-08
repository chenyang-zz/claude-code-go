package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	summaryText := coresession.DerivePreview(session.Messages)

	_, err = r.DB.SQL.ExecContext(
		ctx,
		`INSERT INTO sessions (id, project_path, custom_title, summary_text, updated_at, messages_json)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	project_path = excluded.project_path,
	custom_title = excluded.custom_title,
	summary_text = excluded.summary_text,
	updated_at = excluded.updated_at,
	messages_json = excluded.messages_json`,
		session.ID,
		session.ProjectPath,
		session.CustomTitle,
		summaryText,
		session.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(messagesJSON),
	)
	if err != nil {
		return fmt.Errorf("save session %s: %w", session.ID, err)
	}

	logger.DebugCF("sqlite_session_repo", "saved session snapshot", map[string]any{
		"session_id":    session.ID,
		"message_count": len(session.Messages),
		"summary_set":   summaryText != "",
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
		`SELECT project_path, custom_title, updated_at, messages_json FROM sessions WHERE id = ?`,
		id,
	)

	var projectPath string
	var customTitle string
	var updatedAtText string
	var messagesJSON string
	if err := row.Scan(&projectPath, &customTitle, &updatedAtText, &messagesJSON); err != nil {
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
		ID:          id,
		ProjectPath: projectPath,
		CustomTitle: customTitle,
		Messages:    messages,
		UpdatedAt:   updatedAt,
	}, nil
}

// LoadLatest restores the most recently updated session within one project scope.
func (r *SessionRepository) LoadLatest(ctx context.Context, lookup coresession.Lookup) (coresession.Session, error) {
	if r == nil || r.DB == nil || r.DB.SQL == nil {
		return coresession.Session{}, fmt.Errorf("sqlite session repository is not initialized")
	}
	if lookup.ProjectPath == "" {
		return coresession.Session{}, fmt.Errorf("missing project path")
	}

	row := r.DB.SQL.QueryRowContext(
		ctx,
		`SELECT id, project_path, custom_title, updated_at, messages_json
FROM sessions
WHERE project_path = ?
ORDER BY updated_at DESC, id DESC
LIMIT 1`,
		lookup.ProjectPath,
	)

	var sessionID string
	var projectPath string
	var customTitle string
	var updatedAtText string
	var messagesJSON string
	if err := row.Scan(&sessionID, &projectPath, &customTitle, &updatedAtText, &messagesJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return coresession.Session{}, coresession.ErrSessionNotFound
		}
		return coresession.Session{}, fmt.Errorf("load latest session for %s: %w", lookup.ProjectPath, err)
	}

	updatedAt, err := time.Parse(time.RFC3339Nano, updatedAtText)
	if err != nil {
		return coresession.Session{}, fmt.Errorf("parse latest session %s updated_at: %w", sessionID, err)
	}

	var messages []message.Message
	if err := json.Unmarshal([]byte(messagesJSON), &messages); err != nil {
		return coresession.Session{}, fmt.Errorf("decode latest session %s messages: %w", sessionID, err)
	}

	logger.DebugCF("sqlite_session_repo", "loaded latest session snapshot", map[string]any{
		"session_id":    sessionID,
		"project_path":  projectPath,
		"message_count": len(messages),
		"path":          r.DB.Path,
	})
	return coresession.Session{
		ID:          sessionID,
		ProjectPath: projectPath,
		CustomTitle: customTitle,
		Messages:    messages,
		UpdatedAt:   updatedAt,
	}, nil
}

// ListRecent restores recent session summaries within one project scope.
func (r *SessionRepository) ListRecent(ctx context.Context, lookup coresession.Lookup) ([]coresession.Summary, error) {
	if r == nil || r.DB == nil || r.DB.SQL == nil {
		return nil, fmt.Errorf("sqlite session repository is not initialized")
	}
	if !lookup.AllProjects && lookup.ProjectPath == "" {
		return nil, fmt.Errorf("missing project path")
	}
	if lookup.Limit <= 0 {
		return nil, fmt.Errorf("missing limit")
	}

	query := `SELECT id, project_path, custom_title, summary_text, updated_at, messages_json
FROM sessions
WHERE project_path = ?
ORDER BY updated_at DESC, id DESC
LIMIT ?`
	args := []any{lookup.ProjectPath, lookup.Limit}
	if lookup.AllProjects {
		query = `SELECT id, project_path, custom_title, summary_text, updated_at, messages_json
FROM sessions
ORDER BY updated_at DESC, id DESC
LIMIT ?`
		args = []any{lookup.Limit}
	}

	rows, err := r.DB.SQL.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list recent sessions for %s: %w", lookupLabel(lookup), err)
	}
	defer rows.Close()

	summaries, err := scanSessionSummaries(rows)
	if err != nil {
		return nil, err
	}

	logger.DebugCF("sqlite_session_repo", "listed recent session summaries", map[string]any{
		"project_scope": lookupLabel(lookup),
		"limit":         lookup.Limit,
		"count":         len(summaries),
		"path":          r.DB.Path,
	})
	return summaries, nil
}

// Search restores project-scoped session summaries matching one free-text query.
func (r *SessionRepository) Search(ctx context.Context, lookup coresession.Lookup) ([]coresession.Summary, error) {
	if r == nil || r.DB == nil || r.DB.SQL == nil {
		return nil, fmt.Errorf("sqlite session repository is not initialized")
	}
	if !lookup.AllProjects && lookup.ProjectPath == "" {
		return nil, fmt.Errorf("missing project path")
	}
	if lookup.Limit <= 0 {
		return nil, fmt.Errorf("missing limit")
	}
	query := strings.TrimSpace(lookup.Query)
	if query == "" {
		return nil, fmt.Errorf("missing query")
	}

	pattern := "%" + strings.ToLower(query) + "%"
	statement := `SELECT id, project_path, custom_title, summary_text, updated_at, messages_json
FROM sessions
WHERE project_path = ?
	AND (
		LOWER(id) LIKE ?
		OR LOWER(custom_title) LIKE ?
		OR LOWER(COALESCE(NULLIF(summary_text, ''), messages_json, '')) LIKE ?
	)
ORDER BY updated_at DESC, id DESC
LIMIT ?`
	args := []any{lookup.ProjectPath, pattern, pattern, pattern, lookup.Limit}
	if lookup.AllProjects {
		statement = `SELECT id, project_path, custom_title, summary_text, updated_at, messages_json
FROM sessions
WHERE (
	LOWER(id) LIKE ?
	OR LOWER(custom_title) LIKE ?
	OR LOWER(COALESCE(NULLIF(summary_text, ''), messages_json, '')) LIKE ?
)
ORDER BY updated_at DESC, id DESC
LIMIT ?`
		args = []any{pattern, pattern, pattern, lookup.Limit}
	}

	rows, err := r.DB.SQL.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("search sessions for %s: %w", lookupLabel(lookup), err)
	}
	defer rows.Close()

	summaries, err := scanSessionSummaries(rows)
	if err != nil {
		return nil, err
	}

	logger.DebugCF("sqlite_session_repo", "searched session summaries", map[string]any{
		"project_scope": lookupLabel(lookup),
		"query":         query,
		"limit":         lookup.Limit,
		"count":         len(summaries),
		"path":          r.DB.Path,
	})
	return summaries, nil
}

// FindByCustomTitle restores session summaries whose custom title exactly matches one normalized query.
func (r *SessionRepository) FindByCustomTitle(ctx context.Context, lookup coresession.Lookup) ([]coresession.Summary, error) {
	if r == nil || r.DB == nil || r.DB.SQL == nil {
		return nil, fmt.Errorf("sqlite session repository is not initialized")
	}
	if strings.TrimSpace(lookup.Query) == "" {
		return nil, fmt.Errorf("missing query")
	}
	if lookup.Limit <= 0 {
		return nil, fmt.Errorf("missing limit")
	}

	query := strings.ToLower(strings.TrimSpace(lookup.Query))
	statement := `SELECT id, project_path, custom_title, summary_text, updated_at, messages_json
FROM sessions
WHERE LOWER(custom_title) = ?
ORDER BY updated_at DESC, id DESC
LIMIT ?`

	rows, err := r.DB.SQL.QueryContext(ctx, statement, query, lookup.Limit)
	if err != nil {
		return nil, fmt.Errorf("search sessions by custom title for %s: %w", lookupLabel(lookup), err)
	}
	defer rows.Close()

	summaries, err := scanSessionSummaries(rows)
	if err != nil {
		return nil, err
	}

	logger.DebugCF("sqlite_session_repo", "searched exact session custom titles", map[string]any{
		"query": query,
		"limit": lookup.Limit,
		"count": len(summaries),
		"path":  r.DB.Path,
	})
	return summaries, nil
}

// UpdateCustomTitle overwrites one session's persisted custom title.
func (r *SessionRepository) UpdateCustomTitle(ctx context.Context, id string, title string) error {
	if r == nil || r.DB == nil || r.DB.SQL == nil {
		return fmt.Errorf("sqlite session repository is not initialized")
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("missing session id")
	}

	result, err := r.DB.SQL.ExecContext(ctx, `UPDATE sessions SET custom_title = ? WHERE id = ?`, strings.TrimSpace(title), id)
	if err != nil {
		return fmt.Errorf("update custom title for session %s: %w", id, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated row count for session %s: %w", id, err)
	}
	if rowsAffected == 0 {
		return coresession.ErrSessionNotFound
	}
	return nil
}

// lookupLabel returns one stable scope label for logs and error messages.
func lookupLabel(lookup coresession.Lookup) string {
	if lookup.AllProjects {
		return "all-projects"
	}
	return lookup.ProjectPath
}

// resolveSummaryPreview keeps old rows readable by falling back to messages_json when summary_text is empty.
func resolveSummaryPreview(summaryText string, messagesJSON string) (string, error) {
	if summaryText != "" {
		return summaryText, nil
	}
	if messagesJSON == "" {
		return "", nil
	}

	var messages []message.Message
	if err := json.Unmarshal([]byte(messagesJSON), &messages); err != nil {
		return "", fmt.Errorf("decode session messages for preview: %w", err)
	}
	return coresession.DerivePreview(messages), nil
}

// scanSessionSummaries normalizes summary rows shared by recent-list and search queries.
func scanSessionSummaries(rows *sql.Rows) ([]coresession.Summary, error) {
	var summaries []coresession.Summary
	for rows.Next() {
		var summary coresession.Summary
		var customTitle string
		var summaryText string
		var updatedAtText string
		var messagesJSON string
		if err := rows.Scan(&summary.ID, &summary.ProjectPath, &customTitle, &summaryText, &updatedAtText, &messagesJSON); err != nil {
			return nil, fmt.Errorf("scan session summary: %w", err)
		}
		summary.CustomTitle = customTitle
		updatedAt, err := time.Parse(time.RFC3339Nano, updatedAtText)
		if err != nil {
			return nil, fmt.Errorf("parse session %s updated_at: %w", summary.ID, err)
		}
		summary.UpdatedAt = updatedAt
		preview, err := resolveSummaryPreview(summaryText, messagesJSON)
		if err != nil {
			return nil, fmt.Errorf("resolve session %s preview: %w", summary.ID, err)
		}
		summary.Preview = preview
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session summaries: %w", err)
	}
	return summaries, nil
}
