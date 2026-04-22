package task_update

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	coretask "github.com/sheepzhao/claude-code-go/internal/core/task"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// mockUpdateStore implements TaskUpdater for testing.
type mockUpdateStore struct {
	updated                      *coretask.Task
	updateCalled                 bool
	updateWithDependenciesCalled bool
	deleted                      bool
	deleteErr                    error
	updateErr                    error
	getTask                      *coretask.Task
	tasksByID                    map[string]*coretask.Task
	lastDependencyTaskID         string
	lastDependencyUpdates        coretask.Updates
	lastAddBlocks                []string
	lastAddBlockedBy             []string
}

func (m *mockUpdateStore) Get(_ context.Context, id string) (*coretask.Task, error) {
	if m.tasksByID != nil {
		return m.tasksByID[id], nil
	}
	return m.getTask, nil
}

func (m *mockUpdateStore) Update(_ context.Context, id string, updates coretask.Updates) (*coretask.Task, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	m.updateCalled = true
	t := &coretask.Task{ID: id}
	// Start from existing task data if available.
	base := m.getTask
	if m.tasksByID != nil {
		base = m.tasksByID[id]
	}
	if base != nil {
		t.Subject = base.Subject
		t.Status = base.Status
		t.Blocks = make([]string, len(base.Blocks))
		copy(t.Blocks, base.Blocks)
		t.BlockedBy = make([]string, len(base.BlockedBy))
		copy(t.BlockedBy, base.BlockedBy)
		if base.Metadata != nil {
			t.Metadata = make(map[string]any)
			for k, v := range base.Metadata {
				t.Metadata[k] = v
			}
		}
	}
	if updates.Subject != nil {
		t.Subject = *updates.Subject
	}
	if updates.Status != nil {
		t.Status = *updates.Status
	}
	if updates.Blocks != nil {
		t.Blocks = *updates.Blocks
	}
	if updates.BlockedBy != nil {
		t.BlockedBy = *updates.BlockedBy
	}
	if updates.Metadata != nil {
		if t.Metadata == nil {
			t.Metadata = make(map[string]any)
		}
		for k, v := range updates.Metadata {
			if v == nil {
				delete(t.Metadata, k)
			} else {
				t.Metadata[k] = v
			}
		}
	}
	// Sync back to tasksByID so List reflects updates.
	if m.tasksByID != nil {
		m.tasksByID[id] = t
	}
	m.updated = t
	return t, nil
}

func (m *mockUpdateStore) UpdateWithDependencies(_ context.Context, id string, updates coretask.Updates, addBlocks []string, addBlockedBy []string) (*coretask.Task, error) {
	m.updateWithDependenciesCalled = true
	m.lastDependencyTaskID = id
	m.lastDependencyUpdates = updates
	m.lastAddBlocks = append([]string(nil), addBlocks...)
	m.lastAddBlockedBy = append([]string(nil), addBlockedBy...)
	return m.Update(context.Background(), id, updates)
}

func (m *mockUpdateStore) Delete(_ context.Context, _ string) (bool, error) {
	return m.deleted, m.deleteErr
}

func (m *mockUpdateStore) List(_ context.Context) ([]*coretask.Task, error) {
	var tasks []*coretask.Task
	if m.tasksByID != nil {
		for _, t := range m.tasksByID {
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}

func TestUpdateTool_StatusChange(t *testing.T) {
	store := &mockUpdateStore{
		getTask: &coretask.Task{
			ID:      "1",
			Subject: "Old",
			Status:  coretask.StatusPending,
		},
	}
	tool := NewTool(store)

	newStatus := "in_progress"
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId": "1",
			"status": newStatus,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	data := result.Meta["data"].(Output)
	if data.StatusChange == nil {
		t.Fatal("StatusChange should not be nil")
	}
	if data.StatusChange.From != "pending" {
		t.Errorf("From = %q, want %q", data.StatusChange.From, "pending")
	}
	if data.StatusChange.To != "in_progress" {
		t.Errorf("To = %q, want %q", data.StatusChange.To, "in_progress")
	}
}

func TestUpdateTool_DeleteStatus(t *testing.T) {
	store := &mockUpdateStore{
		getTask: &coretask.Task{
			ID:     "1",
			Status: coretask.StatusPending,
		},
		deleted: true,
	}
	tool := NewTool(store)

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId": "1",
			"status": "deleted",
		},
	})
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	data := result.Meta["data"].(Output)
	if !data.Success {
		t.Error("Success should be true")
	}
	if data.StatusChange.To != "deleted" {
		t.Errorf("StatusChange.To = %q, want %q", data.StatusChange.To, "deleted")
	}
}

