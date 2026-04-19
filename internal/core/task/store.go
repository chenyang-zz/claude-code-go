package task

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/google/uuid"
)

// Store defines the operations that tools need from the task persistence layer.
// Tools depend on this interface rather than the concrete FileStore so that
// testing can substitute a fake implementation.
type Store interface {
	// Create persists a new task with an auto-generated monotonic ID and returns the assigned ID.
	Create(ctx context.Context, data NewTask) (string, error)
	// Get loads a single task by ID. Returns nil without error when the task does not exist.
	Get(ctx context.Context, id string) (*Task, error)
	// List loads all tasks in the current task list directory.
	List(ctx context.Context) ([]*Task, error)
	// Update applies partial updates to an existing task. Returns the updated task or nil if not found.
	Update(ctx context.Context, id string, updates Updates) (*Task, error)
	// UpdateWithDependencies applies an update to a task and the requested reverse
	// dependency mutations while holding a single store lock. Returns the updated
	// task or nil when the task or any dependency target does not exist.
	UpdateWithDependencies(ctx context.Context, taskID string, updates Updates, addBlocks []string, addBlockedBy []string) (*Task, error)
	// Delete removes a task file and cleans up references to it from all other tasks.
	// Returns true when the task was found and deleted.
	Delete(ctx context.Context, id string) (bool, error)
	// BlockTask establishes a bidirectional dependency: fromID blocks toID.
	// Both tasks must exist. Returns false when either task is not found.
	BlockTask(ctx context.Context, fromID, toID string) (bool, error)
}

// NewTask holds the caller-provided fields for creating a new task.
// ID, status, blocks, and blockedBy are set automatically by the store.
type NewTask struct {
	Subject     string         `json:"subject"`
	Description string         `json:"description"`
	ActiveForm  string         `json:"activeForm,omitempty"`
	Owner       string         `json:"owner,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Updates holds optional partial fields for updating an existing task.
// Only non-nil/non-empty fields will be applied.
type Updates struct {
	Subject     *string        `json:"subject,omitempty"`
	Description *string        `json:"description,omitempty"`
	ActiveForm  *string        `json:"activeForm,omitempty"`
	Owner       *string        `json:"owner,omitempty"`
	Status      *Status        `json:"status,omitempty"`
	Blocks      *[]string      `json:"blocks,omitempty"`
	BlockedBy   *[]string      `json:"blockedBy,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// sanitizeRe matches characters that are unsafe in file path components.
var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// SananitizePathComponent rewrites a string so it is safe to use as a single
// path component (directory or file name). Characters outside [a-zA-Z0-9_-]
// are replaced with hyphens.
func SanitizePathComponent(input string) string {
	return sanitizeRe.ReplaceAllString(input, "-")
}

// sessionTaskListID holds the per-process fallback task list ID derived from a
// UUID. It is computed once via sync.Once so all callers within the same
// process receive the same value.
var (
	sessionTaskListID     string
	sessionTaskListIDOnce sync.Once
)

// ResolveTaskListID determines which task list ID to use.
// Priority:
//  1. CLAUDE_CODE_TASK_LIST_ID environment variable — explicit override
//  2. Process-stable UUID fallback — each process gets its own isolated list
func ResolveTaskListID() string {
	if id := os.Getenv("CLAUDE_CODE_TASK_LIST_ID"); id != "" {
		return SanitizePathComponent(id)
	}
	sessionTaskListIDOnce.Do(func() {
		sessionTaskListID = SanitizePathComponent(uuid.NewString())
	})
	return sessionTaskListID
}

// FileStore implements Store using per-task JSON files in a dedicated directory.
// Each task list has its own directory under the config home. The store uses
// syscall.Flock on a .lock file for exclusive access across processes and a
// .highwatermark file for monotonic ID generation.
type FileStore struct {
	// dir is the absolute path to the task list directory.
	dir string
	// mu serializes in-process operations so that concurrent goroutines don't
	// clobber each other. Cross-process safety is handled by flock.
	mu sync.Mutex
}

// NewFileStore creates a FileStore rooted at dir. The directory is created
// lazily on first write.
func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

// tasksDir returns the root directory for task files.
func (s *FileStore) tasksDir() string {
	return s.dir
}

// taskPath returns the file path for a single task JSON file.
func (s *FileStore) taskPath(id string) string {
	return filepath.Join(s.tasksDir(), SanitizePathComponent(id)+".json")
}

// lockPath returns the path to the exclusive lock file for this task list.
func (s *FileStore) lockPath() string {
	return filepath.Join(s.tasksDir(), ".lock")
}

// highwatermarkPath returns the path to the monotonic ID file.
func (s *FileStore) highwatermarkPath() string {
	return filepath.Join(s.tasksDir(), ".highwatermark")
}

// ensureDir creates the task list directory and lock file if they do not exist.
func (s *FileStore) ensureDir() error {
	if err := os.MkdirAll(s.tasksDir(), 0o755); err != nil {
		return fmt.Errorf("task store: create dir %s: %w", s.tasksDir(), err)
	}
	// Ensure the lock file exists so we can flock it.
	lockPath := s.lockPath()
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("task store: create lock file: %w", err)
	}
	f.Close()
	return nil
}

