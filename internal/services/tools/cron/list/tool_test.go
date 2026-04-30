package list

import (
	"context"
	"strings"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	cronshared "github.com/sheepzhao/claude-code-go/internal/services/tools/cron/shared"
)

func newTestTool(t *testing.T) *Tool {
	t.Helper()
	return NewTool(cronshared.NewStore(t.TempDir()))
}

func TestName(t *testing.T) {
	tool := newTestTool(t)
	if tool.Name() != Name {
		t.Errorf("expected Name %q, got %q", Name, tool.Name())
	}
}

func TestDescription(t *testing.T) {
	tool := newTestTool(t)
	desc := tool.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestIsReadOnly(t *testing.T) {
	tool := newTestTool(t)
	if !tool.IsReadOnly() {
		t.Error("expected IsReadOnly to return true")
	}
}

func TestIsConcurrencySafe(t *testing.T) {
	tool := newTestTool(t)
	if !tool.IsConcurrencySafe() {
		t.Error("expected IsConcurrencySafe to return true")
	}
}

func TestInputSchema(t *testing.T) {
	tool := newTestTool(t)
	schema := tool.InputSchema()
	if schema.Properties == nil {
		t.Error("expected non-nil Properties map")
	}
	if len(schema.Properties) != 0 {
		t.Errorf("expected empty Properties map, got %d entries", len(schema.Properties))
	}
}

func TestInvokeEmptyList(t *testing.T) {
	tool := newTestTool(t)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	// Empty list should return "No scheduled jobs."
	if !strings.Contains(result.Output, "No scheduled jobs") {
		t.Errorf("expected 'No scheduled jobs', got %q", result.Output)
	}

	data, ok := result.Meta["data"]
	if !ok {
		t.Fatal("expected Meta to contain 'data' key")
	}
	output, ok := data.(Output)
	if !ok {
		t.Fatalf("expected Meta data to be of type Output, got %T", data)
	}
	if len(output.Jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(output.Jobs))
	}
}

func TestInvokeWithJobs(t *testing.T) {
	store := cronshared.NewStore(t.TempDir())
	store.Create("*/5 * * * *", "task one", true, false)
	store.Create("0 9 * * 1-5", "task two", false, false)

	tool := NewTool(store)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{},
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

	data, ok := result.Meta["data"]
	if !ok {
		t.Fatal("expected Meta to contain 'data' key")
	}
	output, ok := data.(Output)
	if !ok {
		t.Fatalf("expected Meta data to be of type Output, got %T", data)
	}
	if len(output.Jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(output.Jobs))
	}
}

func TestInvokeNilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{},
	})
	if err == nil {
		t.Error("expected error for nil receiver")
	}
}
