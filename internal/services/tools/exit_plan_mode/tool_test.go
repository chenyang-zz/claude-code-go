package exit_plan_mode

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
		t.Error("expected Description to return non-empty string")
	}
	if !strings.Contains(desc, "plan mode") {
		t.Errorf("expected Description to contain 'plan mode', got %q", desc)
	}
}

func TestIsReadOnly(t *testing.T) {
	tool := NewTool()
	if tool.IsReadOnly() {
		t.Error("expected IsReadOnly to return false (ExitPlanMode writes state)")
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

	prop, ok := schema.Properties["allowedPrompts"]
	if !ok {
		t.Error("expected 'allowedPrompts' property in input schema")
		return
	}
	if prop.Type != coretool.ValueKindArray {
		t.Errorf("expected 'allowedPrompts' type to be array, got %s", prop.Type)
	}
	if prop.Required {
		t.Error("expected 'allowedPrompts' to not be required")
	}
	if prop.Items == nil {
		t.Error("expected 'allowedPrompts' to have Items defined")
	} else if prop.Items.Type != coretool.ValueKindObject {
		t.Errorf("expected 'allowedPrompts' items type to be object, got %s", prop.Items.Type)
	}
}

func TestInvokeWithAllowedPrompts(t *testing.T) {
	tool := NewTool()
	input := map[string]any{
		"allowedPrompts": []any{
			map[string]any{
				"tool":   "Bash",
				"prompt": "run tests",
			},
			map[string]any{
				"tool":   "Bash",
				"prompt": "install dependencies",
			},
		},
	}

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: input,
		Context: coretool.UseContext{
			WorkingDir: t.TempDir(),
		},
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

	// Verify Meta contains the Output data.
	if _, ok := result.Meta["data"]; !ok {
		t.Error("expected Meta to contain 'data' key")
	}
}

func TestInvokeEmptyInput(t *testing.T) {
	tool := NewTool()

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: map[string]any{},
		Context: coretool.UseContext{
			WorkingDir: t.TempDir(),
		},
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

	// Since no plan file exists, output should be the default approval message.
	if !strings.Contains(result.Output, "approved exiting plan mode") {
		t.Errorf("expected output to contain 'approved exiting plan mode', got %q", result.Output)
	}
}

func TestInvokeNilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: map[string]any{},
		Context: coretool.UseContext{
			WorkingDir: t.TempDir(),
		},
	})
	if err == nil {
		t.Error("expected error for nil receiver")
	}
}
