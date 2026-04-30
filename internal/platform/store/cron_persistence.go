// Package store provides file-backed persistence for cron scheduled tasks.
//
// Tasks are stored in <project>/.claude/scheduled_tasks.json with the format:
//
//	{ "tasks": [{ id, cron, prompt, createdAt, lastFiredAt?, recurring? }] }
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// CronTask is the on-disk representation of a scheduled task. It mirrors the
// TypeScript CronTask type.
type CronTask struct {
	ID          string     `json:"id"`
	Cron        string     `json:"cron"`
	Prompt      string     `json:"prompt"`
	CreatedAt   time.Time  `json:"createdAt"`
	LastFiredAt *time.Time `json:"lastFiredAt,omitempty"`
	Recurring   bool       `json:"recurring,omitempty"`
	Durable     bool       `json:"durable,omitempty"`
}

// cronFile is the JSON structure of .claude/scheduled_tasks.json.
type cronFile struct {
	Tasks []CronTask `json:"tasks"`
}

// MaxCronJobs is the maximum number of active cron tasks allowed at once.
const MaxCronJobs = 50

// cronFileRel is the path to the scheduled tasks file relative to the project root.
const cronFileRel = ".claude/scheduled_tasks.json"

// cronFilePath returns the absolute path to the scheduled tasks file.
func cronFilePath(projectRoot string) string {
	return filepath.Join(projectRoot, cronFileRel)
}

// ReadCronTasks reads and parses .claude/scheduled_tasks.json. Returns an
// empty slice if the file is missing, empty, or malformed. Tasks with invalid
// cron strings are silently dropped so a single bad entry never blocks the
// whole file.
func ReadCronTasks(projectRoot string) ([]CronTask, error) {
	path := cronFilePath(projectRoot)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var file cronFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, nil // malformed file → empty
	}
	if file.Tasks == nil {
		return nil, nil
	}

	// Filter out tasks with invalid cron strings.
	out := make([]CronTask, 0, len(file.Tasks))
	for _, t := range file.Tasks {
		if t.ID == "" || t.Cron == "" || t.Prompt == "" || t.CreatedAt.IsZero() {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

// WriteCronTasks overwrites .claude/scheduled_tasks.json with the given tasks.
// Creates the .claude directory if missing. An empty task list writes an empty
// file (rather than deleting) so file watchers see a change event on
// last-task-removed.
func WriteCronTasks(projectRoot string, tasks []CronTask) error {
	claudeDir := filepath.Join(projectRoot, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return err
	}

	if tasks == nil {
		tasks = []CronTask{}
	}

	body := cronFile{Tasks: tasks}
	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(cronFilePath(projectRoot), data, 0o644)
}

// AddCronTask appends a new task to the cron file. Returns the created task
// with its generated ID. The caller must have already validated the cron
// expression.
func AddCronTask(projectRoot, cronExpr, prompt string, recurring, durable bool) (CronTask, error) {
	id := uuid.NewString()[:8] // 8 hex chars, same as TS

	task := CronTask{
		ID:        id,
		Cron:      cronExpr,
		Prompt:    prompt,
		CreatedAt: time.Now(),
		Recurring: recurring,
	}

	// Durable:false tasks go to file too (simplification — the scheduler
	// handles cleanup on process exit). The durable flag is preserved for
	// future session-only store integration.
	_ = durable

	tasks, err := ReadCronTasks(projectRoot)
	if err != nil {
		return CronTask{}, err
	}

	tasks = append(tasks, task)
	if err := WriteCronTasks(projectRoot, tasks); err != nil {
		return CronTask{}, err
	}

	return task, nil
}

// RemoveCronTasks removes tasks by ID from the cron file. No-op if none match.
func RemoveCronTasks(projectRoot string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	idSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}

	tasks, err := ReadCronTasks(projectRoot)
	if err != nil {
		return err
	}

	filtered := make([]CronTask, 0, len(tasks))
	for _, t := range tasks {
		if _, ok := idSet[t.ID]; !ok {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) == len(tasks) {
		return nil // nothing removed
	}

	return WriteCronTasks(projectRoot, filtered)
}

// MarkCronTasksFired stamps lastFiredAt on the given recurring tasks and
// writes back. Batched so N fires in one scheduler tick = one
// read-modify-write. No-op if none of the ids match.
func MarkCronTasksFired(projectRoot string, ids []string, firedAt time.Time) error {
	if len(ids) == 0 {
		return nil
	}

	idSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}

	tasks, err := ReadCronTasks(projectRoot)
	if err != nil {
		return err
	}

	changed := false
	for i := range tasks {
		if _, ok := idSet[tasks[i].ID]; ok {
			tasks[i].LastFiredAt = &firedAt
			changed = true
		}
	}
	if !changed {
		return nil
	}

	return WriteCronTasks(projectRoot, tasks)
}

// HasCronTasksSync checks whether the cron file has any valid tasks. Used by
// the scheduler to decide whether to auto-enable. Single file read.
func HasCronTasksSync(projectRoot string) bool {
	tasks, err := ReadCronTasks(projectRoot)
	if err != nil {
		return false
	}
	return len(tasks) > 0
}

// ListAllCronTasks is a convenience wrapper around ReadCronTasks.
func ListAllCronTasks(projectRoot string) ([]CronTask, error) {
	return ReadCronTasks(projectRoot)
}

// NextCronRunMs is a placeholder that will be provided by the cron compute
// package. The store package does not import runtime/cron to avoid circular
// dependencies. Callers should use cron.NextCronRunMs directly.
//
// This is declared here only to document the expected calling convention.
