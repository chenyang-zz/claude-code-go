package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Manager owns the minimum session lifecycle used by the runtime.
type Manager struct {
	// Repository persists and restores normalized session state.
	Repository coresession.Repository
	// Now supplies timestamps for deterministic tests.
	Now func() time.Time
}

// NewManager builds a manager from an optional repository.
func NewManager(repository coresession.Repository) *Manager {
	return &Manager{
		Repository: repository,
		Now:        time.Now,
	}
}

// Start returns an existing session snapshot when available or initializes a new empty session.
func (m *Manager) Start(ctx context.Context, id string) (coresession.Snapshot, error) {
	return m.StartInProject(ctx, id, "")
}

// StartInProject returns an existing session snapshot when available or initializes a new project-scoped session.
func (m *Manager) StartInProject(ctx context.Context, id string, projectPath string) (coresession.Snapshot, error) {
	if id == "" {
		return coresession.Snapshot{}, fmt.Errorf("missing session id")
	}

	snapshot, err := m.Resume(ctx, id)
	if err == nil {
		return snapshot, nil
	}
	if !errors.Is(err, coresession.ErrSessionNotFound) {
		return coresession.Snapshot{}, err
	}

	session := coresession.Session{
		ID:          id,
		ProjectPath: projectPath,
		UpdatedAt:   m.now(),
	}
	logger.DebugCF("session_manager", "initialized new session", map[string]any{
		"session_id":   id,
		"project_path": projectPath,
	})

	return coresession.Snapshot{
		Session: session,
		Resumed: false,
	}, nil
}

// Resume loads one existing session snapshot from the repository.
func (m *Manager) Resume(ctx context.Context, id string) (coresession.Snapshot, error) {
	if id == "" {
		return coresession.Snapshot{}, fmt.Errorf("missing session id")
	}
	if m.Repository == nil {
		return coresession.Snapshot{}, coresession.ErrSessionNotFound
	}

	session, err := m.Repository.Load(ctx, id)
	if err != nil {
		return coresession.Snapshot{}, err
	}

	logger.DebugCF("session_manager", "restored persisted session", map[string]any{
		"session_id":     id,
		"message_count":  len(session.Messages),
		"updated_at_set": !session.UpdatedAt.IsZero(),
	})

	return coresession.Snapshot{
		Session: session.Clone(),
		Resumed: true,
	}, nil
}

// ResumeLatest loads the most recently updated session for one project scope.
func (m *Manager) ResumeLatest(ctx context.Context, projectPath string) (coresession.Snapshot, error) {
	if projectPath == "" {
		return coresession.Snapshot{}, fmt.Errorf("missing project path")
	}
	if m.Repository == nil {
		return coresession.Snapshot{}, coresession.ErrSessionNotFound
	}

	session, err := m.Repository.LoadLatest(ctx, coresession.Lookup{ProjectPath: projectPath})
	if err != nil {
		return coresession.Snapshot{}, err
	}

	logger.DebugCF("session_manager", "restored latest persisted session", map[string]any{
		"session_id":    session.ID,
		"project_path":  session.ProjectPath,
		"message_count": len(session.Messages),
	})
	return coresession.Snapshot{
		Session: session.Clone(),
		Resumed: true,
	}, nil
}

// ReplaceMessages overwrites the stored session history with the supplied normalized messages.
func (m *Manager) ReplaceMessages(ctx context.Context, id string, messages []message.Message) (coresession.Snapshot, error) {
	return m.ReplaceMessagesInProject(ctx, id, "", messages)
}

// ReplaceMessagesInProject overwrites the stored session history while preserving or seeding project scope.
func (m *Manager) ReplaceMessagesInProject(ctx context.Context, id string, projectPath string, messages []message.Message) (coresession.Snapshot, error) {
	if id == "" {
		return coresession.Snapshot{}, fmt.Errorf("missing session id")
	}

	snapshot, err := m.StartInProject(ctx, id, projectPath)
	if err != nil {
		return coresession.Snapshot{}, err
	}

	cloned := make([]message.Message, len(messages))
	copy(cloned, messages)

	snapshot.Session.Messages = cloned
	if snapshot.Session.ProjectPath == "" {
		snapshot.Session.ProjectPath = projectPath
	}
	snapshot.Session.UpdatedAt = m.now()

	if err := m.save(ctx, snapshot.Session); err != nil {
		return coresession.Snapshot{}, err
	}

	logger.DebugCF("session_manager", "saved session snapshot", map[string]any{
		"session_id":    id,
		"project_path":  snapshot.Session.ProjectPath,
		"message_count": len(messages),
	})

	return snapshot.Clone(), nil
}

// Fork clones one restored session into a new target session identifier and persists it immediately.
func (m *Manager) Fork(ctx context.Context, source coresession.Session, targetID string) (coresession.Snapshot, error) {
	if targetID == "" {
		return coresession.Snapshot{}, fmt.Errorf("missing target session id")
	}

	forked := source.Clone()
	forked.ID = targetID
	forked.UpdatedAt = m.now()

	if err := m.save(ctx, forked); err != nil {
		return coresession.Snapshot{}, err
	}

	logger.DebugCF("session_manager", "forked session snapshot", map[string]any{
		"source_session_id": source.ID,
		"target_session_id": targetID,
		"project_path":      forked.ProjectPath,
		"message_count":     len(forked.Messages),
	})
	return coresession.Snapshot{
		Session: forked.Clone(),
		Resumed: true,
	}, nil
}

func (m *Manager) save(ctx context.Context, session coresession.Session) error {
	if m.Repository == nil {
		return nil
	}
	return m.Repository.Save(ctx, session.Clone())
}

func (m *Manager) now() time.Time {
	if m != nil && m.Now != nil {
		return m.Now()
	}
	return time.Now()
}
