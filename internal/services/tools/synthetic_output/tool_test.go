package synthetic_output

import (
	"context"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func TestTool_Name(t *testing.T) {
	tool := NewTool()
	if tool.Name() != Name {
		t.Errorf("Name() = %q, want %q", tool.Name(), Name)
	}
}

func TestTool_Description(t *testing.T) {
	tool := NewTool()
	if tool.Description() != toolDescription {
		t.Errorf("Description() = %q, want %q", tool.Description(), toolDescription)
	}
}

func TestTool_InputSchema(t *testing.T) {
	tool := NewTool()
	schema := tool.InputSchema()
	if len(schema.Properties) != 0 {
		t.Errorf("InputSchema should have empty Properties, got %d", len(schema.Properties))
	}
}

func TestTool_IsReadOnly(t *testing.T) {
	tool := NewTool()
	if !tool.IsReadOnly() {
		t.Error("IsReadOnly() should be true")
	}
}

func TestTool_IsConcurrencySafe(t *testing.T) {
	tool := NewTool()
	if !tool.IsConcurrencySafe() {
		t.Error("IsConcurrencySafe() should be true")
	}
}

func TestTool_Invoke_NilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{})
	if err == nil {
		t.Error("expected error for nil receiver")
	}
}

func TestTool_Invoke_EmptyInput(t *testing.T) {
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{Input: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("result output should not be empty")
	}
	if result.Meta == nil {
		t.Error("result meta should not be nil")
	}
	data, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatalf("meta.data is not Output type, got %T", result.Meta["data"])
	}
	if !data.Success {
		t.Error("data.Success should be true")
	}
	if data.Output != "Structured output provided successfully" {
		t.Errorf("data.Output = %q", data.Output)
	}
}

func TestTool_Invoke_WithInput(t *testing.T) {
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"summary": "Task completed successfully",
			"status":  "done",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatalf("meta.data is not Output type, got %T", result.Meta["data"])
	}
	if data.StructuredOutput == nil {
		t.Error("structured_output should not be nil")
	}
	if data.StructuredOutput["summary"] != "Task completed successfully" {
		t.Errorf("structured_output.summary = %v", data.StructuredOutput["summary"])
	}
	if data.StructuredOutput["status"] != "done" {
		t.Errorf("structured_output.status = %v", data.StructuredOutput["status"])
	}
}

func TestTool_Invoke_NilInput(t *testing.T) {
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{Input: nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	if data.StructuredOutput == nil {
		t.Error("structured_output should not be nil for nil input")
	}
}

func TestTool_Invoke_NestedInput(t *testing.T) {
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"users": []any{
				map[string]any{"name": "Alice", "age": float64(30)},
				map[string]any{"name": "Bob", "age": float64(25)},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	users, ok := data.StructuredOutput["users"].([]any)
	if !ok {
		t.Fatalf("structured_output.users is not []any, got %T", data.StructuredOutput["users"])
	}
	if len(users) != 2 {
		t.Errorf("users length = %d, want 2", len(users))
	}
}

func TestTool_Invoke_ResultOutputIsJSON(t *testing.T) {
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"key": "value"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	// Output should be valid JSON
	if len(result.Output) == 0 || result.Output[0] != '{' {
		t.Errorf("result.Output should be JSON object string, got: %s", result.Output)
	}
}

func TestTool_Invoke_MultipleCallsIndependent(t *testing.T) {
	tool := NewTool()

	r1, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"call": "first"},
	})
	r2, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"call": "second"},
	})

	d1 := r1.Meta["data"].(Output)
	d2 := r2.Meta["data"].(Output)

	if d1.StructuredOutput["call"] != "first" {
		t.Error("first call data corrupted")
	}
	if d2.StructuredOutput["call"] != "second" {
		t.Error("second call data corrupted")
	}
}

func TestFormatOutputJSON(t *testing.T) {
	output := Output{
		Success:          true,
		Output:           "test",
		StructuredOutput: Input{"key": "value"},
	}
	result := formatOutputJSON(output)
	if result == "" {
		t.Error("formatOutputJSON should not return empty string")
	}
	if result[0] != '{' {
		t.Error("formatOutputJSON should return JSON object")
	}
}
