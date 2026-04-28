package enter_plan_mode

import (
	"context"
	"strings"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func TestName(t *testing.T) {
	tool := NewTool()
	if tool.Name() != Name {
		t.Errorf("expected Name %q, got %q", Name, tool.Name())
	}
}

func TestDescription(t *testing.T) {
	tool := NewTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
	if !strings.Contains(desc, "plan mode") {
		t.Errorf("expected Description to contain 'plan mode', got %q", desc)
	}
}

func TestIsReadOnly(t *testing.T) {
	tool := NewTool()
	if !tool.IsReadOnly() {
		t.Error("expected IsReadOnly to return true")
	}
}

func TestIsConcurrencySafe(t *testing.T) {
	tool := NewTool()
	if !tool.IsConcurrencySafe() {
		t.Error("expected IsConcurrencySafe to return true")
	}
}

func TestRequiresUserInteraction(t *testing.T) {
	tool := NewTool()
	if !tool.RequiresUserInteraction() {
		t.Error("expected RequiresUserInteraction to return true")
	}
}

func TestInputSchema(t *testing.T) {
	tool := NewTool()
	schema := tool.InputSchema()

	if schema.Properties == nil {
		t.Error("expected non-nil Properties map")
	}
	if len(schema.Properties) != 0 {
		t.Errorf("expected empty Properties map, got %d entries", len(schema.Properties))
	}
}

func TestInvoke(t *testing.T) {
	tool := NewTool()

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
		t.Error("expected non-empty output text")
	}
	if !strings.Contains(result.Output, "plan mode") {
		t.Errorf("expected output to contain 'plan mode', got %q", result.Output)
	}

	// Verify Meta contains the Output data.
	data, ok := result.Meta["data"]
	if !ok {
		t.Fatal("expected Meta to contain 'data' key")
	}
	output, ok := data.(Output)
	if !ok {
		t.Fatalf("expected Meta data to be of type Output, got %T", data)
	}
	if output.Message == "" {
		t.Error("expected non-empty Message in Output")
	}
	if !strings.Contains(output.Message, "plan mode") {
		t.Errorf("expected Output.Message to contain 'plan mode', got %q", output.Message)
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
