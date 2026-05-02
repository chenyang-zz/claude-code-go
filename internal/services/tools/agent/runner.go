package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	"github.com/sheepzhao/claude-code-go/internal/runtime/executor"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/agent/memory"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Runner executes a single agent task by filtering tools and running the engine.
type Runner struct {
	// ParentRuntime is the engine runtime used to create child runtimes.
	// It provides the client, executor, and default configuration.
	ParentRuntime *engine.Runtime
	// Registry holds available agent definitions.
	Registry agent.Registry
	// SessionConfig carries the current session configuration snapshot for dynamic prompt rendering.
	SessionConfig coretool.SessionConfigSnapshot
	// ServerRegistry provides access to MCP servers for agent-specific dynamic connections.
	ServerRegistry *mcpregistry.ServerRegistry
	// ToolRegistry holds the parent runtime's tool registry, used to build a child registry
	// that includes both parent tools and agent-specific MCP tools.
	ToolRegistry coretool.Registry
}

// NewRunner creates an agent runner wired to the given runtime and registry.
func NewRunner(parent *engine.Runtime, registry agent.Registry) *Runner {
	return &Runner{
		ParentRuntime: parent,
		Registry:      registry,
	}
}

// Run executes an agent task and returns the collected output.
// It looks up the agent definition, filters disallowed tools, builds the system prompt,
// and delegates execution to a child engine runtime.
func (r *Runner) Run(ctx context.Context, input Input) (Output, error) {
	if r.Registry == nil {
		return Output{}, fmt.Errorf("agent registry is nil")
	}

	// 1. Look up agent definition
	def, ok := r.Registry.Get(input.SubagentType)
	if !ok {
		return Output{}, fmt.Errorf("agent type %q not found in registry", input.SubagentType)
	}

	// Generate a unique per-run agent ID (mirrors TS runAgent.ts where agentId is a UUID).
	agentID := fmt.Sprintf("agent-%s-%s", def.AgentType, uuid.NewString()[:8])

	logger.DebugCF("agent.runner", "running agent", map[string]any{
		"agent_type": def.AgentType,
		"model":      def.Model,
	})
	coretool.ReportProgress(ctx, coretool.AgentToolProgressData{
		Type:      "agent_tool_progress",
		Status:    "started",
		AgentType: def.AgentType,
	})

	// 2. Sync agent memory snapshot before building the prompt.
	memoryScope := ""
	if def.Memory != "" {
		memoryScope = def.Memory
		paths := &memory.Paths{CWD: input.Cwd}
		scope := memory.AgentMemoryScope(memoryScope)
		result, err := paths.CheckAgentMemorySnapshot(def.AgentType, scope)
		if err == nil {
			switch result.Action {
			case "initialize":
				if syncErr := paths.InitializeFromSnapshot(def.AgentType, scope, result.SnapshotTimestamp); syncErr != nil {
					logger.WarnCF("agent.runner", "failed to initialize agent memory from snapshot", map[string]any{
						"agent_type": def.AgentType,
						"error":      syncErr.Error(),
					})
				}
			case "prompt-update":
				if syncErr := paths.ReplaceFromSnapshot(def.AgentType, scope, result.SnapshotTimestamp); syncErr != nil {
					logger.WarnCF("agent.runner", "failed to replace agent memory from snapshot", map[string]any{
						"agent_type": def.AgentType,
						"error":      syncErr.Error(),
					})
				}
			}
		}
	}

	// 2.5 Fire SubagentStart hooks and collect additional context.
	// This matches the TS runAgent.ts pattern where hook output is prepended as a
	// user message to the sub-agent's initial prompt.
	_, blocked, blockingMessages, additionalContext := r.ParentRuntime.RunSubagentStartHooks(
		ctx, agentID, def.AgentType, input.Cwd,
	)
	if blocked {
		return Output{}, fmt.Errorf("subagent start blocked by hook: %s",
			strings.Join(blockingMessages, "; "))
	}

	// 3. Build system prompt
	systemPrompt := r.buildSystemPrompt(def, coretool.UseContext{WorkingDir: input.Cwd, SessionConfig: r.SessionConfig}, memoryScope)

	// 3. Resolve tools according to agent definition allowlist / denylist / defaults.
	resolved := resolveAgentTools(def, r.ParentRuntime.ToolCatalog)
	filteredTools := resolved.Tools

	// 3.5 Initialize agent-specific MCP servers (additive to parent's servers).
	mcpResult, mcpErr := r.initializeAgentMCPServers(ctx, def)
	if mcpErr != nil {
		logger.WarnCF("agent.runner", "failed to initialize agent MCP servers", map[string]any{
			"agent_type": def.AgentType,
			"error":      mcpErr.Error(),
		})
	}
	defer mcpResult.cleanup()

	// 3.6 Merge MCP tools with resolved tools, deduplicating by name.
	// Parent tools take precedence (matching TS uniqBy behavior).
	allToolDefs := mergeToolDefinitions(filteredTools, mcpResult.toolDefs)

	// 3.7 Build child executor that includes both parent tools and agent MCP tools.
	var childExecutor engine.ToolExecutor
	if len(mcpResult.tools) > 0 && r.ToolRegistry != nil {
		childRegistry := coretool.NewMemoryRegistry()
		for _, t := range r.ToolRegistry.List() {
			_ = childRegistry.Register(t)
		}
		for _, t := range mcpResult.tools {
			if regErr := childRegistry.Register(t); regErr != nil {
				logger.WarnCF("agent.runner", "failed to register agent MCP tool", map[string]any{
					"tool":  t.Name(),
					"error": regErr.Error(),
				})
			}
		}
		childExecutor = executor.NewToolExecutor(childRegistry)
	} else {
		childExecutor = r.ParentRuntime.Executor
	}

	// 4. Determine model (inherit from parent or use agent override)
	modelName := r.selectModel(def)

	// 5. Create child runtime with merged tools and child executor
	child := engine.New(r.ParentRuntime.Client, modelName, childExecutor, allToolDefs...)
	// Copy key configuration from parent
	child.MaxToolIterations = r.resolveMaxTurns(def)
	child.ApprovalService = r.ParentRuntime.ApprovalService
	child.EnablePromptCaching = r.ParentRuntime.EnablePromptCaching
	child.RetryPolicy = r.ParentRuntime.RetryPolicy
	child.MaxConcurrentToolCalls = r.ParentRuntime.MaxConcurrentToolCalls
	// Merge agent hooks with parent hooks; Stop hooks become SubagentStop in agent context.
	child.Hooks = r.mergeAgentHooks(def)
	child.HookRunner = r.ParentRuntime.HookRunner

	// 6. Build and run request
	msgs := buildAgentMessages(def.InitialPrompt, input.Prompt)
	if additionalContext != "" {
		hookMsg := message.Message{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart(additionalContext),
			},
		}
		msgs = append([]message.Message{hookMsg}, msgs...)
	}
	req := conversation.RunRequest{
		SessionID: agentID,
		Messages:  msgs,
		CWD:       input.Cwd,
		System:    systemPrompt,
	}

	start := time.Now()
	stream, err := child.Run(ctx, req)
	if err != nil {
		return Output{}, fmt.Errorf("agent run failed: %w", err)
	}

	// 7. Collect events and build output
	output := r.collectOutput(stream, def.AgentType, start)
	coretool.ReportProgress(ctx, coretool.AgentToolProgressData{
		Type:              "agent_tool_progress",
		Status:            "finished",
		AgentType:         def.AgentType,
		DurationMs:        output.TotalDurationMs,
		TotalToolUseCount: output.TotalToolUseCount,
		TotalTokens:       output.TotalTokens,
	})

	logger.DebugCF("agent.runner", "agent run completed", map[string]any{
		"agent_type":   def.AgentType,
		"duration_ms":  output.TotalDurationMs,
		"tool_uses":    output.TotalToolUseCount,
		"total_tokens": output.TotalTokens,
	})

	return output, nil
}

