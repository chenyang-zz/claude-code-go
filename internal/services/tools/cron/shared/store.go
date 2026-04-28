package shared

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MaxJobs is the maximum number of active cron tasks allowed at once.
const MaxJobs = 50

// CronTask represents a single scheduled cron task entry.
type CronTask struct {
	// ID is the unique identifier returned to the caller.
	ID string
	// Cron is the raw 5-field cron expression.
	Cron string
	// Prompt is the text prompt to enqueue at each fire time.
	Prompt string
	// Recurring reports whether the task fires repeatedly on schedule.
	Recurring bool
	// Durable reports whether the task persists across sessions.
	Durable bool
	// CreatedAt records when the task was created.
	CreatedAt time.Time
}

// Store provides a concurrency-safe in-memory registry for cron tasks.
type Store struct {
	mu    sync.Mutex
	tasks map[string]CronTask
}

// NewStore constructs an empty cron task store.
func NewStore() *Store {
	return &Store{
		tasks: make(map[string]CronTask),
	}
}

// Create adds a new cron task with a generated UUID. Returns an error if the
// maximum task count has been reached.
func (s *Store) Create(cron, prompt string, recurring, durable bool) (CronTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.tasks) >= MaxJobs {
		return CronTask{}, fmt.Errorf("too many scheduled jobs (max %d), cancel one first", MaxJobs)
	}

	task := CronTask{
		ID:        uuid.NewString(),
		Cron:      cron,
		Prompt:    prompt,
		Recurring: recurring,
		Durable:   durable,
		CreatedAt: time.Now(),
	}
	s.tasks[task.ID] = task
	return task, nil
}

// Delete removes a cron task by ID. Returns an error if the task does not exist.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return fmt.Errorf("no scheduled job with id %q", id)
	}
	delete(s.tasks, id)
	return nil
}

// List returns a snapshot of all currently active cron tasks.
func (s *Store) List() []CronTask {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]CronTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		result = append(result, task)
	}
	return result
}

// Exists reports whether a task with the given ID is registered.
func (s *Store) Exists(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.tasks[id]
	return ok
}

// Count returns the number of currently registered tasks.
func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.tasks)
}
