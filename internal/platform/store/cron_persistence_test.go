package store

import (
	"testing"
	"time"
)

func TestReadCronTasksEmptyFile(t *testing.T) {
	dir := t.TempDir()
	tasks, err := ReadCronTasks(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestWriteAndReadCronTasks(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	tasks := []CronTask{
		{ID: "abc", Cron: "*/5 * * * *", Prompt: "test", CreatedAt: now, Recurring: true},
		{ID: "def", Cron: "0 9 * * *", Prompt: "daily", CreatedAt: now, Recurring: false},
	}

	if err := WriteCronTasks(dir, tasks); err != nil {
		t.Fatalf("write error: %v", err)
	}

	read, err := ReadCronTasks(dir)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if len(read) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(read))
	}
	if read[0].ID != "abc" {
		t.Errorf("expected first task ID 'abc', got %q", read[0].ID)
	}
	if read[1].ID != "def" {
		t.Errorf("expected second task ID 'def', got %q", read[1].ID)
	}
}

func TestWriteCronTasksNilSlice(t *testing.T) {
	dir := t.TempDir()
	if err := WriteCronTasks(dir, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should create an empty file.
	tasks, err := ReadCronTasks(dir)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestAddCronTask(t *testing.T) {
	dir := t.TempDir()

	task, err := AddCronTask(dir, "*/5 * * * *", "test prompt", true, false)
	if err != nil {
		t.Fatalf("add error: %v", err)
	}
	if task.ID == "" {
		t.Error("expected non-empty ID")
	}
	if task.Cron != "*/5 * * * *" {
		t.Errorf("expected cron, got %q", task.Cron)
	}
	if !task.Recurring {
		t.Error("expected recurring")
	}

	// Verify it was persisted.
	tasks, err := ReadCronTasks(dir)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
}

func TestRemoveCronTasks(t *testing.T) {
	dir := t.TempDir()

	_, _ = AddCronTask(dir, "*/5 * * * *", "keep", true, false)
	task2, _ := AddCronTask(dir, "0 9 * * *", "remove me", false, false)

	if err := RemoveCronTasks(dir, []string{task2.ID}); err != nil {
		t.Fatalf("remove error: %v", err)
	}

	tasks, _ := ReadCronTasks(dir)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after remove, got %d", len(tasks))
	}
	if tasks[0].Prompt != "keep" {
		t.Errorf("expected 'keep' task, got %q", tasks[0].Prompt)
	}
}

func TestRemoveCronTasksEmpty(t *testing.T) {
	dir := t.TempDir()
	// Empty id list should be a no-op.
	if err := RemoveCronTasks(dir, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := RemoveCronTasks(dir, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkCronTasksFired(t *testing.T) {
	dir := t.TempDir()

	task, _ := AddCronTask(dir, "*/5 * * * *", "recurring", true, false)
	firedAt := time.Now()

	if err := MarkCronTasksFired(dir, []string{task.ID}, firedAt); err != nil {
		t.Fatalf("mark error: %v", err)
	}

	tasks, _ := ReadCronTasks(dir)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].LastFiredAt == nil {
		t.Error("expected LastFiredAt to be set")
	} else if !tasks[0].LastFiredAt.Equal(firedAt) {
		t.Errorf("expected LastFiredAt %v, got %v", firedAt, *tasks[0].LastFiredAt)
	}
}

func TestMarkCronTasksFiredEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := MarkCronTasksFired(dir, nil, time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHasCronTasksSync(t *testing.T) {
	dir := t.TempDir()

	if HasCronTasksSync(dir) {
		t.Error("expected false for empty dir")
	}

	_, _ = AddCronTask(dir, "*/5 * * * *", "test", true, false)
	if !HasCronTasksSync(dir) {
		t.Error("expected true after adding task")
	}
}

func TestListAllCronTasks(t *testing.T) {
	dir := t.TempDir()

	_, _ = AddCronTask(dir, "*/5 * * * *", "first", true, false)
	_, _ = AddCronTask(dir, "0 9 * * *", "second", false, false)

	tasks, err := ListAllCronTasks(dir)
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
}
