package task

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// newTestStore creates a FileStore rooted in a temporary directory.
func newTestStore(t *testing.T) *FileStore {
	t.Helper()
	dir := t.TempDir()
	return NewFileStore(dir)
}

func TestCreateAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.Create(ctx, NewTask{
		Subject:     "Test task",
		Description: "A test description",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if id != "1" {
		t.Fatalf("Create() id = %q, want %q", id, "1")
	}

	task, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if task == nil {
		t.Fatal("Get() returned nil task")
	}
	if task.Subject != "Test task" {
		t.Errorf("Subject = %q, want %q", task.Subject, "Test task")
	}
	if task.Status != StatusPending {
		t.Errorf("Status = %q, want %q", task.Status, StatusPending)
	}
	if len(task.Blocks) != 0 || len(task.BlockedBy) != 0 {
		t.Errorf("Blocks = %v, BlockedBy = %v, want empty", task.Blocks, task.BlockedBy)
	}
}

func TestGetNonexistent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	task, err := s.Get(ctx, "999")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if task != nil {
		t.Fatal("Get() should return nil for nonexistent task")
	}
}

func TestMonotonicID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _ := s.Create(ctx, NewTask{Subject: "First", Description: "1"})
	id2, _ := s.Create(ctx, NewTask{Subject: "Second", Description: "2"})
	id3, _ := s.Create(ctx, NewTask{Subject: "Third", Description: "3"})

	if id1 != "1" || id2 != "2" || id3 != "3" {
		t.Fatalf("IDs = %q, %q, %q; want 1, 2, 3", id1, id2, id3)
	}
}

func TestMonotonicIDAfterDelete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _ := s.Create(ctx, NewTask{Subject: "First", Description: "1"})
	_, _ = s.Create(ctx, NewTask{Subject: "Second", Description: "2"})

	deleted, err := s.Delete(ctx, id1)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatal("Delete() should return true")
	}

	// Next ID should be 3, not reusing deleted 1.
	id3, _ := s.Create(ctx, NewTask{Subject: "Third", Description: "3"})
	if id3 != "3" {
		t.Fatalf("Create after delete id = %q, want %q", id3, "3")
	}
}

func TestList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.Create(ctx, NewTask{Subject: "A", Description: "1"})
	s.Create(ctx, NewTask{Subject: "B", Description: "2"})

	tasks, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("List() returned %d tasks, want 2", len(tasks))
	}
}

func TestUpdate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, _ := s.Create(ctx, NewTask{Subject: "Original", Description: "desc"})

	newSubject := "Updated"
	newStatus := StatusInProgress
	updated, err := s.Update(ctx, id, Updates{
		Subject: &newSubject,
		Status:  &newStatus,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated == nil {
		t.Fatal("Update() returned nil")
	}
	if updated.Subject != "Updated" {
		t.Errorf("Subject = %q, want %q", updated.Subject, "Updated")
	}
	if updated.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q", updated.Status, StatusInProgress)
	}
}

func TestUpdateWithDependencies(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _ := s.Create(ctx, NewTask{Subject: "A", Description: "a"})
	id2, _ := s.Create(ctx, NewTask{Subject: "B", Description: "b"})
	id3, _ := s.Create(ctx, NewTask{Subject: "C", Description: "c"})

	newSubject := "Updated"
	mergedBlocks := []string{id2}
	mergedBlockedBy := []string{id3}

	updated, err := s.UpdateWithDependencies(ctx, id1, Updates{
		Subject:   &newSubject,
		Blocks:    &mergedBlocks,
		BlockedBy: &mergedBlockedBy,
	}, []string{id2}, []string{id3})
	if err != nil {
		t.Fatalf("UpdateWithDependencies() error = %v", err)
	}
	if updated == nil {
		t.Fatal("UpdateWithDependencies() returned nil")
	}
	if updated.Subject != "Updated" {
		t.Errorf("Subject = %q, want %q", updated.Subject, "Updated")
	}
	if len(updated.Blocks) != 1 || updated.Blocks[0] != id2 {
		t.Errorf("Blocks = %v, want [%s]", updated.Blocks, id2)
	}
	if len(updated.BlockedBy) != 1 || updated.BlockedBy[0] != id3 {
		t.Errorf("BlockedBy = %v, want [%s]", updated.BlockedBy, id3)
	}

	task2, _ := s.Get(ctx, id2)
	if !containsString(task2.BlockedBy, id1) {
		t.Errorf("task2.BlockedBy = %v, want to contain %s", task2.BlockedBy, id1)
	}

	task3, _ := s.Get(ctx, id3)
	if !containsString(task3.Blocks, id1) {
		t.Errorf("task3.Blocks = %v, want to contain %s", task3.Blocks, id1)
	}
}

