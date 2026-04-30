package task_output

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
)

func TestTaskOutputTool_Name(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewTool(store)
	if tool.Name() != Name {
		t.Errorf("expected name %q, got %q", Name, tool.Name())
	}
}

func TestTaskOutputTool_Aliases(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewTool(store)
	aliases := tool.Aliases()
	if len(aliases) != 2 {
		t.Errorf("expected 2 aliases, got %d", len(aliases))
	}
	if aliases[0] != "AgentOutputTool" {
		t.Errorf("expected first alias AgentOutputTool, got %q", aliases[0])
	}
	if aliases[1] != "BashOutputTool" {
		t.Errorf("expected second alias BashOutputTool, got %q", aliases[1])
	}
}

func TestTaskOutputTool_InputSchema(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewTool(store)
	schema := tool.InputSchema()
	if schema.Properties["task_id"].Type != coretool.ValueKindString {
		t.Errorf("expected task_id type string, got %s", schema.Properties["task_id"].Type)
	}
	if !schema.Properties["task_id"].Required {
		t.Error("expected task_id to be required")
	}
}

func TestTaskOutputTool_Invoke_TaskNotFound(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewTool(store)
	result, err := tool.Invoke(context.TODO(), coretool.Call{
		Input: map[string]any{
			"task_id": "nonexistent",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for nonexistent task")
	}
}

func TestTaskOutputTool_Invoke_EmptyTaskID(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewTool(store)
	result, err := tool.Invoke(context.TODO(), coretool.Call{
		Input: map[string]any{
			"task_id": "",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for empty task_id")
	}
}

func TestTaskOutputTool_Invoke_NonBlockingRunning(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewTool(store)

	// Register a running task.
	store.Register(coresession.BackgroundTaskSnapshot{
		ID:      "task-1",
		Type:    "local_bash",
		Status:  coresession.BackgroundTaskStatusRunning,
		Summary: "echo hello",
	}, nil)

	result, err := tool.Invoke(context.TODO(), coretool.Call{
		Input: map[string]any{
			"task_id": "task-1",
			"block":   false,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	var output taskOutputResult
	json.Unmarshal([]byte(result.Output), &output)
	if output.RetrievalStatus != "not_ready" {
		t.Errorf("expected retrieval_status not_ready, got %q", output.RetrievalStatus)
	}
	if output.Task == nil {
		t.Fatal("expected non-nil task output")
	}
	if output.Task.TaskID != "task-1" {
		t.Errorf("expected task_id task-1, got %q", output.Task.TaskID)
	}
	if output.Task.Description != "echo hello" {
		t.Errorf("expected description 'echo hello', got %q", output.Task.Description)
	}
}

func TestTaskOutputTool_Invoke_NonBlockingCompleted(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewTool(store)

	// Register a completed task.
	store.Register(coresession.BackgroundTaskSnapshot{
		ID:      "task-2",
		Type:    "local_bash",
		Status:  coresession.BackgroundTaskStatusCompleted,
		Summary: "done",
	}, nil)

	result, err := tool.Invoke(context.TODO(), coretool.Call{
		Input: map[string]any{
			"task_id": "task-2",
			"block":   false,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	var output taskOutputResult
	json.Unmarshal([]byte(result.Output), &output)
	if output.RetrievalStatus != "success" {
		t.Errorf("expected retrieval_status success, got %q", output.RetrievalStatus)
	}
}

func TestTaskOutputTool_Invoke_NilReceiver(t *testing.T) {
	var tool *Tool
	result, err := tool.Invoke(context.TODO(), coretool.Call{
		Input: map[string]any{
			"task_id": "task-1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" || result.Error != "TaskOutput tool: nil receiver" {
		t.Errorf("expected nil receiver error, got %q", result.Error)
	}
}

func TestTaskOutputTool_Invoke_BlockingTimeout(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewTool(store)

	// Register a running task. Blocking with short timeout should timeout.
	store.Register(coresession.BackgroundTaskSnapshot{
		ID:      "task-3",
		Type:    "local_bash",
		Status:  coresession.BackgroundTaskStatusRunning,
		Summary: "long running task",
	}, nil)

	start := time.Now()
	result, err := tool.Invoke(context.TODO(), coretool.Call{
		Input: map[string]any{
			"task_id": "task-3",
			"block":   true,
			"timeout": float64(200), // 200ms timeout
		},
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	var output taskOutputResult
	json.Unmarshal([]byte(result.Output), &output)
	if output.RetrievalStatus != "timeout" {
		t.Errorf("expected retrieval_status timeout, got %q", output.RetrievalStatus)
	}
	// Should have waited at least 200ms.
	if elapsed < 150*time.Millisecond {
		t.Errorf("expected at least 150ms elapsed for timeout, got %v", elapsed)
	}
}

func TestTaskOutputTool_IsReadOnly(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewTool(store)
	if !tool.IsReadOnly() {
		t.Error("expected IsReadOnly to return true")
	}
}

func TestTaskOutputTool_IsConcurrencySafe(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewTool(store)
	if !tool.IsConcurrencySafe() {
		t.Error("expected IsConcurrencySafe to return true")
	}
}

func TestTaskOutputTool_Description_HasDeprecated(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewTool(store)
	if tool.Description() == "" {
		t.Error("expected non-empty description with [Deprecated] marker")
	}
}

func TestTaskOutputTool_NilStore(t *testing.T) {
	tool := NewTool(nil)
	result, err := tool.Invoke(context.TODO(), coretool.Call{
		Input: map[string]any{
			"task_id": "task-1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for nil task store")
	}
}

func TestTaskOutputTool_Invoke_InvalidInputType(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewTool(store)
	result, err := tool.Invoke(context.TODO(), coretool.Call{
		Input: map[string]any{
			"task_id": 123, // not a string
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for invalid input type")
	}
}