func TestUpdateTool_NotFound(t *testing.T) {
	store := &mockUpdateStore{getTask: nil}
	tool := NewTool(store)

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId": "999",
			"status": "in_progress",
		},
	})
	data := result.Meta["data"].(Output)
	if data.Success {
		t.Error("Should not succeed for nonexistent task")
	}
	if data.Error != "Task not found" {
		t.Errorf("Error = %q, want %q", data.Error, "Task not found")
	}
}

func TestUpdateTool_AddBlocks(t *testing.T) {
	store := &mockUpdateStore{
		tasksByID: map[string]*coretask.Task{
			"1": {
				ID:      "1",
				Subject: "Task",
				Status:  coretask.StatusPending,
				Blocks:  []string{},
			},
			"2": {ID: "2", Subject: "Task 2", Status: coretask.StatusPending},
			"3": {ID: "3", Subject: "Task 3", Status: coretask.StatusPending},
		},
	}
	tool := NewTool(store)

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId":    "1",
			"addBlocks": []any{"2", "3"},
		},
	})
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if !store.updateCalled {
		t.Fatal("UpdateWithDependencies should have been called")
	}
	if store.updated == nil {
		t.Fatal("updated should not be nil")
	}
	if len(store.updated.Blocks) != 2 {
		t.Fatalf("Blocks = %v, want 2 entries", store.updated.Blocks)
	}
	if !store.updateWithDependenciesCalled {
		t.Fatal("UpdateWithDependencies should have been called")
	}
	if store.lastDependencyTaskID != "1" {
		t.Fatalf("dependency task ID = %q, want %q", store.lastDependencyTaskID, "1")
	}
	if len(store.lastAddBlocks) != 2 || store.lastAddBlocks[0] != "2" || store.lastAddBlocks[1] != "3" {
		t.Fatalf("addBlocks = %v, want [2 3]", store.lastAddBlocks)
	}
}

func TestUpdateTool_AddBlocksMissingReferencedTask(t *testing.T) {
	store := &mockUpdateStore{
		tasksByID: map[string]*coretask.Task{
			"1": {
				ID:      "1",
				Subject: "Task",
				Status:  coretask.StatusPending,
				Blocks:  []string{},
			},
		},
	}
	tool := NewTool(store)

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId":    "1",
			"addBlocks": []any{"999"},
		},
	})
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	data := result.Meta["data"].(Output)
	if data.Success {
		t.Fatal("Success should be false when referenced blocked task does not exist")
	}
	if data.Error == "" {
		t.Fatal("Error should be set when referenced blocked task does not exist")
	}
	if store.updateCalled {
		t.Fatal("UpdateWithDependencies should not be called when a referenced blocked task does not exist")
	}
}

func TestUpdateTool_AddBlockedByMissingReferencedTask(t *testing.T) {
	store := &mockUpdateStore{
		tasksByID: map[string]*coretask.Task{
			"1": {
				ID:        "1",
				Subject:   "Task",
				Status:    coretask.StatusPending,
				BlockedBy: []string{},
			},
		},
	}
	tool := NewTool(store)

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId":       "1",
			"addBlockedBy": []any{"999"},
		},
	})
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	data := result.Meta["data"].(Output)
	if data.Success {
		t.Fatal("Success should be false when referenced blocker task does not exist")
	}
	if data.Error == "" {
		t.Fatal("Error should be set when referenced blocker task does not exist")
	}
	if store.updateCalled {
		t.Fatal("UpdateWithDependencies should not be called when a referenced blocker task does not exist")
	}
}

func TestUpdateTool_PreValidatesBlocksBeforeBasicFieldUpdate(t *testing.T) {
	store := &mockUpdateStore{
		tasksByID: map[string]*coretask.Task{
			"1": {
				ID:      "1",
				Subject: "Old",
				Status:  coretask.StatusPending,
				Blocks:  []string{},
			},
		},
	}
	tool := NewTool(store)

	newSubject := "New"
	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId":    "1",
			"subject":   newSubject,
			"addBlocks": []any{"999"},
		},
	})
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	data := result.Meta["data"].(Output)
	if data.Success {
		t.Fatal("Success should be false when referenced blocked task does not exist")
	}
	if store.updateCalled {
		t.Fatal("Update should not be called before block target pre-validation succeeds")
	}
	if store.updated != nil {
		t.Fatal("updated should remain nil when pre-validation fails")
	}
	if store.updateWithDependenciesCalled {
		t.Fatal("UpdateWithDependencies should not be called when pre-validation fails")
	}
}