func TestUpdateWithDependenciesMissingTargetDoesNotPartiallyUpdate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _ := s.Create(ctx, NewTask{Subject: "A", Description: "a"})
	id2, _ := s.Create(ctx, NewTask{Subject: "B", Description: "b"})

	newSubject := "Updated"
	mergedBlocks := []string{"999"}

	updated, err := s.UpdateWithDependencies(ctx, id1, Updates{
		Subject: &newSubject,
		Blocks:  &mergedBlocks,
	}, []string{"999"}, nil)
	if err != nil {
		t.Fatalf("UpdateWithDependencies() error = %v", err)
	}
	if updated != nil {
		t.Fatal("UpdateWithDependencies() should return nil when a dependency target is missing")
	}

	task1, _ := s.Get(ctx, id1)
	if task1.Subject != "A" {
		t.Errorf("task1.Subject = %q, want %q", task1.Subject, "A")
	}
	if len(task1.Blocks) != 0 {
		t.Errorf("task1.Blocks = %v, want empty", task1.Blocks)
	}

	task2, _ := s.Get(ctx, id2)
	if len(task2.BlockedBy) != 0 {
		t.Errorf("task2.BlockedBy = %v, want empty", task2.BlockedBy)
	}
}

func TestUpdateNonexistent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	newSubject := "Nope"
	updated, err := s.Update(ctx, "999", Updates{Subject: &newSubject})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated != nil {
		t.Fatal("Update() should return nil for nonexistent task")
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, _ := s.Create(ctx, NewTask{Subject: "ToDelete", Description: "d"})

	deleted, err := s.Delete(ctx, id)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatal("Delete() should return true")
	}

	task, _ := s.Get(ctx, id)
	if task != nil {
		t.Fatal("Get() after delete should return nil")
	}
}

func TestDeleteNonexistent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	deleted, err := s.Delete(ctx, "999")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleted {
		t.Fatal("Delete() should return false for nonexistent task")
	}
}

func TestDeleteCleansReferences(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _ := s.Create(ctx, NewTask{Subject: "Blocker", Description: "b"})
	id2, _ := s.Create(ctx, NewTask{Subject: "Blocked", Description: "d"})

	// id1 blocks id2
	ok, err := s.BlockTask(ctx, id1, id2)
	if err != nil || !ok {
		t.Fatalf("BlockTask() ok=%v err=%v", ok, err)
	}

	// Delete id1 should clean references in id2.
	deleted, _ := s.Delete(ctx, id1)
	if !deleted {
		t.Fatal("Delete() should succeed")
	}

	task2, _ := s.Get(ctx, id2)
	if task2 == nil {
		t.Fatal("Task 2 should still exist")
	}
	if len(task2.BlockedBy) != 0 {
		t.Errorf("BlockedBy = %v, want empty after blocker deletion", task2.BlockedBy)
	}
}

