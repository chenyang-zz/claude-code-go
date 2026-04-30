package sleep

import (
	"context"
	"strings"
	"testing"
	"time"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func TestTool_Name(t *testing.T) {
	tool := NewTool()
	if tool.Name() != Name {
		t.Errorf("expected Name %q, got %q", Name, tool.Name())
	}
}

func TestTool_Description(t *testing.T) {
	tool := NewTool()
	if tool.Description() == "" {
		t.Error("expected non-empty Description")
	}
	if !strings.Contains(tool.Description(), "Wait for a specified duration") {
		t.Error("Description should contain the primary purpose")
	}
	if !strings.Contains(tool.Description(), "Bash(sleep)") {
		t.Error("Description should mention Bash(sleep) alternative")
	}
}

func TestTool_IsReadOnly(t *testing.T) {
	tool := NewTool()
	if !tool.IsReadOnly() {
		t.Error("Sleep should be read-only")
	}
}

func TestTool_IsConcurrencySafe(t *testing.T) {
	tool := NewTool()
	if !tool.IsConcurrencySafe() {
		t.Error("Sleep should be concurrency-safe")
	}
}

func TestTool_InputSchema(t *testing.T) {
	tool := NewTool()
	schema := tool.InputSchema()

	durationField, ok := schema.Properties["duration"]
	if !ok {
		t.Fatal("expected 'duration' property in schema")
	}
	if durationField.Type != coretool.ValueKindNumber {
		t.Errorf("expected type 'number', got %q", durationField.Type)
	}
	if durationField.Required {
		t.Error("duration should not be required")
	}
	if durationField.Description == "" {
		t.Error("duration should have a description")
	}
}

func TestTool_Invoke_DefaultDuration(t *testing.T) {
	tool := NewTool()
	ctx := context.Background()
	call := coretool.Call{
		Input: map[string]any{},
	}

	result, err := tool.Invoke(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Slept for") {
		t.Errorf("expected output to contain 'Slept for', got %q", result.Output)
	}
}

func TestTool_Invoke_CustomDuration(t *testing.T) {
	tool := NewTool()
	ctx := context.Background()
	call := coretool.Call{
		Input: map[string]any{
			"duration": 0.5,
		},
	}

	result, err := tool.Invoke(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "0.5 seconds") {
		t.Errorf("expected output to contain '0.5 seconds', got %q", result.Output)
	}
}

func TestTool_Invoke_DurationTooSmall(t *testing.T) {
	tool := NewTool()
	ctx := context.Background()
	call := coretool.Call{
		Input: map[string]any{
			"duration": 0.05,
		},
	}

	result, err := tool.Invoke(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for too-small duration")
	}
}

func TestTool_Invoke_DurationTooLarge(t *testing.T) {
	tool := NewTool()
	ctx := context.Background()
	call := coretool.Call{
		Input: map[string]any{
			"duration": 7200.0,
		},
	}

	result, err := tool.Invoke(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for too-large duration")
	}
}

func TestTool_Invoke_ContextCancelled(t *testing.T) {
	tool := NewTool()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	call := coretool.Call{
		Input: map[string]any{
			"duration": 10.0,
		},
	}

	result, err := tool.Invoke(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "interrupted") {
		t.Errorf("expected output to contain 'interrupted', got %q", result.Output)
	}
}

func TestTool_Invoke_ContextTimeout(t *testing.T) {
	tool := NewTool()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	call := coretool.Call{
		Input: map[string]any{
			"duration": 10.0,
		},
	}

	result, err := tool.Invoke(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "interrupted") {
		t.Errorf("expected output to contain 'interrupted', got %q", result.Output)
	}
}

func TestTool_Invoke_NilReceiver(t *testing.T) {
	var tool *Tool
	ctx := context.Background()
	call := coretool.Call{
		Input: map[string]any{},
	}

	_, err := tool.Invoke(ctx, call)
	if err == nil {
		t.Error("expected error for nil receiver")
	}
}

func TestTool_Invoke_InvalidInputType(t *testing.T) {
	tool := NewTool()
	ctx := context.Background()
	call := coretool.Call{
		Input: map[string]any{
			"duration": "not-a-number",
		},
	}

	result, err := tool.Invoke(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected decode error for invalid type")
	}
}

func TestTool_Invoke_NegativeDuration(t *testing.T) {
	tool := NewTool()
	ctx := context.Background()
	call := coretool.Call{
		Input: map[string]any{
			"duration": -1.0,
		},
	}

	result, err := tool.Invoke(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for negative duration")
	}
}

func TestTool_Invoke_OutputMeta(t *testing.T) {
	tool := NewTool()
	ctx := context.Background()
	call := coretool.Call{
		Input: map[string]any{
			"duration": 0.2,
		},
	}

	result, err := tool.Invoke(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	if result.Meta == nil {
		t.Fatal("expected non-nil Meta")
	}
	data, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatalf("expected Meta[\"data\"] to be Output, got %T", result.Meta["data"])
	}
	if data.Interrupted {
		t.Error("expected Interrupted to be false")
	}
	if data.Duration != 0.2 {
		t.Errorf("expected Duration 0.2, got %f", data.Duration)
	}
}