func TestUpdateTool_AddBlockedBy(t *testing.T) {
	store := &mockUpdateStore{
		tasksByID: map[string]*coretask.Task{
			"1": {
				ID:        "1",
				Subject:   "Task",
				Status:    coretask.StatusPending,
				BlockedBy: []string{},
			},
			"2": {ID: "2", Subject: "Task 2", Status: coretask.StatusPending},
			"3": {ID: "3", Subject: "Task 3", Status: coretask.StatusPending},
		},
	}
	tool := NewTool(store)

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId":       "1",
			"addBlockedBy": []any{"2", "3"},
		},
	})
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if !store.updateCalled {
		t.Fatal("UpdateWithDependencies should have been called")
	}
	if store.updated == nil {
		t.Fatal("updated should not be nil")
	}
	if len(store.updated.BlockedBy) != 2 {
		t.Fatalf("BlockedBy = %v, want 2 entries", store.updated.BlockedBy)
	}
	if !store.updateWithDependenciesCalled {
		t.Fatal("UpdateWithDependencies should have been called")
	}
	if len(store.lastAddBlockedBy) != 2 || store.lastAddBlockedBy[0] != "2" || store.lastAddBlockedBy[1] != "3" {
		t.Fatalf("addBlockedBy = %v, want [2 3]", store.lastAddBlockedBy)
	}
}

func TestUpdateTool_CombinesFieldsAndBlocksInSingleUpdate(t *testing.T) {
	store := &mockUpdateStore{
		tasksByID: map[string]*coretask.Task{
			"1": {
				ID:        "1",
				Subject:   "Old",
				Status:    coretask.StatusPending,
				Blocks:    []string{},
				BlockedBy: []string{},
			},
			"2": {ID: "2", Subject: "Task 2", Status: coretask.StatusPending},
		},
	}
	tool := NewTool(store)

	newSubject := "New"
	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId":    "1",
			"subject":   newSubject,
			"addBlocks": []any{"2"},
		},
	})
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if !store.updateCalled {
		t.Fatal("UpdateWithDependencies should have been called")
	}
	if store.updated == nil {
		t.Fatal("updated should not be nil")
	}
	if store.updated.Subject != "New" {
		t.Errorf("Subject = %q, want %q", store.updated.Subject, "New")
	}
	if len(store.updated.Blocks) != 1 || store.updated.Blocks[0] != "2" {
		t.Errorf("Blocks = %v, want [2]", store.updated.Blocks)
	}
	if !store.updateWithDependenciesCalled {
		t.Fatal("UpdateWithDependencies should have been called")
	}
	if len(store.lastAddBlocks) != 1 || store.lastAddBlocks[0] != "2" {
		t.Fatalf("addBlocks = %v, want [2]", store.lastAddBlocks)
	}
}

func TestUpdateTool_MetadataNullDelete(t *testing.T) {
	store := &mockUpdateStore{
		getTask: &coretask.Task{
			ID:      "1",
			Subject: "Task",
			Status:  coretask.StatusPending,
			Metadata: map[string]any{
				"keep":   "yes",
				"remove": "no",
			},
		},
	}
	tool := NewTool(store)

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId": "1",
			"metadata": map[string]any{
				"remove": nil,
				"add":    "new",
			},
		},
	})
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if store.updated == nil {
		t.Fatal("updated should not be nil")
	}
	if store.updated.Metadata["keep"] != "yes" {
		t.Error("metadata[keep] should still be 'yes'")
	}
	if store.updated.Metadata["add"] != "new" {
		t.Error("metadata[add] should be 'new'")
	}
	if _, exists := store.updated.Metadata["remove"]; exists {
		t.Error("metadata[remove] should be deleted")
	}
}

func TestUpdateTool_MissingTaskID(t *testing.T) {
	store := &mockUpdateStore{}
	tool := NewTool(store)

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{},
	})
	if result.Error == "" {
		t.Fatal("Expected error for missing taskId")
	}
}

func TestUpdateTool_NilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{})
	if err == nil {
		t.Fatal("Expected error for nil receiver")
	}
}

// mockCompletedHookDispatcher returns blocking results for TaskCompleted events.
type mockCompletedHookDispatcher struct {
	called bool
	events []hook.HookEvent
}

func (m *mockCompletedHookDispatcher) RunHooks(_ context.Context, event hook.HookEvent, _ any, _ string) []hook.HookResult {
	m.called = true
	m.events = append(m.events, event)
	return []hook.HookResult{{ExitCode: 2, Stderr: "task completion blocked by policy"}}
}