func TestBlockTask(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _ := s.Create(ctx, NewTask{Subject: "A", Description: "a"})
	id2, _ := s.Create(ctx, NewTask{Subject: "B", Description: "b"})

	ok, err := s.BlockTask(ctx, id1, id2)
	if err != nil {
		t.Fatalf("BlockTask() error = %v", err)
	}
	if !ok {
		t.Fatal("BlockTask() should return true")
	}

	task1, _ := s.Get(ctx, id1)
	task2, _ := s.Get(ctx, id2)

	if !containsString(task1.Blocks, id2) {
		t.Error("task1.Blocks should contain id2")
	}
	if !containsString(task2.BlockedBy, id1) {
		t.Error("task2.BlockedBy should contain id1")
	}
}

func TestBlockTaskNonexistent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	ok, err := s.BlockTask(ctx, "1", "2")
	if err != nil {
		t.Fatalf("BlockTask() error = %v", err)
	}
	if ok {
		t.Fatal("BlockTask() should return false for nonexistent tasks")
	}
}

func TestBlockTaskIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _ := s.Create(ctx, NewTask{Subject: "A", Description: "a"})
	id2, _ := s.Create(ctx, NewTask{Subject: "B", Description: "b"})

	s.BlockTask(ctx, id1, id2)
	ok, _ := s.BlockTask(ctx, id1, id2)
	if !ok {
		t.Fatal("Second BlockTask() should still return true")
	}

	task1, _ := s.Get(ctx, id1)
	count := 0
	for _, id := range task1.Blocks {
		if id == id2 {
			count++
		}
	}
	if count != 1 {
		t.Errorf("id2 appears %d times in Blocks, want 1", count)
	}
}

