package task_get

import (
	"context"
	"testing"

	coretask "github.com/sheepzhao/claude-code-go/internal/core/task"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// mockGetStore implements TaskGetter for testing.
type mockGetStore struct {
	task *coretask.Task
	err  error
}

func (m *mockGetStore) Get(_ context.Context, _ string) (*coretask.Task, error) {
	return m.task, m.err
}

func TestGetTool_Found(t *testing.T) {
	store := &mockGetStore{task: &coretask.Task{
		ID:          "1",
		Subject:     "Test",
		Description: "Desc",
		Status:      coretask.StatusPending,
		Blocks:      []string{"2"},
		BlockedBy:   []string{"3"},
	}}
	tool := NewTool(store)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"taskId": "1"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
}

func TestGetTool_NotFound(t *testing.T) {
	store := &mockGetStore{task: nil}
	tool := NewTool(store)

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"taskId": "999"},
	})
	if result.Error != "" {
		t.Fatalf("result.Error should be empty for not found, got %q", result.Error)
	}
	if result.Output != "Task not found" {
		t.Errorf("Output = %q, want %q", result.Output, "Task not found")
	}
}

func TestGetTool_MissingTaskID(t *testing.T) {
	store := &mockGetStore{}
	tool := NewTool(store)

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{},
	})
	if result.Error == "" {
		t.Fatal("Expected error for missing taskId")
	}
}

func TestGetTool_EmptyTaskID(t *testing.T) {
	store := &mockGetStore{}
	tool := NewTool(store)

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"taskId": "  "},
	})
	if result.Error == "" {
		t.Fatal("Expected error for empty taskId")
	}
}

func TestGetTool_NilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{})
	if err == nil {
		t.Fatal("Expected error for nil receiver")
	}
}
