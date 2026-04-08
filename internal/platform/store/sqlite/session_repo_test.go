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
		ID:          "session-1",
		ProjectPath: "/repo-a",
		CustomTitle: "Deploy fix",
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
	if got.ProjectPath != input.ProjectPath {
		t.Fatalf("Load() project path = %q, want %q", got.ProjectPath, input.ProjectPath)
	}
	if got.CustomTitle != input.CustomTitle {
		t.Fatalf("Load() custom title = %q, want %q", got.CustomTitle, input.CustomTitle)
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
		ID:          "session-2",
		ProjectPath: "/repo-a",
		Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("first")}}},
		UpdatedAt:   time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC),
	}
	second := coresession.Session{
		ID:          "session-2",
		ProjectPath: "/repo-b",
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
	if got.ProjectPath != "/repo-b" {
		t.Fatalf("Load() project path = %q, want /repo-b", got.ProjectPath)
	}
	if got.Messages[0].Content[0].Text != "second" {
		t.Fatalf("Load() first message = %q, want second", got.Messages[0].Content[0].Text)
	}
}

// TestSessionRepositoryLoadLatestByProject verifies project-scoped latest lookups return the newest matching session.
func TestSessionRepositoryLoadLatestByProject(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionRepository(db)

	sessions := []coresession.Session{
		{
			ID:          "session-1",
			ProjectPath: "/repo-a",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("first")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC),
		},
		{
			ID:          "session-2",
			ProjectPath: "/repo-a",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("second")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 14, 0, 0, 0, time.UTC),
		},
		{
			ID:          "session-3",
			ProjectPath: "/repo-b",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("other")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 15, 0, 0, 0, time.UTC),
		},
	}
	for _, session := range sessions {
		if err := repo.Save(context.Background(), session); err != nil {
			t.Fatalf("Save(%s) error = %v", session.ID, err)
		}
	}

	got, err := repo.LoadLatest(context.Background(), coresession.Lookup{ProjectPath: "/repo-a"})
	if err != nil {
		t.Fatalf("LoadLatest() error = %v", err)
	}

	if got.ID != "session-2" {
		t.Fatalf("LoadLatest() id = %q, want session-2", got.ID)
	}
	if got.ProjectPath != "/repo-a" {
		t.Fatalf("LoadLatest() project path = %q, want /repo-a", got.ProjectPath)
	}
}

// TestSessionRepositoryListRecentByProject verifies recent-session summaries are returned in descending update order.
func TestSessionRepositoryListRecentByProject(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionRepository(db)

	sessions := []coresession.Session{
		{
			ID:          "session-1",
			ProjectPath: "/repo-a",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("first prompt")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC),
		},
		{
			ID:          "session-2",
			ProjectPath: "/repo-a",
			CustomTitle: "Pinned title",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("latest prompt")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 14, 0, 0, 0, time.UTC),
		},
		{
			ID:          "session-3",
			ProjectPath: "/repo-b",
			UpdatedAt:   time.Date(2026, 4, 4, 15, 0, 0, 0, time.UTC),
		},
	}
	for _, session := range sessions {
		if err := repo.Save(context.Background(), session); err != nil {
			t.Fatalf("Save(%s) error = %v", session.ID, err)
		}
	}

	got, err := repo.ListRecent(context.Background(), coresession.Lookup{ProjectPath: "/repo-a", Limit: 2})
	if err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListRecent() len = %d, want 2", len(got))
	}
	if got[0].ID != "session-2" || got[1].ID != "session-1" {
		t.Fatalf("ListRecent() ids = %#v, want session-2 then session-1", []string{got[0].ID, got[1].ID})
	}
	if got[0].Preview != "latest prompt" || got[1].Preview != "first prompt" {
		t.Fatalf("ListRecent() previews = %#v, want latest prompt then first prompt", []string{got[0].Preview, got[1].Preview})
	}
	if got[0].CustomTitle != "Pinned title" {
		t.Fatalf("ListRecent() custom title = %q, want Pinned title", got[0].CustomTitle)
	}
}

