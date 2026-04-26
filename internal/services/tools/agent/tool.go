package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Tool implements the coretool.Tool interface for the Agent tool.
// It dispatches agent requests to a Runner created on demand from the
// configured registry and parent runtime.
type Tool struct {
	registry       agent.Registry
	parentRuntime  *engine.Runtime
	serverRegistry *mcpregistry.ServerRegistry
	toolRegistry   coretool.Registry
	descriptor     *Descriptor
}

// NewTool creates an Agent tool wired to the given registry and parent runtime.
func NewTool(registry agent.Registry, parentRuntime *engine.Runtime, serverRegistry *mcpregistry.ServerRegistry, toolRegistry coretool.Registry) *Tool {
	t := &Tool{
		registry:       registry,
		parentRuntime:  parentRuntime,
		serverRegistry: serverRegistry,
		toolRegistry:   toolRegistry,
	}
	if registry != nil {
		t.descriptor = &Descriptor{Registry: registry}
	}
	return t
}

// Name returns the tool name used for registration and dispatch.
func (t *Tool) Name() string {
	return "Agent"
}

// Description returns the tool description exposed to the model.
// When a descriptor is configured, it returns a dynamic description based
// on the registered agent types; otherwise it falls back to a static string.
func (t *Tool) Description() string {
	if t.descriptor != nil {
		return t.descriptor.Description()
	}
	return "Launch a specialized agent to perform a task. Use this when you need to delegate work to a subagent."
}

// InputSchema returns the JSON schema for the agent tool input.
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"description": {
				Type:        coretool.ValueKindString,
				Description: "A short (3-5 word) description of the task.",
				Required:    true,
			},
			"prompt": {
				Type:        coretool.ValueKindString,
				Description: "The task for the agent to perform.",
				Required:    true,
			},
			"subagent_type": {
				Type:        coretool.ValueKindString,
				Description: "The type of specialized agent to use (e.g., 'Explore').",
				Required:    false,
			},
			"model": {
				Type:        coretool.ValueKindString,
				Description: "Optional model override for the agent.",
				Required:    false,
			},
			"run_in_background": {
				Type:        coretool.ValueKindBoolean,
				Description: "Whether the agent should run in the background.",
				Required:    false,
			},
			"name": {
				Type:        coretool.ValueKindString,
				Description: "Optional name for the spawned agent.",
				Required:    false,
			},
			"cwd": {
				Type:        coretool.ValueKindString,
				Description: "Optional working directory override for the agent.",
				Required:    false,
			},
		},
	}
}

// IsReadOnly reports whether the tool avoids mutating external state.
// The Agent tool itself is not read-only because subagents may modify files.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports whether multiple invocations can run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// Invoke executes the agent tool.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t.registry == nil {
		return coretool.Result{Error: "agent registry is not configured"}, nil
	}
	if t.parentRuntime == nil {
		return coretool.Result{Error: "agent parent runtime is not configured"}, nil
	}

	input, err := coretool.DecodeInput[Input](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("invalid agent tool input: %v", err)}, nil
	}

	logger.DebugCF("agent.tool", "invoking agent", map[string]any{
		"subagent_type": input.SubagentType,
		"description":   input.Description,
	})

	runner := NewRunner(t.parentRuntime, t.registry)
	if t.parentRuntime != nil {
		runner.SessionConfig = t.parentRuntime.SessionConfig
	}
	runner.ServerRegistry = t.serverRegistry
	runner.ToolRegistry = t.toolRegistry
	output, err := runner.Run(ctx, input)
	if err != nil {
		logger.WarnCF("agent.tool", "agent run failed", map[string]any{
			"subagent_type": input.SubagentType,
			"error":         err.Error(),
		})
		return coretool.Result{Error: fmt.Sprintf("agent run failed: %v", err)}, nil
	}

	resultJSON, marshalErr := json.Marshal(output)
	if marshalErr != nil {
		return coretool.Result{Error: fmt.Sprintf("failed to marshal agent output: %v", marshalErr)}, nil
	}

	logger.DebugCF("agent.tool", "agent run completed", map[string]any{
		"subagent_type":     input.SubagentType,
		"total_tool_uses":   output.TotalToolUseCount,
		"total_duration_ms": output.TotalDurationMs,
	})

	return coretool.Result{
		Output: string(resultJSON),
	}, nil
}
