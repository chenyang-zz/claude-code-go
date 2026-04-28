package delete

import (
	"context"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	cronshared "github.com/sheepzhao/claude-code-go/internal/services/tools/cron/shared"
)

func newTestTool() *Tool {
	return NewTool(cronshared.NewStore())
}

func TestName(t *testing.T) {
	tool := newTestTool()
	if tool.Name() != Name {
		t.Errorf("expected Name %q, got %q", Name, tool.Name())
	}
}

func TestDescription(t *testing.T) {
	tool := newTestTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestIsReadOnly(t *testing.T) {
	tool := newTestTool()
	if tool.IsReadOnly() {
		t.Error("expected IsReadOnly to return false")
	}
}

func TestIsConcurrencySafe(t *testing.T) {
	tool := newTestTool()
	if !tool.IsConcurrencySafe() {
		t.Error("expected IsConcurrencySafe to return true")
	}
}

func TestRequiresUserInteraction(t *testing.T) {
	tool := newTestTool()
	if !tool.RequiresUserInteraction() {
		t.Error("expected RequiresUserInteraction to return true")
	}
}

func TestInputSchema(t *testing.T) {
	tool := newTestTool()
	schema := tool.InputSchema()

	prop, ok := schema.Properties["id"]
	if !ok {
		t.Error("expected 'id' property in input schema")
	} else {
		if prop.Type != coretool.ValueKindString {
			t.Errorf("expected 'id' type to be string, got %s", prop.Type)
		}
		if !prop.Required {
			t.Error("expected 'id' to be required")
		}
	}
}

func TestInvokeDeleteExist(t *testing.T) {
	store := cronshared.NewStore()
	task, _ := store.Create("*/5 * * * *", "test", true, false)

	tool := NewTool(store)
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"id": task.ID,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}

	// Verify the task is actually deleted.
	if store.Exists(task.ID) {
		t.Error("expected task to be deleted after Invoke")
	}
}

func TestInvokeDeleteNotFound(t *testing.T) {
	tool := newTestTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"id": "nonexistent",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected result error for nonexistent ID")
	}
}

func TestInvokeNilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"id": "test-id",
		},
	})
	if err == nil {
		t.Error("expected error for nil receiver")
	}
}