// withLock acquires an exclusive flock on the task list lock file, calls fn,
// and releases the lock. The lock is held for the entire duration of fn,
// serializing all writes across processes and in-process goroutines.
func (s *FileStore) withLock(fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureDir(); err != nil {
		return err
	}

	f, err := os.OpenFile(s.lockPath(), os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("task store: open lock file: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("task store: acquire lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	return fn()
}

// readHighwatermark reads the current high-water mark value from disk.
// Returns 0 if the file does not exist or cannot be parsed.
func (s *FileStore) readHighwatermark() int {
	data, err := os.ReadFile(s.highwatermarkPath())
	if err != nil {
		return 0
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return v
}

// writeHighwatermark persists the high-water mark value.
func (s *FileStore) writeHighwatermark(value int) error {
	return os.WriteFile(s.highwatermarkPath(), []byte(strconv.Itoa(value)), 0o644)
}

// findHighestTaskID scans existing task files and returns the highest numeric
// ID found. Returns 0 if no task files exist.
func (s *FileStore) findHighestTaskID() (int, error) {
	entries, err := os.ReadDir(s.tasksDir())
	if err != nil {
		return 0, nil
	}
	highest := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		id, err := strconv.Atoi(name)
		if err == nil && id > highest {
			highest = id
		}
	}
	return highest, nil
}

// nextID computes the next monotonic task ID under lock.
// Must be called while holding the file lock.
func (s *FileStore) nextID() (string, error) {
	fromFiles, err := s.findHighestTaskID()
	if err != nil {
		return "", err
	}
	fromMark := s.readHighwatermark()
	highest := max(fromFiles, fromMark)
	next := highest + 1
	return strconv.Itoa(next), nil
}

// readTaskFile reads and parses a single task JSON file.
func (s *FileStore) readTaskFile(path string) (*Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("task store: read file %s: %w", path, err)
	}
	var t Task
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("task store: parse file %s: %w", path, err)
	}
	return &t, nil
}

// writeTaskFile serializes and writes a task to its JSON file.
func (s *FileStore) writeTaskFile(t *Task) error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("task store: marshal task %s: %w", t.ID, err)
	}
	return os.WriteFile(s.taskPath(t.ID), data, 0o644)
}

// Create persists a new task with an auto-generated monotonic ID.
func (s *FileStore) Create(_ context.Context, data NewTask) (string, error) {
	var id string
	err := s.withLock(func() error {
		next, err := s.nextID()
		if err != nil {
			return err
		}
		id = next
		t := &Task{
			ID:          id,
			Subject:     data.Subject,
			Description: data.Description,
			ActiveForm:  data.ActiveForm,
			Owner:       data.Owner,
			Status:      StatusPending,
			Blocks:      []string{},
			BlockedBy:   []string{},
			Metadata:    data.Metadata,
		}
		return s.writeTaskFile(t)
	})
	return id, err
}

// Get loads a single task by ID under a shared lock so concurrent writes
// cannot produce partially-read JSON.
func (s *FileStore) Get(_ context.Context, id string) (*Task, error) {
	var result *Task
	err := s.withLock(func() error {
		if err := s.ensureDir(); err != nil {
			return err
		}
		var readErr error
		result, readErr = s.readTaskFile(s.taskPath(id))
		return readErr
	})
	return result, err
}

// List loads all tasks in the current task list directory under a shared lock.
func (s *FileStore) List(_ context.Context) ([]*Task, error) {
	var tasks []*Task
	err := s.withLock(func() error {
		var listErr error
		tasks, listErr = s.listLocked()
		return listErr
	})
	return tasks, err
}

