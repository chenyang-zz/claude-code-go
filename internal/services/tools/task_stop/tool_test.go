package task_stop

import (
	"context"
	"fmt"
	"testing"

	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

type stubTaskStore struct {
	snapshot coresession.BackgroundTaskSnapshot
	err      error
	stopped  []string
}

func (s *stubTaskStore) Stop(id string) (coresession.BackgroundTaskSnapshot, error) {
	s.stopped = append(s.stopped, id)
	if s.err != nil {
		return coresession.BackgroundTaskSnapshot{}, s.err
	}
	return s.snapshot, nil
}

// TestToolInvokeSuccess verifies the stop tool returns one stable success message for a stopped task.
func TestToolInvokeSuccess(t *testing.T) {
	tool := NewTool(&stubTaskStore{
		snapshot: coresession.BackgroundTaskSnapshot{
			ID:      "task-1",
			Type:    "bash",
			Summary: "npm run dev",
		},
	})

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"task_id": "task-1",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error output = %q, want empty", result.Error)
	}
	if result.Output != "Stopped background task: task-1 (npm run dev)" {
		t.Fatalf("Invoke() output = %q, want stable stop message", result.Output)
	}
}

// TestToolInvokeFailure verifies the stop tool surfaces store lookup failures as stable tool errors.
func TestToolInvokeFailure(t *testing.T) {
	tool := NewTool(&stubTaskStore{
		err: fmt.Errorf("no background task found with ID: task-404"),
	})

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"task_id": "task-404",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "no background task found with ID: task-404" {
		t.Fatalf("Invoke() error output = %q, want missing-task error", result.Error)
	}
}
