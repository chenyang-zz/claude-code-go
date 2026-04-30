package todo_write

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/task"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// stubStore implements task.Store with a fixed task list for testing.
type stubStore struct {
	tasks []*task.Task
	err   error
}

func (s *stubStore) Create(ctx context.Context, data task.NewTask) (string, error) { return "", nil }
func (s *stubStore) Get(ctx context.Context, id string) (*task.Task, error)        { return nil, nil }
func (s *stubStore) List(ctx context.Context) ([]*task.Task, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.tasks, nil
}
func (s *stubStore) Update(ctx context.Context, id string, updates task.Updates) (*task.Task, error) {
	return nil, nil
}
func (s *stubStore) UpdateWithDependencies(ctx context.Context, taskID string, updates task.Updates, addBlocks []string, addBlockedBy []string) (*task.Task, error) {
	return nil, nil
}
func (s *stubStore) Delete(ctx context.Context, id string) (bool, error)              { return false, nil }
func (s *stubStore) BlockTask(ctx context.Context, fromID, toID string) (bool, error) { return false, nil }
func (s *stubStore) ClaimTask(ctx context.Context, id, claimantAgentID string, opts task.ClaimTaskOptions) (*task.ClaimTaskResult, error) {
	return nil, nil
}
func (s *stubStore) ResetTaskList(ctx context.Context) error { return nil }
func (s *stubStore) UnassignTeammateTasks(ctx context.Context, teammateID string) (*task.UnassignResult, error) {
	return nil, nil
}

func TestName(t *testing.T) {
	tw := NewTool(&stubStore{})
	if tw.Name() != Name {
		t.Fatalf("Name() = %q, want %q", tw.Name(), Name)
	}
}

func TestDescription(t *testing.T) {
	tw := NewTool(&stubStore{})
	if tw.Description() == "" {
		t.Fatal("Description() returned empty string")
	}
	if !strings.Contains(tw.Description(), "TodoV2") {
		t.Fatal("Description() should mention TodoV2 tools")
	}
}

func TestInputSchema(t *testing.T) {
	tw := NewTool(&stubStore{})
	schema := tw.InputSchema()

	todos, ok := schema.Properties["todos"]
	if !ok {
		t.Fatal("InputSchema missing 'todos' property")
	}
	if todos.Type != tool.ValueKindArray || !todos.Required {
		t.Fatalf("todos field: type=%s required=%v, want array required=true", todos.Type, todos.Required)
	}
}

func TestIsReadOnly(t *testing.T) {
	tw := NewTool(&stubStore{})
	if tw.IsReadOnly() {
		t.Fatal("IsReadOnly() = true, want false")
	}
}

func TestIsConcurrencySafe(t *testing.T) {
	tw := NewTool(&stubStore{})
	if !tw.IsConcurrencySafe() {
		t.Fatal("IsConcurrencySafe() = false, want true")
	}
}

func TestInvoke_WithTasks(t *testing.T) {
	store := &stubStore{
		tasks: []*task.Task{
			{ID: "1", Subject: "Fix bug", Status: task.StatusInProgress, ActiveForm: "Fixing bug"},
			{ID: "2", Subject: "Add feature", Status: task.StatusPending, ActiveForm: "Adding feature"},
		},
	}
	tw := NewTool(store)

	result, err := tw.Invoke(context.Background(), tool.Call{
		Input: map[string]any{
			"todos": []any{
				map[string]any{"content": "Fix bug", "status": "in_progress", "activeForm": "Fixing bug"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error = %q, want empty", result.Error)
	}

	var output todoWriteOutput
	if err := json.Unmarshal([]byte(result.Output), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if len(output.OldTodos) != 2 {
		t.Fatalf("oldTodos length = %d, want 2", len(output.OldTodos))
	}
	if output.OldTodos[0].Content != "Fix bug" {
		t.Fatalf("oldTodos[0].Content = %q, want 'Fix bug'", output.OldTodos[0].Content)
	}
	if output.OldTodos[0].Status != "in_progress" {
		t.Fatalf("oldTodos[0].Status = %q, want 'in_progress'", output.OldTodos[0].Status)
	}
	if len(output.NewTodos) != 1 {
		t.Fatalf("newTodos length = %d, want 1", len(output.NewTodos))
	}
	if !strings.Contains(output.Message, "TodoV2") {
		t.Fatal("output.Message should mention TodoV2")
	}
}

func TestInvoke_EmptyStore(t *testing.T) {
	store := &stubStore{}
	tw := NewTool(store)

	result, err := tw.Invoke(context.Background(), tool.Call{
		Input: map[string]any{
			"todos": []any{},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error = %q, want empty", result.Error)
	}

	var output todoWriteOutput
	if err := json.Unmarshal([]byte(result.Output), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if len(output.OldTodos) != 0 {
		t.Fatalf("oldTodos length = %d, want 0", len(output.OldTodos))
	}
}

func TestInvoke_InvalidSchema(t *testing.T) {
	store := &stubStore{}
	tw := NewTool(store)

	result, err := tw.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"unknown": "field"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "missing required field") {
		t.Fatalf("Invoke() error = %q, want missing required field", result.Error)
	}
}

func TestInvoke_MissingTodos(t *testing.T) {
	store := &stubStore{}
	tw := NewTool(store)

	result, err := tw.Invoke(context.Background(), tool.Call{
		Input: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "missing required field") {
		t.Fatalf("Invoke() error = %q, want missing required field", result.Error)
	}
}
