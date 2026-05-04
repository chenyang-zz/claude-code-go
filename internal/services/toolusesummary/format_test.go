package toolusesummary

import (
	"strings"
	"testing"
)

func TestFormatToolBatch_Single(t *testing.T) {
	tools := []ToolInfo{
		{Name: "Read", Input: map[string]any{"file_path": "/tmp/a"}, Output: "content"},
	}
	result := formatToolBatch(tools)
	if !strings.Contains(result, "Tool: Read") {
		t.Errorf("missing tool name: %q", result)
	}
	if !strings.Contains(result, "Input:") {
		t.Errorf("missing input: %q", result)
	}
	if !strings.Contains(result, "Output:") {
		t.Errorf("missing output: %q", result)
	}
}

func TestFormatToolBatch_Multiple(t *testing.T) {
	tools := []ToolInfo{
		{Name: "Read", Input: "x", Output: "y"},
		{Name: "Bash", Input: "z", Output: "w"},
	}
	result := formatToolBatch(tools)
	parts := strings.Split(result, "\n\n")
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d: %q", len(parts), result)
	}
}

func TestFormatToolBatch_Empty(t *testing.T) {
	result := formatToolBatch(nil)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
	result = formatToolBatch([]ToolInfo{})
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestTruncateJSON_ExactFit(t *testing.T) {
	val := map[string]any{"a": 1}
	result := truncateJSON(val, 20)
	if result == "" {
		t.Error("expected non-empty")
	}
	if strings.Contains(result, "...") {
		t.Error("expected no truncation")
	}
}

func TestTruncateJSON_Truncates(t *testing.T) {
	val := map[string]any{"key": "this is a long value"}
	result := truncateJSON(val, 10)
	if len(result) != 10 {
		t.Errorf("len = %d, want 10", len(result))
	}
	if !strings.HasSuffix(result, "...") {
		t.Errorf("expected ... suffix: %q", result)
	}
}

func TestTruncateJSON_MaxLengthSmall(t *testing.T) {
	result := truncateJSON("x", 3)
	if result != "..." {
		t.Errorf("result = %q, want ...", result)
	}
	result = truncateJSON("x", 2)
	if result != "..." {
		t.Errorf("result = %q, want ...", result)
	}
}

func TestTruncateJSON_Unserializable(t *testing.T) {
	result := truncateJSON(make(chan int), 100)
	if result != "[unable to serialize]" {
		t.Errorf("result = %q, want [unable to serialize]", result)
	}
}

func TestTruncateJSON_Nil(t *testing.T) {
	result := truncateJSON(nil, 100)
	if result != "null" {
		t.Errorf("result = %q, want null", result)
	}
}