// buildSystemPrompt generates the system prompt for the given agent definition.
// When a SystemPromptProvider or static SystemPrompt is configured, the prompt
// is returned as-is with an appended "Available tools" note. Otherwise only the
// tools note is returned.
//
// If memoryScope is non-empty, the agent's persistent memory prompt is loaded
// and prepended to the base prompt so the agent receives its memory context
// before task-specific instructions.
func (r *Runner) buildSystemPrompt(def agent.Definition, toolCtx coretool.UseContext, memoryScope string) string {
	var base string
	if def.SystemPromptProvider != nil {
		base = def.SystemPromptProvider.GetSystemPrompt(toolCtx)
	} else if strings.TrimSpace(def.SystemPrompt) != "" {
		base = strings.TrimSpace(def.SystemPrompt)
	}

	// Inject memory prompt when the agent declares a memory scope.
	var memoryPrompt string
	if memoryScope != "" {
		paths := &memory.Paths{CWD: toolCtx.WorkingDir}
		scope := memory.AgentMemoryScope(memoryScope)
		memoryPrompt = memory.LoadAgentMemoryPrompt(def.AgentType, scope, paths)
	}

	toolsNote := fmt.Sprintf("\n\nAvailable tools: %s", formatToolList(def))

	if memoryPrompt != "" {
		if base != "" {
			return memoryPrompt + "\n\n" + base + toolsNote
		}
		return memoryPrompt + toolsNote
	}
	return base + toolsNote
}

