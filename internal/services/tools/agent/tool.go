package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Tool implements the coretool.Tool interface for the Agent tool.
// It dispatches agent requests to a Runner created on demand from the
// configured registry and parent runtime.
type Tool struct {
	registry        agent.Registry
	parentRuntime   *engine.Runtime
	serverRegistry  *mcpregistry.ServerRegistry
	toolRegistry    coretool.Registry
	taskStore       BackgroundTaskStore
	descriptor      *Descriptor
	teammateStarter teammateStarter
	runnerFactory   func() runner
	backgroundMu    sync.Mutex
	backgroundInput map[string]Input
}

// runner executes one agent task and returns the normalized output.
type runner interface {
	// Run executes one agent task.
	Run(ctx context.Context, input Input) (Output, error)
}

// BackgroundTaskStore describes the shared lifecycle store used by background Agent tasks.
type BackgroundTaskStore interface {
	// Register inserts one new live background task snapshot into the shared store.
	Register(task coresession.BackgroundTaskSnapshot, stopper interface{ Stop() error })
	// Update replaces the stored snapshot for one existing task.
	Update(task coresession.BackgroundTaskSnapshot) bool
	// Remove deletes one task from the shared task list.
	Remove(id string)
}

// NewTool creates an Agent tool wired to the given registry and parent runtime.
func NewTool(registry agent.Registry, parentRuntime *engine.Runtime, serverRegistry *mcpregistry.ServerRegistry, toolRegistry coretool.Registry, taskStore BackgroundTaskStore) *Tool {
	t := &Tool{
		registry:        registry,
		parentRuntime:   parentRuntime,
		serverRegistry:  serverRegistry,
		toolRegistry:    toolRegistry,
		taskStore:       taskStore,
		teammateStarter: osTeammateStarter{},
		backgroundInput: make(map[string]Input),
	}
	t.runnerFactory = func() runner {
		r := NewRunner(t.parentRuntime, t.registry)
		if t.parentRuntime != nil {
			r.SessionConfig = t.parentRuntime.SessionConfig
		}
		r.ServerRegistry = t.serverRegistry
		r.ToolRegistry = t.toolRegistry
		return r
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
			"team_name": {
				Type:        coretool.ValueKindString,
				Description: "Optional team name for teammate spawn.",
				Required:    false,
			},
			"mode": {
				Type:        coretool.ValueKindString,
				Description: "Optional permission mode for teammate spawn.",
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
		"background":    input.RunInBackground,
	})
	if strings.TrimSpace(input.Name) != "" && strings.TrimSpace(input.TeamName) != "" {
		spawnRequest := teammateSpawnRequest{
			Name:          strings.TrimSpace(input.Name),
			TeamName:      strings.TrimSpace(input.TeamName),
			Cwd:           strings.TrimSpace(input.Cwd),
			SessionConfig: t.parentRuntime.SessionConfig,
		}
		pid, spawnErr := t.teammateStarter.Start(ctx, spawnRequest)
		if spawnErr != nil {
			return coretool.Result{Error: fmt.Sprintf("teammate spawn failed: %v", spawnErr)}, nil
		}
		logger.DebugCF("agent.tool", "teammate spawn launched", map[string]any{
			"name":      spawnRequest.Name,
			"team_name": spawnRequest.TeamName,
			"pid":       pid,
		})
		spawnOutput := Output{
			AgentID:   fmt.Sprintf("%s@%s", spawnRequest.Name, spawnRequest.TeamName),
			AgentType: input.SubagentType,
			Content: []TextBlock{{
				Type: "text",
				Text: fmt.Sprintf("Teammate %s launched in team %s", spawnRequest.Name, spawnRequest.TeamName),
			}},
		}
		resultJSON, marshalErr := json.Marshal(spawnOutput)
		if marshalErr != nil {
			return coretool.Result{Error: fmt.Sprintf("failed to marshal teammate spawn output: %v", marshalErr)}, nil
		}
		return coretool.Result{Output: string(resultJSON)}, nil
	}

	if input.RunInBackground {
		return t.launchBackground(ctx, input), nil
	}

	output, err := t.runnerFactory().Run(ctx, input)
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
