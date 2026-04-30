package brief

import (
	"context"
	"encoding/json"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func TestBriefTool_Name(t *testing.T) {
	tool := NewTool()
	if tool.Name() != Name {
		t.Errorf("expected name %q, got %q", Name, tool.Name())
	}
}

func TestBriefTool_NilReceiver(t *testing.T) {
	var tool *Tool
	if tool.Name() != Name {
		t.Errorf("expected nil-safe name %q, got %q", Name, tool.Name())
	}
	result, err := tool.Invoke(context.TODO(), coretool.Call{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" || result.Error != "SendUserMessage tool: nil receiver" {
		t.Errorf("expected nil receiver error, got %q", result.Error)
	}
}

func TestBriefTool_Aliases(t *testing.T) {
	tool := NewTool()
	aliases := tool.Aliases()
	if len(aliases) != 1 || aliases[0] != legacyAlias {
		t.Errorf("expected alias [%q], got %v", legacyAlias, aliases)
	}
}

func TestBriefTool_InputSchema(t *testing.T) {
	tool := NewTool()
	schema := tool.InputSchema()
	if schema.Properties["message"].Type != coretool.ValueKindString {
		t.Errorf("expected message type string, got %s", schema.Properties["message"].Type)
	}
	if !schema.Properties["message"].Required {
		t.Error("expected message to be required")
	}
	if schema.Properties["status"].Type != coretool.ValueKindString {
		t.Errorf("expected status type string, got %s", schema.Properties["status"].Type)
	}
}

func TestBriefTool_Invoke_BasicMessage(t *testing.T) {
	tool := NewTool()
	result, err := tool.Invoke(context.TODO(), coretool.Call{
		Input: map[string]any{
			"message": "Hello, world!",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	var output briefOutput
	if err := json.Unmarshal([]byte(result.Output), &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	if output.Message != "Hello, world!" {
		t.Errorf("expected message %q, got %q", "Hello, world!", output.Message)
	}
	if output.SentAt == "" {
		t.Error("expected non-empty sentAt timestamp")
	}
}

func TestBriefTool_Invoke_WithStatus(t *testing.T) {
	tool := NewTool()
	result, err := tool.Invoke(context.TODO(), coretool.Call{
		Input: map[string]any{
			"message": "Task completed!",
			"status":  "proactive",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	var output briefOutput
	json.Unmarshal([]byte(result.Output), &output)
	if output.Message != "Task completed!" {
		t.Errorf("expected message %q, got %q", "Task completed!", output.Message)
	}
}

func TestBriefTool_Invoke_EmptyMessage(t *testing.T) {
	tool := NewTool()
	result, err := tool.Invoke(context.TODO(), coretool.Call{
		Input: map[string]any{
			"message": "",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for empty message")
	}
}

func TestBriefTool_Invoke_InvalidInput(t *testing.T) {
	tool := NewTool()
	result, err := tool.Invoke(context.TODO(), coretool.Call{
		Input: map[string]any{
			"message": 123, // not a string
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for invalid input type")
	}
}

func TestBriefTool_IsReadOnly(t *testing.T) {
	tool := NewTool()
	if !tool.IsReadOnly() {
		t.Error("expected IsReadOnly to return true")
	}
}

func TestBriefTool_IsConcurrencySafe(t *testing.T) {
	tool := NewTool()
	if !tool.IsConcurrencySafe() {
		t.Error("expected IsConcurrencySafe to return true")
	}
}

func TestBriefTool_Description(t *testing.T) {
	tool := NewTool()
	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}
}
