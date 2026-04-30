package shared

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	platformstore "github.com/sheepzhao/claude-code-go/internal/platform/store"
)

// MaxJobs is the maximum number of active cron tasks allowed at once.
const MaxJobs = 50

// CronTask mirrors the platform CronTask type and is used as the shared
// representation across all cron tools and the scheduler.
type CronTask = platformstore.CronTask

// Store provides a concurrency-safe registry for cron tasks backed by
// .claude/scheduled_tasks.json. It wraps the platform persistence layer with
// the tool-facing interface used by CronCreate/CronDelete/CronList.
type Store struct {
	mu          sync.Mutex
	projectRoot string
}

// NewStore constructs a Store that reads and writes tasks under
// <projectRoot>/.claude/scheduled_tasks.json.
func NewStore(projectRoot string) *Store {
	return &Store{projectRoot: projectRoot}
}

// Create adds a new cron task with a generated UUID and persists it to disk.
// Returns an error if the maximum task count has been reached.
func (s *Store) Create(cron, prompt string, recurring, durable bool) (CronTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := platformstore.ReadCronTasks(s.projectRoot)
	if err != nil {
		return CronTask{}, fmt.Errorf("read cron tasks: %w", err)
	}

	if len(tasks) >= MaxJobs {
		return CronTask{}, fmt.Errorf("too many scheduled jobs (max %d), cancel one first", MaxJobs)
	}

	task := CronTask{
		ID:        uuid.NewString(),
		Cron:      cron,
		Prompt:    prompt,
		CreatedAt: time.Now(),
		Recurring: recurring,
		Durable:   durable,
	}

	tasks = append(tasks, task)
	if err := platformstore.WriteCronTasks(s.projectRoot, tasks); err != nil {
		return CronTask{}, fmt.Errorf("write cron tasks: %w", err)
	}

	return task, nil
}

// Delete removes a cron task by ID and persists the change to disk. Returns an
// error if the task does not exist.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := platformstore.ReadCronTasks(s.projectRoot)
	if err != nil {
		return fmt.Errorf("read cron tasks: %w", err)
	}

	found := false
	filtered := make([]CronTask, 0, len(tasks))
	for _, t := range tasks {
		if t.ID == id {
			found = true
		} else {
			filtered = append(filtered, t)
		}
	}

	if !found {
		return fmt.Errorf("no scheduled job with id %q", id)
	}

	return platformstore.WriteCronTasks(s.projectRoot, filtered)
}

// List returns a snapshot of all currently active cron tasks read from disk.
func (s *Store) List() []CronTask {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := platformstore.ReadCronTasks(s.projectRoot)
	if err != nil {
		return nil
	}
	return tasks
}

// Exists reports whether a task with the given ID exists on disk.
func (s *Store) Exists(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := platformstore.ReadCronTasks(s.projectRoot)
	if err != nil {
		return false
	}
	for _, t := range tasks {
		if t.ID == id {
			return true
		}
	}
	return false
}

// Count returns the number of tasks currently on disk.
func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := platformstore.ReadCronTasks(s.projectRoot)
	if err != nil {
		return 0
	}
	return len(tasks)
}

// ProjectRoot returns the project root directory for this store, used by the
// scheduler to locate the cron file.
func (s *Store) ProjectRoot() string {
	return s.projectRoot
}