func TestUpdateMetadataMerge(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, _ := s.Create(ctx, NewTask{
		Subject:     "Meta",
		Description: "d",
		Metadata:    map[string]any{"a": 1, "b": 2},
	})

	// Merge: update "a", add "c", delete "b" via null.
	_, err := s.Update(ctx, id, Updates{
		Metadata: map[string]any{"a": 10, "b": nil, "c": 3},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	task, _ := s.Get(ctx, id)
	if task.Metadata["a"] != float64(10) {
		t.Errorf("metadata[a] = %v, want 10", task.Metadata["a"])
	}
	if _, exists := task.Metadata["b"]; exists {
		t.Error("metadata[b] should be deleted")
	}
	if task.Metadata["c"] != float64(3) {
		t.Errorf("metadata[c] = %v, want 3", task.Metadata["c"])
	}
}

func TestSanitizePathComponent(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello", "hello"},
		{"hello world", "hello-world"},
		{"../../etc/passwd", "------etc-passwd"},
		{"a-b_c", "a-b_c"},
		{"UPPER", "UPPER"},
	}
	for _, tt := range tests {
		got := SanitizePathComponent(tt.input)
		if got != tt.want {
			t.Errorf("SanitizePathComponent(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveTaskListIDSanitizesOverride(t *testing.T) {
	t.Setenv("CLAUDE_CODE_TASK_LIST_ID", "../../tmp/evil")

	if got, want := ResolveTaskListID(), "------tmp-evil"; got != want {
		t.Fatalf("ResolveTaskListID() = %q, want %q", got, want)
	}
}

func TestHighwatermarkFileCreated(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _ = s.Create(ctx, NewTask{Subject: "T", Description: "d"})

	hwmPath := filepath.Join(s.dir, ".highwatermark")
	// The highwatermark file is only written on deletion, so it shouldn't exist yet.
	if _, err := os.Stat(hwmPath); !os.IsNotExist(err) {
		t.Log("Highwatermark file exists (expected on some implementations)")
	}

	// Delete the task — this should create/update the highwatermark.
	_, _ = s.Delete(ctx, "1")

	data, err := os.ReadFile(hwmPath)
	if err != nil {
		t.Fatalf("ReadFile(highwatermark) error = %v", err)
	}
	if string(data) != "1" {
		t.Errorf("highwatermark = %q, want %q", string(data), "1")
	}
}

func TestListEmpty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	tasks, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("List() on empty store returned %d tasks, want 0", len(tasks))
	}
}

func TestClaimTask(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.Create(ctx, NewTask{Subject: "Task 1", Description: "desc"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Successful claim.
	result, err := s.ClaimTask(ctx, id, "agent-a", ClaimTaskOptions{})
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if !result.Success {
		t.Fatalf("ClaimTask() success = false, want true")
	}
	if result.Task.Owner != "agent-a" {
		t.Errorf("owner = %q, want %q", result.Task.Owner, "agent-a")
	}

	// Re-claim by same agent should succeed (idempotent).
	result, err = s.ClaimTask(ctx, id, "agent-a", ClaimTaskOptions{})
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if !result.Success {
		t.Fatalf("re-claim by same agent failed")
	}

	// Claim by different agent should fail.
	result, err = s.ClaimTask(ctx, id, "agent-b", ClaimTaskOptions{})
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if result.Success {
		t.Fatal("claim by different agent should fail")
	}
	if result.Reason != ClaimReasonAlreadyClaimed {
		t.Errorf("reason = %q, want %q", result.Reason, ClaimReasonAlreadyClaimed)
	}
}

func TestClaimTaskNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	result, err := s.ClaimTask(ctx, "99", "agent-a", ClaimTaskOptions{})
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if result.Success {
		t.Fatal("claim of nonexistent task should fail")
	}
	if result.Reason != ClaimReasonTaskNotFound {
		t.Errorf("reason = %q, want %q", result.Reason, ClaimReasonTaskNotFound)
	}
}

func TestClaimTaskAlreadyResolved(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, _ := s.Create(ctx, NewTask{Subject: "Task", Description: "desc"})
	completed := StatusCompleted
	s.Update(ctx, id, Updates{Status: &completed})

	result, err := s.ClaimTask(ctx, id, "agent-a", ClaimTaskOptions{})
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if result.Success {
		t.Fatal("claim of completed task should fail")
	}
	if result.Reason != ClaimReasonAlreadyResolved {
		t.Errorf("reason = %q, want %q", result.Reason, ClaimReasonAlreadyResolved)
	}
}

func TestClaimTaskBlocked(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _ := s.Create(ctx, NewTask{Subject: "Blocker", Description: "desc"})
	id2, _ := s.Create(ctx, NewTask{Subject: "Blocked", Description: "desc"})
	// Make id1 block id2.
	s.BlockTask(ctx, id1, id2)

	result, err := s.ClaimTask(ctx, id2, "agent-a", ClaimTaskOptions{})
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if result.Success {
		t.Fatal("claim of blocked task should fail")
	}
	if result.Reason != ClaimReasonBlocked {
		t.Errorf("reason = %q, want %q", result.Reason, ClaimReasonBlocked)
	}
	if len(result.BlockedByTasks) != 1 || result.BlockedByTasks[0] != id1 {
		t.Errorf("blockedByTasks = %v, want [%s]", result.BlockedByTasks, id1)
	}
}

func TestClaimTaskWithBusyCheck(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _ := s.Create(ctx, NewTask{Subject: "Task 1", Description: "desc"})
	id2, _ := s.Create(ctx, NewTask{Subject: "Task 2", Description: "desc"})

	// Claim first task.
	result, err := s.ClaimTask(ctx, id1, "agent-a", ClaimTaskOptions{CheckAgentBusy: true})
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if !result.Success {
		t.Fatalf("first claim should succeed")
	}

	// Claim second task with busy check should fail.
	result, err = s.ClaimTask(ctx, id2, "agent-a", ClaimTaskOptions{CheckAgentBusy: true})
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if result.Success {
		t.Fatal("second claim with busy check should fail")
	}
	if result.Reason != ClaimReasonAgentBusy {
		t.Errorf("reason = %q, want %q", result.Reason, ClaimReasonAgentBusy)
	}
	if len(result.BusyWithTasks) != 1 || result.BusyWithTasks[0] != id1 {
		t.Errorf("busyWithTasks = %v, want [%s]", result.BusyWithTasks, id1)
	}
}

func TestClaimTaskWithoutBusyCheckAllowsMultiple(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _ := s.Create(ctx, NewTask{Subject: "Task 1", Description: "desc"})
	id2, _ := s.Create(ctx, NewTask{Subject: "Task 2", Description: "desc"})

	_, _ = s.ClaimTask(ctx, id1, "agent-a", ClaimTaskOptions{})
	result, err := s.ClaimTask(ctx, id2, "agent-a", ClaimTaskOptions{})
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if !result.Success {
		t.Fatal("second claim without busy check should succeed")
	}
}

func TestResetTaskList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _ = s.Create(ctx, NewTask{Subject: "Task 1", Description: "desc"})
	_, _ = s.Create(ctx, NewTask{Subject: "Task 2", Description: "desc"})

	if err := s.ResetTaskList(ctx); err != nil {
		t.Fatalf("ResetTaskList() error = %v", err)
	}

	tasks, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("List() after reset returned %d tasks, want 0", len(tasks))
	}

	// Highwatermark should be set to prevent ID reuse.
	hwmPath := filepath.Join(s.dir, ".highwatermark")
	data, err := os.ReadFile(hwmPath)
	if err != nil {
		t.Fatalf("ReadFile(highwatermark) error = %v", err)
	}
	if string(data) != "2" {
		t.Errorf("highwatermark = %q, want %q", string(data), "2")
	}

	// Next creation should start after the highwatermark.
	id3, _ := s.Create(ctx, NewTask{Subject: "Task 3", Description: "desc"})
	if id3 != "3" {
		t.Errorf("next id = %q, want %q", id3, "3")
	}
}

func TestUnassignTeammateTasks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _ := s.Create(ctx, NewTask{Subject: "Task 1", Description: "desc"})
	id2, _ := s.Create(ctx, NewTask{Subject: "Task 2", Description: "desc"})
	id3, _ := s.Create(ctx, NewTask{Subject: "Task 3", Description: "desc"})

	// Claim tasks for agent-a.
	_, _ = s.ClaimTask(ctx, id1, "agent-a", ClaimTaskOptions{})
	_, _ = s.ClaimTask(ctx, id2, "agent-a", ClaimTaskOptions{})
	// Complete one task.
	completed := StatusCompleted
	_, _ = s.Update(ctx, id2, Updates{Status: &completed})

	result, err := s.UnassignTeammateTasks(ctx, "agent-a")
	if err != nil {
		t.Fatalf("UnassignTeammateTasks() error = %v", err)
	}
	if len(result.UnassignedTasks) != 1 {
		t.Fatalf("unassigned tasks = %d, want 1", len(result.UnassignedTasks))
	}
	if result.UnassignedTasks[0].ID != id1 {
		t.Errorf("unassigned task id = %q, want %q", result.UnassignedTasks[0].ID, id1)
	}

	// Verify the unassigned task is now pending with no owner.
	task, _ := s.Get(ctx, id1)
	if task.Owner != "" {
		t.Errorf("owner = %q, want empty", task.Owner)
	}
	if task.Status != StatusPending {
		t.Errorf("status = %q, want %q", task.Status, StatusPending)
	}

	// Completed task should remain unchanged.
	task2, _ := s.Get(ctx, id2)
	if task2.Status != StatusCompleted {
		t.Errorf("completed task status = %q, want %q", task2.Status, StatusCompleted)
	}

	// Unclaimed task should remain unchanged.
	task3, _ := s.Get(ctx, id3)
	if task3.Owner != "" {
		t.Errorf("unclaimed task owner = %q, want empty", task3.Owner)
	}
}
