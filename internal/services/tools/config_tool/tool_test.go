package config_tool

import (
	"context"
	"strings"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func newTestTool(t *testing.T) *Tool {
	t.Helper()
	dir := t.TempDir()
	return NewTool(dir, dir)
}

func TestTool_Name(t *testing.T) {
	tool := newTestTool(t)
	if tool.Name() != Name {
		t.Errorf("Name() = %q, want %q", tool.Name(), Name)
	}
}

func TestTool_Description(t *testing.T) {
	tool := newTestTool(t)
	desc := tool.Description()
	if !strings.Contains(desc, "configuration settings") {
		t.Errorf("Description() should mention configuration settings: %q", desc)
	}
}

func TestTool_InputSchema(t *testing.T) {
	tool := newTestTool(t)
	schema := tool.InputSchema()

	settingField, ok := schema.Properties["setting"]
	if !ok {
		t.Fatal("schema missing 'setting' property")
	}
	if !settingField.Required {
		t.Error("setting should be required")
	}

	valueField, ok := schema.Properties["value"]
	if !ok {
		t.Fatal("schema missing 'value' property")
	}
	if valueField.Required {
		t.Error("value should not be required")
	}
}

func TestTool_IsReadOnly(t *testing.T) {
	tool := newTestTool(t)
	if tool.IsReadOnly() {
		t.Error("Config tool should not be read-only (supports set)")
	}
}

func TestTool_IsConcurrencySafe(t *testing.T) {
	tool := newTestTool(t)
	if !tool.IsConcurrencySafe() {
		t.Error("Config tool should be concurrency-safe")
	}
}

func TestTool_Invoke_GetTheme(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	// First set a value
	_, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": "theme", "value": "dark"},
	})
	if err != nil {
		t.Fatalf("set: %v", err)
	}

	// Now get it
	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": "theme"},
	})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "get") {
		t.Errorf("expected get operation in output: %s", result.Output)
	}
	if !strings.Contains(result.Output, "dark") {
		t.Errorf("expected 'dark' in output: %s", result.Output)
	}
}

func TestTool_Invoke_SetModel(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": "model", "value": "sonnet"},
	})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "set") {
		t.Errorf("expected set operation: %s", result.Output)
	}
	if !strings.Contains(result.Output, "sonnet") {
		t.Errorf("expected sonnet in output: %s", result.Output)
	}
}

func TestTool_Invoke_SetFastMode(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": "fastMode", "value": "true"},
	})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "\"success\":true") {
		t.Errorf("expected success: %s", result.Output)
	}
}

func TestTool_Invoke_SetBooleanStringCoercion(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	// "true" string should be coerced to boolean true
	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": "fastMode", "value": "true"},
	})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "\"success\":true") {
		t.Errorf("string 'true' should be accepted as boolean: %s", result.Output)
	}

	// "false" string should be coerced to boolean false
	result, err = tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": "fastMode", "value": "false"},
	})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
}

func TestTool_Invoke_SetBooleanInvalid(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": "fastMode", "value": "yes"},
	})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for invalid boolean value")
	}
	if !strings.Contains(result.Error, "requires true or false") {
		t.Errorf("expected boolean error: %s", result.Error)
	}
}

func TestTool_Invoke_SetNestedKey(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": "permissions.defaultMode", "value": "plan"},
	})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	// Verify can get it back
	result, err = tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": "permissions.defaultMode"},
	})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !strings.Contains(result.Output, "plan") {
		t.Errorf("expected plan in output: %s", result.Output)
	}
}

func TestTool_Invoke_SetInvalidOption(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": "theme", "value": "rainbow"},
	})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for invalid option")
	}
	if !strings.Contains(result.Error, "Invalid value") {
		t.Errorf("expected Invalid value error: %s", result.Error)
	}
}

func TestTool_Invoke_UnknownSetting(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": "nonexistentSetting"},
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for unknown setting")
	}
	if !strings.Contains(result.Error, "Unknown setting") {
		t.Errorf("expected Unknown setting error: %s", result.Error)
	}
}

func TestTool_Invoke_EmptySetting(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": ""},
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for empty setting")
	}
}

func TestTool_Invoke_MissingSettingField(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{},
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for missing setting field")
	}
}

func TestTool_Invoke_GetWithNoWriter(t *testing.T) {
	tool := &Tool{writer: nil}
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": "theme"},
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for nil writer")
	}
}

func TestTool_Invoke_SetEffortLevel(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": "effortLevel", "value": "high"},
	})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "\"success\":true") {
		t.Errorf("expected success: %s", result.Output)
	}
}

func TestTool_Invoke_SetEditorMode(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"setting": "editorMode", "value": "vim"},
	})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
}
