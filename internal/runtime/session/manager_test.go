package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
)

type stubRepository struct {
	loadResult   coresession.Session
	loadErr      error
	latestResult coresession.Session
	latestErr    error
	saved        []coresession.Session
}

func (r *stubRepository) Save(ctx context.Context, session coresession.Session) error {
	_ = ctx
	r.saved = append(r.saved, session.Clone())
	return nil
}

func (r *stubRepository) Load(ctx context.Context, id string) (coresession.Session, error) {
	_ = ctx
	_ = id
	if r.loadErr != nil {
		return coresession.Session{}, r.loadErr
	}
	return r.loadResult.Clone(), nil
}

func (r *stubRepository) LoadLatest(ctx context.Context, lookup coresession.Lookup) (coresession.Session, error) {
	_ = ctx
	_ = lookup
	if r.latestErr != nil {
		return coresession.Session{}, r.latestErr
	}
	return r.latestResult.Clone(), nil
}

// TestManagerStartCreatesNewSession verifies the manager initializes an empty session when nothing is persisted yet.
func TestManagerStartCreatesNewSession(t *testing.T) {
	now := time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)
	manager := NewManager(&stubRepository{loadErr: coresession.ErrSessionNotFound})
	manager.Now = func() time.Time { return now }

	snapshot, err := manager.Start(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if snapshot.Resumed {
		t.Fatalf("Start() resumed = true, want false")
	}
	if snapshot.Session.ID != "session-1" {
		t.Fatalf("Start() session id = %q, want session-1", snapshot.Session.ID)
	}
	if !snapshot.Session.UpdatedAt.Equal(now) {
		t.Fatalf("Start() updated_at = %v, want %v", snapshot.Session.UpdatedAt, now)
	}
}

// TestManagerResumeLoadsExistingSession verifies persisted sessions are marked as resumed.
func TestManagerResumeLoadsExistingSession(t *testing.T) {
	repo := &stubRepository{
		loadResult: coresession.Session{
			ID:          "session-2",
			ProjectPath: "/repo",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hi")}},
			},
		},
	}
	manager := NewManager(repo)

	snapshot, err := manager.Resume(context.Background(), "session-2")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}

	if !snapshot.Resumed {
		t.Fatalf("Resume() resumed = false, want true")
	}
	if len(snapshot.Session.Messages) != 1 {
		t.Fatalf("Resume() message count = %d, want 1", len(snapshot.Session.Messages))
	}
	if snapshot.Session.ProjectPath != "/repo" {
		t.Fatalf("Resume() project path = %q, want /repo", snapshot.Session.ProjectPath)
	}
}

// TestManagerResumeLatestLoadsExistingSession verifies project-scoped latest-session queries are bridged through the manager.
func TestManagerResumeLatestLoadsExistingSession(t *testing.T) {
	repo := &stubRepository{
		latestResult: coresession.Session{
			ID:          "session-latest",
			ProjectPath: "/repo",
			Messages: []message.Message{
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("welcome back")}},
			},
		},
	}
	manager := NewManager(repo)

	snapshot, err := manager.ResumeLatest(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("ResumeLatest() error = %v", err)
	}

	if !snapshot.Resumed {
		t.Fatalf("ResumeLatest() resumed = false, want true")
	}
	if snapshot.Session.ID != "session-latest" {
		t.Fatalf("ResumeLatest() session id = %q, want session-latest", snapshot.Session.ID)
	}
}

// TestManagerReplaceMessagesSavesSnapshot verifies autosave-style overwrites persist the latest normalized history.
func TestManagerReplaceMessagesSavesSnapshot(t *testing.T) {
	now := time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC)
	repo := &stubRepository{loadErr: coresession.ErrSessionNotFound}
	manager := NewManager(repo)
	manager.Now = func() time.Time { return now }

	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("world")}},
	}

	snapshot, err := manager.ReplaceMessages(context.Background(), "session-3", messages)
	if err != nil {
		t.Fatalf("ReplaceMessages() error = %v", err)
	}

	if len(repo.saved) != 1 {
		t.Fatalf("saved count = %d, want 1", len(repo.saved))
	}
	if !repo.saved[0].UpdatedAt.Equal(now) {
		t.Fatalf("saved updated_at = %v, want %v", repo.saved[0].UpdatedAt, now)
	}
	if len(snapshot.Session.Messages) != 2 {
		t.Fatalf("snapshot messages = %d, want 2", len(snapshot.Session.Messages))
	}
}

// TestManagerResumePropagatesRepositoryErrors verifies unexpected repository failures are not swallowed.
func TestManagerResumePropagatesRepositoryErrors(t *testing.T) {
	repo := &stubRepository{loadErr: errors.New("boom")}
	manager := NewManager(repo)

	_, err := manager.Resume(context.Background(), "session-4")
	if err == nil || err.Error() != "boom" {
		t.Fatalf("Resume() error = %v, want boom", err)
	}
}

// TestManagerForkPersistsNewTargetSession verifies forking preserves history while switching to a new session id.
func TestManagerForkPersistsNewTargetSession(t *testing.T) {
	now := time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC)
	repo := &stubRepository{}
	manager := NewManager(repo)
	manager.Now = func() time.Time { return now }

	source := coresession.Session{
		ID:          "session-source",
		ProjectPath: "/repo",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("carry over")}},
		},
	}

	snapshot, err := manager.Fork(context.Background(), source, "session-forked")
	if err != nil {
		t.Fatalf("Fork() error = %v", err)
	}

	if len(repo.saved) != 1 {
		t.Fatalf("saved count = %d, want 1", len(repo.saved))
	}
	if repo.saved[0].ID != "session-forked" {
		t.Fatalf("saved id = %q, want session-forked", repo.saved[0].ID)
	}
	if repo.saved[0].ProjectPath != "/repo" {
		t.Fatalf("saved project path = %q, want /repo", repo.saved[0].ProjectPath)
	}
	if !repo.saved[0].UpdatedAt.Equal(now) {
		t.Fatalf("saved updated_at = %v, want %v", repo.saved[0].UpdatedAt, now)
	}
	if snapshot.Session.ID != "session-forked" {
		t.Fatalf("snapshot id = %q, want session-forked", snapshot.Session.ID)
	}
}

// TestAutoSavePersistHistoryVerifiesConversationHistoryBridge ensures runtime history can be saved through the autosave helper.
func TestAutoSavePersistHistoryVerifiesConversationHistoryBridge(t *testing.T) {
	now := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	repo := &stubRepository{loadErr: coresession.ErrSessionNotFound}
	manager := NewManager(repo)
	manager.Now = func() time.Time { return now }
	autosave := NewAutoSave(manager)

	history := conversation.History{
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("first")}},
		},
	}

	snapshot, err := autosave.PersistHistory(context.Background(), "session-5", history)
	if err != nil {
		t.Fatalf("PersistHistory() error = %v", err)
	}

	if len(repo.saved) != 1 {
		t.Fatalf("saved count = %d, want 1", len(repo.saved))
	}
	if snapshot.Session.ID != "session-5" {
		t.Fatalf("snapshot session id = %q, want session-5", snapshot.Session.ID)
	}
}
