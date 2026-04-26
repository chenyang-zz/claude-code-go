package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
)

func TestTool_Name(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil)
	if got := tool.Name(); got != "Agent" {
		t.Errorf("Name() = %q, want %q", got, "Agent")
	}
}

func TestTool_Description(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil)
	got := tool.Description()
	if got == "" {
		t.Error("Description() should not be empty")
	}
	// Verify it contains the fallback content when registry is nil
	if !strings.Contains(got, "Launch a specialized agent") {
		t.Error("Description() missing expected content")
	}
}

func TestTool_Description_WithRegistry(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	_ = registry.Register(agent.Definition{
		AgentType: "explore",
		WhenToUse: "Search specialist",
		Tools:     []string{"Read", "Bash"},
	})
	tool := NewTool(registry, nil, nil, nil)
	got := tool.Description()
	if got == "" {
		t.Error("Description() should not be empty")
	}
	// With registry, should contain dynamic content
	if !strings.Contains(got, "Available agent types") {
		t.Error("Description() missing dynamic content")
	}
	if !strings.Contains(got, "- explore: Search specialist (Tools: Read, Bash)") {
		t.Error("Description() missing agent listing")
	}
}

func TestTool_IsReadOnly(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil)
	if got := tool.IsReadOnly(); got != false {
		t.Errorf("IsReadOnly() = %v, want false", got)
	}
}

func TestTool_IsConcurrencySafe(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil)
	if got := tool.IsConcurrencySafe(); got != true {
		t.Errorf("IsConcurrencySafe() = %v, want true", got)
	}
}

func TestTool_InputSchema(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil)
	schema := tool.InputSchema()

	requiredFields := []string{"description", "prompt"}
	for _, field := range requiredFields {
		fs, ok := schema.Properties[field]
		if !ok {
			t.Errorf("InputSchema() missing required field %q", field)
			continue
		}
		if !fs.Required {
			t.Errorf("field %q should be required", field)
		}
	}

	optionalFields := []string{"subagent_type", "model", "run_in_background", "name", "cwd"}
	for _, field := range optionalFields {
		fs, ok := schema.Properties[field]
		if !ok {
			t.Errorf("InputSchema() missing optional field %q", field)
			continue
		}
		if fs.Required {
			t.Errorf("field %q should not be required", field)
		}
	}
}

func TestTool_Invoke_NilRegistry(t *testing.T) {
	agentTool := NewTool(nil, nil, nil, nil)
	call := tool.Call{
		Input: map[string]any{
			"description": "test task",
			"prompt":      "do something",
		},
	}
	result, err := agentTool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error result when registry is nil")
	}
}

func TestTool_Invoke_NilParentRuntime(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	agentTool := NewTool(registry, nil, nil, nil)
	call := tool.Call{
		Input: map[string]any{
			"description": "test task",
			"prompt":      "do something",
		},
	}
	result, err := agentTool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error result when parent runtime is nil")
	}
}

func TestTool_Invoke_InvalidInput(t *testing.T) {
	// Create a minimal registry and a dummy runtime so we pass the nil checks.
	registry := agent.NewInMemoryRegistry()
	// We can't easily create a working engine.Runtime without a real client,
	// but for the invalid-input path the runtime is only checked for nil.
	// Use a zero-value runtime; the decode failure should happen before runner.Run.
	parentRuntime := &engine.Runtime{}
	agentTool := NewTool(registry, parentRuntime, nil, nil)

	call := tool.Call{
		Input: map[string]any{
			// Missing required "description" and "prompt"
			"subagent_type": "Explore",
		},
	}
	result, err := agentTool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error result for invalid input")
	}
}