// TestSessionRepositoryListRecentFallsBackToMessagesJSON verifies old rows without summary_text still expose a derived preview.
func TestSessionRepositoryListRecentFallsBackToMessagesJSON(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionRepository(db)

	session := coresession.Session{
		ID:          "session-legacy",
		ProjectPath: "/repo-a",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("legacy prompt")}},
		},
		UpdatedAt: time.Date(2026, 4, 4, 16, 0, 0, 0, time.UTC),
	}
	if err := repo.Save(context.Background(), session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := db.SQL.ExecContext(context.Background(), `UPDATE sessions SET summary_text = '' WHERE id = ?`, session.ID); err != nil {
		t.Fatalf("clear summary_text error = %v", err)
	}

	got, err := repo.ListRecent(context.Background(), coresession.Lookup{ProjectPath: "/repo-a", Limit: 1})
	if err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListRecent() len = %d, want 1", len(got))
	}
	if got[0].Preview != "legacy prompt" {
		t.Fatalf("ListRecent() preview = %q, want legacy prompt", got[0].Preview)
	}
}

// TestSessionRepositoryListRecentAllProjects verifies all-project recent-session queries return sessions across project boundaries.
func TestSessionRepositoryListRecentAllProjects(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionRepository(db)
	sessions := seedResumeFixtureSessions(t, repo)

	got, err := repo.ListRecent(context.Background(), coresession.Lookup{AllProjects: true, Limit: 5})
	if err != nil {
		t.Fatalf("ListRecent(all-projects) error = %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("ListRecent(all-projects) len = %d, want 4", len(got))
	}
	wantIDs := []string{
		sessions[3].ID,
		sessions[2].ID,
		sessions[1].ID,
		sessions[0].ID,
	}
	gotIDs := []string{got[0].ID, got[1].ID, got[2].ID, got[3].ID}
	for index := range wantIDs {
		if gotIDs[index] != wantIDs[index] {
			t.Fatalf("ListRecent(all-projects) ids = %#v, want %#v", gotIDs, wantIDs)
		}
	}
}

// TestSessionRepositorySearchByProject verifies project-scoped session search matches summary preview and session id.
func TestSessionRepositorySearchByProject(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionRepository(db)

	sessions := []coresession.Session{
		{
			ID:          "session-deploy-2",
			ProjectPath: "/repo-a",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("deploy pipeline fix")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 15, 0, 0, 0, time.UTC),
		},
		{
			ID:          "session-1",
			ProjectPath: "/repo-a",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("deploy checklist")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 14, 0, 0, 0, time.UTC),
		},
		{
			ID:          "session-deploy-other",
			ProjectPath: "/repo-b",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("deploy outside scope")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 16, 0, 0, 0, time.UTC),
		},
	}
	for _, session := range sessions {
		if err := repo.Save(context.Background(), session); err != nil {
			t.Fatalf("Save(%s) error = %v", session.ID, err)
		}
	}

	got, err := repo.Search(context.Background(), coresession.Lookup{ProjectPath: "/repo-a", Query: "deploy", Limit: 5})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Search() len = %d, want 2", len(got))
	}
	if got[0].ID != "session-deploy-2" || got[1].ID != "session-1" {
		t.Fatalf("Search() ids = %#v, want session-deploy-2 then session-1", []string{got[0].ID, got[1].ID})
	}
}

// TestSessionRepositorySearchFallsBackToMessagesJSON verifies old rows without summary_text remain searchable.
func TestSessionRepositorySearchFallsBackToMessagesJSON(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionRepository(db)

	session := coresession.Session{
		ID:          "session-legacy-search",
		ProjectPath: "/repo-a",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("legacy deploy plan")}},
		},
		UpdatedAt: time.Date(2026, 4, 4, 17, 0, 0, 0, time.UTC),
	}
	if err := repo.Save(context.Background(), session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := db.SQL.ExecContext(context.Background(), `UPDATE sessions SET summary_text = '' WHERE id = ?`, session.ID); err != nil {
		t.Fatalf("clear summary_text error = %v", err)
	}

	got, err := repo.Search(context.Background(), coresession.Lookup{ProjectPath: "/repo-a", Query: "deploy", Limit: 5})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Search() len = %d, want 1", len(got))
	}
	if got[0].ID != session.ID {
		t.Fatalf("Search() id = %q, want %q", got[0].ID, session.ID)
	}
}