func TestUpdateTool_TaskCompletedHookBlocking(t *testing.T) {
	completed := "completed"
	store := &mockUpdateStore{
		getTask: &coretask.Task{
			ID:      "1",
			Subject: "Test task",
			Status:  coretask.StatusInProgress,
		},
	}
	dispatcher := &mockCompletedHookDispatcher{}
	hookCfg := hook.HooksConfig{
		hook.EventTaskCompleted: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo block"}`)},
		}},
	}
	tool := NewToolWithHooks(store, dispatcher, hookCfg, false)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId": "1",
			"status": completed,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "task completion blocked by policy") {
		t.Fatalf("Error = %q, want to contain blocking message", result.Error)
	}
	if result.Output != "" {
		t.Fatalf("Output = %q, want empty on blocked completion", result.Output)
	}
	if !dispatcher.called {
		t.Error("HookDispatcher.RunHooks was not called")
	}
	// The store should NOT have been updated (blocking prevents status change).
	if store.updateWithDependenciesCalled {
		t.Error("store.UpdateWithDependencies was called, but the update should have been blocked")
	}
}

func TestUpdateTool_TaskCompletedHookDisabledByPolicy(t *testing.T) {
	completed := "completed"
	store := &mockUpdateStore{
		getTask: &coretask.Task{
			ID:      "1",
			Subject: "Test task",
			Status:  coretask.StatusInProgress,
		},
	}
	dispatcher := &mockCompletedHookDispatcher{}
	hookCfg := hook.HooksConfig{
		hook.EventTaskCompleted: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo block"}`)},
		}},
	}
	tool := NewToolWithHooks(store, dispatcher, hookCfg, true)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId": "1",
			"status": completed,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q, want empty", result.Error)
	}
	if dispatcher.called {
		t.Fatal("HookDispatcher.RunHooks was called, want hooks disabled")
	}
	if !store.updateWithDependenciesCalled {
		t.Fatal("store.UpdateWithDependencies was not called, want task update to proceed")
	}
}

func TestUpdateTool_TaskCompletedHookNotTriggeredForOtherStatus(t *testing.T) {
	inProgress := "in_progress"
	store := &mockUpdateStore{
		getTask: &coretask.Task{
			ID:      "1",
			Subject: "Test task",
			Status:  coretask.StatusPending,
		},
	}
	dispatcher := &mockCompletedHookDispatcher{}
	hookCfg := hook.HooksConfig{
		hook.EventTaskCompleted: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo block"}`)},
		}},
	}
	tool := NewToolWithHooks(store, dispatcher, hookCfg, false)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId": "1",
			"status": inProgress,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Unexpected error: %q", result.Error)
	}
	// TaskCompleted hooks should NOT have been called for a non-completed status change.
	if dispatcher.called {
		t.Error("HookDispatcher.RunHooks should not be called for in_progress status")
	}
}

func TestUpdateTool_VerificationNudge_Triggers(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_VERIFICATION_AGENT", "1")
	store := &mockUpdateStore{
		getTask: &coretask.Task{
			ID:      "3",
			Subject: "Deploy app",
			Status:  coretask.StatusInProgress,
		},
		tasksByID: map[string]*coretask.Task{
			"1": {ID: "1", Subject: "Write tests", Status: coretask.StatusCompleted},
			"2": {ID: "2", Subject: "Refactor code", Status: coretask.StatusCompleted},
			"3": {ID: "3", Subject: "Deploy app", Status: coretask.StatusInProgress},
		},
	}
	tool := NewTool(store)

	completed := "completed"
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId": "3",
			"status": completed,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Unexpected error: %q", result.Error)
	}
	if !strings.Contains(result.Output, "verification agent") {
		t.Errorf("Output should contain verification nudge, got: %q", result.Output)
	}
	data := result.Meta["data"].(Output)
	if !data.VerificationNudgeNeeded {
		t.Error("VerificationNudgeNeeded should be true")
	}
}

func TestUpdateTool_VerificationNudge_NotEnoughTasks(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_VERIFICATION_AGENT", "1")
	store := &mockUpdateStore{
		getTask: &coretask.Task{
			ID:      "2",
			Subject: "Deploy app",
			Status:  coretask.StatusInProgress,
		},
		tasksByID: map[string]*coretask.Task{
			"1": {ID: "1", Subject: "Write tests", Status: coretask.StatusCompleted},
			"2": {ID: "2", Subject: "Deploy app", Status: coretask.StatusInProgress},
		},
	}
	tool := NewTool(store)

	completed := "completed"
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId": "2",
			"status": completed,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if strings.Contains(result.Output, "verification agent") {
		t.Errorf("Output should NOT contain verification nudge for < 3 tasks, got: %q", result.Output)
	}
	data := result.Meta["data"].(Output)
	if data.VerificationNudgeNeeded {
		t.Error("VerificationNudgeNeeded should be false")
	}
}

