package task_list

import (
	"context"
	"testing"

	coretask "github.com/sheepzhao/claude-code-go/internal/core/task"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// mockListStore implements TaskLister for testing.
type mockListStore struct {
	tasks []*coretask.Task
	err   error
}

func (m *mockListStore) List(_ context.Context) ([]*coretask.Task, error) {
	return m.tasks, m.err
}

func TestListTool_Empty(t *testing.T) {
	store := &mockListStore{tasks: nil}
	tool := NewTool(store)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Output != "No tasks found" {
		t.Errorf("Output = %q, want %q", result.Output, "No tasks found")
	}
}

func TestListTool_WithTasks(t *testing.T) {
	store := &mockListStore{tasks: []*coretask.Task{
		{ID: "1", Subject: "Task 1", Status: coretask.StatusPending},
		{ID: "2", Subject: "Task 2", Status: coretask.StatusInProgress, Owner: "agent-1"},
	}}
	tool := NewTool(store)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
}

func TestListTool_FiltersInternal(t *testing.T) {
	store := &mockListStore{tasks: []*coretask.Task{
		{ID: "1", Subject: "Visible", Status: coretask.StatusPending},
		{ID: "2", Subject: "Internal", Status: coretask.StatusPending, Metadata: map[string]any{"_internal": true}},
	}}
	tool := NewTool(store)

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{},
	})
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	// Should only contain the visible task.
	data := result.Meta["data"].(Output)
	if len(data.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(data.Tasks))
	}
	if data.Tasks[0].Subject != "Visible" {
		t.Errorf("Subject = %q, want %q", data.Tasks[0].Subject, "Visible")
	}
}

func TestListTool_FiltersCompletedBlockedBy(t *testing.T) {
	store := &mockListStore{tasks: []*coretask.Task{
		{ID: "1", Subject: "Done", Status: coretask.StatusCompleted},
		{ID: "2", Subject: "Pending", Status: coretask.StatusPending, BlockedBy: []string{"1", "3"}},
		{ID: "3", Subject: "Also pending", Status: coretask.StatusInProgress},
	}}
	tool := NewTool(store)

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{},
	})
	data := result.Meta["data"].(Output)
	// Task 2's blockedBy should only contain "3" since "1" is completed.
	for _, task := range data.Tasks {
		if task.ID == "2" {
			if len(task.BlockedBy) != 1 || task.BlockedBy[0] != "3" {
				t.Errorf("BlockedBy = %v, want [%q]", task.BlockedBy, "3")
			}
		}
	}
}

func TestListTool_NilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{})
	if err == nil {
		t.Fatal("Expected error for nil receiver")
	}
}
