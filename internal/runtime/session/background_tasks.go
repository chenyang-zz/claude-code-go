package session

import (
	"sync"

	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// BackgroundTaskStore exposes one in-memory runtime task snapshot source for `/tasks`.
type BackgroundTaskStore struct {
	// mu guards task snapshot replacement and reads.
	mu sync.RWMutex
	// tasks stores the latest detached task snapshot slice.
	tasks []coresession.BackgroundTaskSnapshot
}

// NewBackgroundTaskStore builds an empty task snapshot store.
func NewBackgroundTaskStore() *BackgroundTaskStore {
	return &BackgroundTaskStore{}
}

// Replace overwrites the currently visible runtime task snapshots.
func (s *BackgroundTaskStore) Replace(tasks []coresession.BackgroundTaskSnapshot) {
	if s == nil {
		return
	}

	s.mu.Lock()
	s.tasks = coresession.CloneBackgroundTaskSnapshots(tasks)
	count := len(s.tasks)
	s.mu.Unlock()

	logger.DebugCF("background_task_store", "replaced runtime background task snapshots", map[string]any{
		"count": count,
	})
}

// List returns a detached copy of the currently visible runtime task snapshots.
func (s *BackgroundTaskStore) List() []coresession.BackgroundTaskSnapshot {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return coresession.CloneBackgroundTaskSnapshots(s.tasks)
}
