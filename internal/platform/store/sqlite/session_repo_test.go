package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
)

// TestSessionRepositorySaveAndLoadRoundTrip verifies one persisted session can be restored with normalized message content intact.
func TestSessionRepositorySaveAndLoadRoundTrip(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionRepository(db)

	now := time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC)
	input := coresession.Session{
		ID: "session-1",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
			{Role: message.RoleAssistant, Content: []message.ContentPart{
				message.ToolUsePart("tool-1", "grep", map[string]any{"pattern": "todo", "limit": float64(5)}),
			}},
			{Role: message.RoleUser, Content: []message.ContentPart{
				message.ToolResultPart("tool-1", "found", false),
			}},
		},
		UpdatedAt: now,
	}

	if err := repo.Save(context.Background(), input); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := repo.Load(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.ID != input.ID {
		t.Fatalf("Load() id = %q, want %q", got.ID, input.ID)
	}
	if !got.UpdatedAt.Equal(now) {
		t.Fatalf("Load() updated_at = %v, want %v", got.UpdatedAt, now)
	}
	if len(got.Messages) != len(input.Messages) {
		t.Fatalf("Load() message count = %d, want %d", len(got.Messages), len(input.Messages))
	}

	toolInput := got.Messages[1].Content[0].ToolInput
	if toolInput["pattern"] != "todo" {
		t.Fatalf("Load() tool input pattern = %#v, want todo", toolInput["pattern"])
	}
	if toolInput["limit"] != float64(5) {
		t.Fatalf("Load() tool input limit = %#v, want 5", toolInput["limit"])
	}
}

// TestSessionRepositorySaveOverwritesExistingSnapshot verifies later saves replace the previous normalized history for the same session id.
func TestSessionRepositorySaveOverwritesExistingSnapshot(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionRepository(db)

	first := coresession.Session{
		ID:        "session-2",
		Messages:  []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("first")}}},
		UpdatedAt: time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC),
	}
	second := coresession.Session{
		ID: "session-2",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("second")}},
			{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("reply")}},
		},
		UpdatedAt: time.Date(2026, 4, 4, 13, 5, 0, 0, time.UTC),
	}

	if err := repo.Save(context.Background(), first); err != nil {
		t.Fatalf("Save(first) error = %v", err)
	}
	if err := repo.Save(context.Background(), second); err != nil {
		t.Fatalf("Save(second) error = %v", err)
	}

	got, err := repo.Load(context.Background(), "session-2")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(got.Messages) != 2 {
		t.Fatalf("Load() message count = %d, want 2", len(got.Messages))
	}
	if got.Messages[0].Content[0].Text != "second" {
		t.Fatalf("Load() first message = %q, want second", got.Messages[0].Content[0].Text)
	}
}

// TestSessionRepositoryLoadMissingSession verifies unknown session ids map to the shared not-found error.
func TestSessionRepositoryLoadMissingSession(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionRepository(db)

	_, err := repo.Load(context.Background(), "missing")
	if !errors.Is(err, coresession.ErrSessionNotFound) {
		t.Fatalf("Load() error = %v, want ErrSessionNotFound", err)
	}
}

// openTestDB opens one isolated SQLite database for repository tests.
func openTestDB(t *testing.T) *DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "sessions.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	return db
}
