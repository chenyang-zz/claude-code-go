package commands

import (
	"context"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
)

type recordingSessionStore struct {
	saved []coresession.Session
}

func (s *recordingSessionStore) Save(ctx context.Context, session coresession.Session) error {
	_ = ctx
	s.saved = append(s.saved, session.Clone())
	return nil
}

func (s *recordingSessionStore) Load(ctx context.Context, id string) (coresession.Session, error) {
	_ = ctx
	_ = id
	return coresession.Session{}, coresession.ErrSessionNotFound
}

func (s *recordingSessionStore) LoadLatest(ctx context.Context, lookup coresession.Lookup) (coresession.Session, error) {
	_ = ctx
	_ = lookup
	return coresession.Session{}, coresession.ErrSessionNotFound
}

func (s *recordingSessionStore) ListRecent(ctx context.Context, lookup coresession.Lookup) ([]coresession.Summary, error) {
	_ = ctx
	_ = lookup
	return nil, nil
}

func (s *recordingSessionStore) Search(ctx context.Context, lookup coresession.Lookup) ([]coresession.Summary, error) {
	_ = ctx
	_ = lookup
	return nil, nil
}

func (s *recordingSessionStore) FindByCustomTitle(ctx context.Context, lookup coresession.Lookup) ([]coresession.Summary, error) {
	_ = ctx
	_ = lookup
	return nil, nil
}

func (s *recordingSessionStore) UpdateCustomTitle(ctx context.Context, id string, title string) error {
	_ = ctx
	_ = id
	_ = title
	return nil
}

// TestSeedSessionsCommandExecuteWritesDemoSessions verifies the command seeds a stable demo dataset into the configured session store.
func TestSeedSessionsCommandExecuteWritesDemoSessions(t *testing.T) {
	store := &recordingSessionStore{}
	result, err := SeedSessionsCommand{
		Repository:  store,
		ProjectPath: "/repo/current",
		Now: func() time.Time {
			return time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)
		},
	}.Execute(context.Background(), testCommandArgs())
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(store.saved) != 4 {
		t.Fatalf("saved count = %d, want 4", len(store.saved))
	}
	if store.saved[0].ID != "seed-current-latest" || store.saved[0].ProjectPath != "/repo/current" {
		t.Fatalf("first saved session = %#v, want current project latest seed", store.saved[0])
	}
	if store.saved[1].CustomTitle != "Deploy retrospective" {
		t.Fatalf("second saved custom title = %q, want Deploy retrospective", store.saved[1].CustomTitle)
	}
	if store.saved[3].ProjectPath != "/repo/current-other" {
		t.Fatalf("other project path = %q, want /repo/current-other", store.saved[3].ProjectPath)
	}
	want := "Seeded 4 demo conversations into session storage.\nCurrent project: /repo/current\nOther project: /repo/current-other\nInserted sessions:\n- seed-current-latest\n- seed-current-retro (Deploy retrospective)\n- seed-current-debug\n- seed-other-project (Other repo deploy)\nTry these next:\n  cc /resume\n  cc /resume deploy\n  cc /resume seed-current-latest hello"
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestSeedSessionsCommandExecuteRequiresStorage verifies the command reports the stable prerequisite when session storage is unavailable.
func TestSeedSessionsCommandExecuteRequiresStorage(t *testing.T) {
	result, err := SeedSessionsCommand{}.Execute(context.Background(), testCommandArgs())
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != seedSessionsNotConfiguredMessage {
		t.Fatalf("Execute() output = %q, want %q", result.Output, seedSessionsNotConfiguredMessage)
	}
}

func testCommandArgs() command.Args { return command.Args{} }
