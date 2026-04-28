package session

import (
	"fmt"
	"slices"
	"sync"

	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

type backgroundTaskEntry struct {
	// snapshot stores the user-visible task metadata exposed to commands and tools.
	snapshot coresession.BackgroundTaskSnapshot
	// stopper carries the best-effort stop capability for the task when available.
	stopper interface{ Stop() error }
}

// BackgroundTaskStore exposes one in-memory runtime task snapshot source for `/tasks`.
type BackgroundTaskStore struct {
	// mu guards task snapshot replacement and reads.
	mu sync.RWMutex
	// tasks stores the latest task entries keyed by task ID.
	tasks map[string]backgroundTaskEntry
	// order preserves deterministic list ordering across reads.
	order []string
}

// NewBackgroundTaskStore builds an empty task snapshot store.
func NewBackgroundTaskStore() *BackgroundTaskStore {
	return &BackgroundTaskStore{
		tasks: make(map[string]backgroundTaskEntry),
	}
}

// Replace overwrites the currently visible runtime task snapshots.
func (s *BackgroundTaskStore) Replace(tasks []coresession.BackgroundTaskSnapshot) {
	if s == nil {
		return
	}

	s.mu.Lock()
	s.tasks = make(map[string]backgroundTaskEntry, len(tasks))
	s.order = s.order[:0]
	for _, task := range tasks {
		if task.ID == "" {
			continue
		}
		s.tasks[task.ID] = backgroundTaskEntry{snapshot: task}
		s.order = append(s.order, task.ID)
	}
	count := len(s.tasks)
	s.mu.Unlock()

	logger.DebugCF("background_task_store", "replaced runtime background task snapshots", map[string]any{
		"count": count,
	})
}

// Register inserts or replaces one live background task snapshot together with its optional stop capability.
func (s *BackgroundTaskStore) Register(task coresession.BackgroundTaskSnapshot, stopper interface{ Stop() error }) {
	if s == nil || task.ID == "" {
		return
	}

	s.mu.Lock()
	if s.tasks == nil {
		s.tasks = make(map[string]backgroundTaskEntry)
	}
	if _, exists := s.tasks[task.ID]; !exists {
		s.order = append(s.order, task.ID)
	}
	s.tasks[task.ID] = backgroundTaskEntry{
		snapshot: task,
		stopper:  stopper,
	}
	count := len(s.tasks)
	s.mu.Unlock()

	logger.DebugCF("background_task_store", "registered runtime background task", map[string]any{
		"task_id": task.ID,
		"type":    task.Type,
		"status":  task.Status,
		"count":   count,
	})
}

// Update replaces the stored snapshot for one existing task while preserving its stop capability.
func (s *BackgroundTaskStore) Update(task coresession.BackgroundTaskSnapshot) bool {
	if s == nil || task.ID == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.tasks[task.ID]
	if !exists {
		return false
	}
	entry.snapshot = task
	s.tasks[task.ID] = entry

	logger.DebugCF("background_task_store", "updated runtime background task", map[string]any{
		"task_id": task.ID,
		"type":    task.Type,
		"status":  task.Status,
	})
	return true
}

// Get returns one detached task snapshot when the task currently exists.
func (s *BackgroundTaskStore) Get(id string) (coresession.BackgroundTaskSnapshot, bool) {
	if s == nil || id == "" {
		return coresession.BackgroundTaskSnapshot{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.tasks[id]
	if !ok {
		return coresession.BackgroundTaskSnapshot{}, false
	}
	return entry.snapshot, true
}

// Remove deletes one task snapshot from the shared store.
func (s *BackgroundTaskStore) Remove(id string) {
	if s == nil || id == "" {
		return
	}

	s.mu.Lock()
	if _, exists := s.tasks[id]; !exists {
		s.mu.Unlock()
		return
	}
	delete(s.tasks, id)
	s.order = slices.DeleteFunc(s.order, func(existing string) bool {
		return existing == id
	})
	count := len(s.tasks)
	s.mu.Unlock()

	logger.DebugCF("background_task_store", "removed runtime background task", map[string]any{
		"task_id": id,
		"count":   count,
	})
}

// Stop requests termination of one running background task and keeps the stopped snapshot visible for follow-up queries.
func (s *BackgroundTaskStore) Stop(id string) (coresession.BackgroundTaskSnapshot, error) {
	if s == nil || id == "" {
		return coresession.BackgroundTaskSnapshot{}, fmt.Errorf("task_id is required")
	}

	s.mu.RLock()
	entry, exists := s.tasks[id]
	s.mu.RUnlock()
	if !exists {
		return coresession.BackgroundTaskSnapshot{}, fmt.Errorf("no background task found with ID: %s", id)
	}
	if entry.stopper == nil || !entry.snapshot.ControlsAvailable {
		return coresession.BackgroundTaskSnapshot{}, fmt.Errorf("background task %s cannot be stopped", id)
	}

	if err := entry.stopper.Stop(); err != nil {
		return coresession.BackgroundTaskSnapshot{}, fmt.Errorf("stop background task %s: %w", id, err)
	}

	stopped := entry.snapshot
	stopped.Status = coresession.BackgroundTaskStatusStopped
	stopped.ControlsAvailable = false
	s.Update(stopped)

	return stopped, nil
}

// List returns a detached copy of the currently visible runtime task snapshots.
func (s *BackgroundTaskStore) List() []coresession.BackgroundTaskSnapshot {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.order) == 0 {
		return nil
	}

	snapshots := make([]coresession.BackgroundTaskSnapshot, 0, len(s.order))
	for _, id := range s.order {
		entry, ok := s.tasks[id]
		if !ok {
			continue
		}
		snapshots = append(snapshots, entry.snapshot)
	}
	return coresession.CloneBackgroundTaskSnapshots(snapshots)
}