// buildAgentMessages builds the agent request message sequence for one task.
// When an initial prompt is configured, it is prepended as the first user turn
// so the actual task prompt remains intact as the next message.
func buildAgentMessages(initialPrompt string, prompt string) []message.Message {
	messages := make([]message.Message, 0, 2)
	if strings.TrimSpace(initialPrompt) != "" {
		messages = append(messages, message.Message{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart(initialPrompt),
			},
		})
	}
	messages = append(messages, message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart(prompt),
		},
	})
	return messages
}

// selectModel chooses the model for the agent run.
// If the agent defines a model override, use it; otherwise inherit from the parent.
func (r *Runner) selectModel(def agent.Definition) string {
	model := strings.TrimSpace(def.Model)
	if model != "" && model != "inherit" {
		return model
	}
	if r.ParentRuntime != nil {
		return r.ParentRuntime.DefaultModel
	}
	return ""
}

// resolveMaxTurns returns the max tool iterations for the agent.
func (r *Runner) resolveMaxTurns(def agent.Definition) int {
	if def.MaxTurns > 0 {
		return def.MaxTurns
	}
	if r.ParentRuntime != nil && r.ParentRuntime.MaxToolIterations > 0 {
		return r.ParentRuntime.MaxToolIterations
	}
	return 8 // default
}

// collectOutput drains the event stream and builds the agent Output.
func (r *Runner) collectOutput(stream event.Stream, agentType string, start time.Time) Output {
	var content []TextBlock
	var totalTokens int
	var toolUseCount int
	var usage UsageStats

	for evt := range stream {
		switch evt.Type {
		case event.TypeMessageDelta:
			if payload, ok := evt.Payload.(event.MessageDeltaPayload); ok {
				content = append(content, TextBlock{Type: "text", Text: payload.Text})
			}
		case event.TypeToolCallStarted:
			toolUseCount++
		case event.TypeUsage:
			if payload, ok := evt.Payload.(event.UsagePayload); ok {
				u := payload.CumulativeUsage
				totalTokens = u.InputTokens + u.OutputTokens
				usage.InputTokens = u.InputTokens
				usage.OutputTokens = u.OutputTokens
				usage.CacheReadInputTokens = u.CacheReadInputTokens
				usage.CacheCreationInputTokens = u.CacheCreationInputTokens
			}
		}
	}

	// Compact consecutive text blocks into a single block
	if len(content) > 0 {
		var merged []string
		for _, block := range content {
			merged = append(merged, block.Text)
		}
		content = []TextBlock{{Type: "text", Text: strings.Join(merged, "")}}
	}

	return Output{
		AgentID:           fmt.Sprintf("%s-%d", agentType, start.Unix()),
		AgentType:         agentType,
		Content:           content,
		TotalToolUseCount: toolUseCount,
		TotalDurationMs:   int(time.Since(start).Milliseconds()),
		TotalTokens:       totalTokens,
		Usage:             usage,
	}
}

// mergeAgentHooks merges the agent's frontmatter hooks with the parent runtime's
// hooks. For agent contexts, Stop hooks are converted to SubagentStop to match
// the subagent lifecycle events, mirroring the TypeScript registerFrontmatterHooks
// behavior where isAgent=true causes Stop hooks to be registered under SubagentStop.
func (r *Runner) mergeAgentHooks(def agent.Definition) hook.HooksConfig {
	if r.ParentRuntime == nil {
		return convertStopToSubagentStop(def.Hooks)
	}
	agentHooks := def.Hooks
	if len(agentHooks) == 0 {
		return r.ParentRuntime.Hooks
	}
	agentHooks = convertStopToSubagentStop(agentHooks)
	return hook.MergeHooksConfig(r.ParentRuntime.Hooks, agentHooks)
}

// convertStopToSubagentStop converts Stop event hooks to SubagentStop event hooks.
// This ensures that agent-specific stop hooks fire on SubagentStop (the event
// emitted when a subagent completes) rather than Stop (the main session event).
func convertStopToSubagentStop(cfg hook.HooksConfig) hook.HooksConfig {
	if len(cfg) == 0 {
		return cfg
	}
	result := make(hook.HooksConfig, len(cfg))
	for event, matchers := range cfg {
		if event == hook.EventStop {
			result[hook.EventSubagentStop] = matchers
		} else {
			result[event] = matchers
		}
	}
	return result
}