// TestSessionRepositorySearchAllProjects verifies all-project search queries can match sessions outside the active project.
func TestSessionRepositorySearchAllProjects(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionRepository(db)
	sessions := seedResumeFixtureSessions(t, repo)

	got, err := repo.Search(context.Background(), coresession.Lookup{AllProjects: true, Query: "deploy", Limit: 5})
	if err != nil {
		t.Fatalf("Search(all-projects) error = %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("Search(all-projects) len = %d, want 4", len(got))
	}
	wantIDs := []string{
		sessions[3].ID,
		sessions[2].ID,
		sessions[1].ID,
		sessions[0].ID,
	}
	gotIDs := []string{got[0].ID, got[1].ID, got[2].ID, got[3].ID}
	for index := range wantIDs {
		if gotIDs[index] != wantIDs[index] {
			t.Fatalf("Search(all-projects) ids = %#v, want %#v", gotIDs, wantIDs)
		}
	}
}

// TestSessionRepositorySearchMatchesCustomTitle verifies search queries can fuzzy-match custom titles in recency order.
func TestSessionRepositorySearchMatchesCustomTitle(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionRepository(db)

	sessions := []coresession.Session{
		{
			ID:          "session-title-old",
			ProjectPath: "/repo-a",
			CustomTitle: "Deploy follow-up",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("unrelated preview")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 14, 0, 0, 0, time.UTC),
		},
		{
			ID:          "session-title-new",
			ProjectPath: "/repo-a",
			CustomTitle: "Deploy retrospective",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("still unrelated")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 15, 0, 0, 0, time.UTC),
		},
	}
	for _, session := range sessions {
		if err := repo.Save(context.Background(), session); err != nil {
			t.Fatalf("Save(%s) error = %v", session.ID, err)
		}
	}

	got, err := repo.Search(context.Background(), coresession.Lookup{ProjectPath: "/repo-a", Query: "deploy", Limit: 5})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Search() len = %d, want 2", len(got))
	}
	if got[0].ID != "session-title-new" || got[1].ID != "session-title-old" {
		t.Fatalf("Search() ids = %#v, want session-title-new then session-title-old", []string{got[0].ID, got[1].ID})
	}
}

// TestSessionRepositoryFindByCustomTitle verifies exact title lookups return matching sessions in recency order.
func TestSessionRepositoryFindByCustomTitle(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionRepository(db)

	sessions := []coresession.Session{
		{
			ID:          "session-repo-a",
			ProjectPath: "/repo-a",
			CustomTitle: "Deploy fix",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("deploy repo a")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 14, 0, 0, 0, time.UTC),
		},
		{
			ID:          "session-repo-b",
			ProjectPath: "/repo-b",
			CustomTitle: "Deploy fix",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("deploy repo b")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 15, 0, 0, 0, time.UTC),
		},
	}
	for _, session := range sessions {
		if err := repo.Save(context.Background(), session); err != nil {
			t.Fatalf("Save(%s) error = %v", session.ID, err)
		}
	}

	got, err := repo.FindByCustomTitle(context.Background(), coresession.Lookup{AllProjects: true, Query: "Deploy fix", Limit: 5})
	if err != nil {
		t.Fatalf("FindByCustomTitle() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("FindByCustomTitle() len = %d, want 2", len(got))
	}
	if got[0].ID != "session-repo-b" || got[1].ID != "session-repo-a" {
		t.Fatalf("FindByCustomTitle() ids = %#v, want session-repo-b then session-repo-a", []string{got[0].ID, got[1].ID})
	}
}

// seedResumeFixtureSessions writes a reusable cross-project `/resume` fixture into one repository.
func seedResumeFixtureSessions(t *testing.T, repo *SessionRepository) []coresession.Session {
	t.Helper()

	sessions := []coresession.Session{
		{
			ID:          "session-repo-a-old",
			ProjectPath: "/repo-a",
			CustomTitle: "Deploy fix",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("deploy repo a old")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC),
		},
		{
			ID:          "session-repo-b-old",
			ProjectPath: "/repo-b",
			CustomTitle: "Deploy fix",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("deploy repo b old")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 14, 0, 0, 0, time.UTC),
		},
		{
			ID:          "session-repo-a-new",
			ProjectPath: "/repo-a",
			CustomTitle: "Deploy retrospective",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("deploy repo a new")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 15, 0, 0, 0, time.UTC),
		},
		{
			ID:          "session-repo-b-new",
			ProjectPath: "/repo-b",
			CustomTitle: "Deploy checklist",
			Messages:    []message.Message{{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("deploy repo b new")}}},
			UpdatedAt:   time.Date(2026, 4, 4, 16, 0, 0, 0, time.UTC),
		},
	}
	for _, session := range sessions {
		if err := repo.Save(context.Background(), session); err != nil {
			t.Fatalf("Save(%s) error = %v", session.ID, err)
		}
	}
	return sessions
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