// listLocked reads all tasks without acquiring the lock. The caller must
// already hold the store lock (e.g. from within a withLock callback).
func (s *FileStore) listLocked() ([]*Task, error) {
	if err := s.ensureDir(); err != nil {
		return nil, err
	}
	entries, readErr := os.ReadDir(s.tasksDir())
	if readErr != nil {
		return nil, nil
	}
	var tasks []*Task
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		t, fileErr := s.readTaskFile(s.taskPath(id))
		if fileErr != nil {
			continue
		}
		if t != nil {
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}

// Update applies partial updates to an existing task and persists the result.
func (s *FileStore) Update(_ context.Context, id string, updates Updates) (*Task, error) {
	var result *Task
	err := s.withLock(func() error {
		t, err := s.readTaskFile(s.taskPath(id))
		if err != nil {
			return err
		}
		if t == nil {
			return nil
		}
		applyUpdates(t, updates)
		if err := s.writeTaskFile(t); err != nil {
			return err
		}
		result = t
		return nil
	})
	return result, err
}

// UpdateWithDependencies applies partial updates to the current task and any
// required reverse dependency references while holding a single file lock.
func (s *FileStore) UpdateWithDependencies(_ context.Context, taskID string, updates Updates, addBlocks []string, addBlockedBy []string) (*Task, error) {
	var result *Task
	err := s.withLock(func() error {
		current, err := s.readTaskFile(s.taskPath(taskID))
		if err != nil {
			return err
		}
		if current == nil {
			return nil
		}

		blockTargets := make([]*Task, 0, len(addBlocks))
		for _, targetID := range addBlocks {
			target, err := s.readTaskFile(s.taskPath(targetID))
			if err != nil {
				return err
			}
			if target == nil {
				return nil
			}
			blockTargets = append(blockTargets, target)
		}

		blockerTargets := make([]*Task, 0, len(addBlockedBy))
		for _, targetID := range addBlockedBy {
			target, err := s.readTaskFile(s.taskPath(targetID))
			if err != nil {
				return err
			}
			if target == nil {
				return nil
			}
			blockerTargets = append(blockerTargets, target)
		}

		applyUpdates(current, updates)
		if err := s.writeTaskFile(current); err != nil {
			return err
		}

		for _, target := range blockTargets {
			if !containsString(target.BlockedBy, taskID) {
				target.BlockedBy = append(target.BlockedBy, taskID)
				if err := s.writeTaskFile(target); err != nil {
					return err
				}
			}
		}

		for _, target := range blockerTargets {
			if !containsString(target.Blocks, taskID) {
				target.Blocks = append(target.Blocks, taskID)
				if err := s.writeTaskFile(target); err != nil {
					return err
				}
			}
		}

		result = current
		return nil
	})
	return result, err
}

// applyUpdates sets non-nil fields from updates onto the task in place.
func applyUpdates(t *Task, u Updates) {
	if u.Subject != nil {
		t.Subject = *u.Subject
	}
	if u.Description != nil {
		t.Description = *u.Description
	}
	if u.ActiveForm != nil {
		t.ActiveForm = *u.ActiveForm
	}
	if u.Owner != nil {
		t.Owner = *u.Owner
	}
	if u.Status != nil {
		t.Status = *u.Status
	}
	if u.Blocks != nil {
		t.Blocks = *u.Blocks
	}
	if u.BlockedBy != nil {
		t.BlockedBy = *u.BlockedBy
	}
	if u.Metadata != nil {
		if t.Metadata == nil {
			t.Metadata = make(map[string]any)
		}
		for k, v := range u.Metadata {
			if v == nil {
				delete(t.Metadata, k)
			} else {
				t.Metadata[k] = v
			}
		}
	}
}

// Delete removes a task file and cleans up all references to it from other
// tasks in the same list.
func (s *FileStore) Delete(_ context.Context, id string) (bool, error) {
	var deleted bool
	err := s.withLock(func() error {
		path := s.taskPath(id)

		// Update highwatermark before deletion to prevent ID reuse.
		numericID, err := strconv.Atoi(id)
		if err == nil {
			current := s.readHighwatermark()
			if numericID > current {
				if err := s.writeHighwatermark(numericID); err != nil {
					return err
				}
			}
		}

		// Remove the task file.
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		deleted = true

		// Clean up references in remaining tasks.
		tasks, err := s.listLocked()
		if err != nil {
			return nil // best-effort cleanup
		}
		for _, t := range tasks {
			newBlocks := removeString(t.Blocks, id)
			newBlockedBy := removeString(t.BlockedBy, id)
			if len(newBlocks) != len(t.Blocks) || len(newBlockedBy) != len(t.BlockedBy) {
				t.Blocks = newBlocks
				t.BlockedBy = newBlockedBy
				_ = s.writeTaskFile(t)
			}
		}
		return nil
	})
	return deleted, err
}

// BlockTask establishes a bidirectional dependency: fromID blocks toID.
func (s *FileStore) BlockTask(_ context.Context, fromID, toID string) (bool, error) {
	var ok bool
	err := s.withLock(func() error {
		from, err := s.readTaskFile(s.taskPath(fromID))
		if err != nil || from == nil {
			return nil
		}
		to, err := s.readTaskFile(s.taskPath(toID))
		if err != nil || to == nil {
			return nil
		}

		// from blocks to
		if !containsString(from.Blocks, toID) {
			from.Blocks = append(from.Blocks, toID)
			if err := s.writeTaskFile(from); err != nil {
				return err
			}
		}

		// to is blockedBy from
		if !containsString(to.BlockedBy, fromID) {
			to.BlockedBy = append(to.BlockedBy, fromID)
			if err := s.writeTaskFile(to); err != nil {
				return err
			}
		}

		ok = true
		return nil
	})
	return ok, err
}

// containsString reports whether slice contains s.
func containsString(slice []string, s string) bool {
	return slices.Contains(slice, s)
}

// removeString returns a new slice with all occurrences of s removed.
func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, v := range slice {
		if v != s {
			result = append(result, v)
		}
	}
	return result
}