func TestUpdateTool_VerificationNudge_HasVerificationTask(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_VERIFICATION_AGENT", "1")
	store := &mockUpdateStore{
		getTask: &coretask.Task{
			ID:      "3",
			Subject: "Deploy app",
			Status:  coretask.StatusInProgress,
		},
		tasksByID: map[string]*coretask.Task{
			"1": {ID: "1", Subject: "Write tests", Status: coretask.StatusCompleted},
			"2": {ID: "2", Subject: "Verify deployment", Status: coretask.StatusCompleted},
			"3": {ID: "3", Subject: "Deploy app", Status: coretask.StatusInProgress},
		},
	}
	tool := NewTool(store)

	completed := "completed"
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId": "3",
			"status": completed,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if strings.Contains(result.Output, "verification agent") {
		t.Errorf("Output should NOT contain verification nudge when verification task exists, got: %q", result.Output)
	}
	data := result.Meta["data"].(Output)
	if data.VerificationNudgeNeeded {
		t.Error("VerificationNudgeNeeded should be false")
	}
}

func TestUpdateTool_VerificationNudge_FeatureDisabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_VERIFICATION_AGENT", "0")
	store := &mockUpdateStore{
		getTask: &coretask.Task{
			ID:      "3",
			Subject: "Deploy app",
			Status:  coretask.StatusInProgress,
		},
		tasksByID: map[string]*coretask.Task{
			"1": {ID: "1", Subject: "Write tests", Status: coretask.StatusCompleted},
			"2": {ID: "2", Subject: "Refactor code", Status: coretask.StatusCompleted},
			"3": {ID: "3", Subject: "Deploy app", Status: coretask.StatusInProgress},
		},
	}
	tool := NewTool(store)

	completed := "completed"
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId": "3",
			"status": completed,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if strings.Contains(result.Output, "verification agent") {
		t.Errorf("Output should NOT contain verification nudge when feature disabled, got: %q", result.Output)
	}
	data := result.Meta["data"].(Output)
	if data.VerificationNudgeNeeded {
		t.Error("VerificationNudgeNeeded should be false")
	}
}

func TestUpdateTool_VerificationNudge_NotAllDone(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_VERIFICATION_AGENT", "1")
	store := &mockUpdateStore{
		getTask: &coretask.Task{
			ID:      "3",
			Subject: "Deploy app",
			Status:  coretask.StatusInProgress,
		},
		tasksByID: map[string]*coretask.Task{
			"1": {ID: "1", Subject: "Write tests", Status: coretask.StatusCompleted},
			"2": {ID: "2", Subject: "Refactor code", Status: coretask.StatusPending},
			"3": {ID: "3", Subject: "Deploy app", Status: coretask.StatusInProgress},
		},
	}
	tool := NewTool(store)

	completed := "completed"
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId": "3",
			"status": completed,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if strings.Contains(result.Output, "verification agent") {
		t.Errorf("Output should NOT contain verification nudge when not all done, got: %q", result.Output)
	}
	data := result.Meta["data"].(Output)
	if data.VerificationNudgeNeeded {
		t.Error("VerificationNudgeNeeded should be false")
	}
}

func TestUpdateTool_VerificationNudge_NonCompletedStatus(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_VERIFICATION_AGENT", "1")
	store := &mockUpdateStore{
		getTask: &coretask.Task{
			ID:      "1",
			Subject: "Write tests",
			Status:  coretask.StatusPending,
		},
		tasksByID: map[string]*coretask.Task{
			"1": {ID: "1", Subject: "Write tests", Status: coretask.StatusPending},
			"2": {ID: "2", Subject: "Refactor code", Status: coretask.StatusCompleted},
			"3": {ID: "3", Subject: "Deploy app", Status: coretask.StatusCompleted},
		},
	}
	tool := NewTool(store)

	inProgress := "in_progress"
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"taskId": "1",
			"status": inProgress,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if strings.Contains(result.Output, "verification agent") {
		t.Errorf("Output should NOT contain verification nudge for non-completed status, got: %q", result.Output)
	}
	data := result.Meta["data"].(Output)
	if data.VerificationNudgeNeeded {
		t.Error("VerificationNudgeNeeded should be false")
	}
}
