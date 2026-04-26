package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
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

	logger.DebugCF("agent.runner", "running agent", map[string]any{
		"agent_type": def.AgentType,
		"model":      def.Model,
	})

	// 2. Build system prompt
	systemPrompt := r.buildSystemPrompt(def, coretool.UseContext{WorkingDir: input.Cwd, SessionConfig: r.SessionConfig})

	// 3. Filter tools based on disallowed list
	filteredTools := filterToolCatalog(r.ParentRuntime.ToolCatalog, def.DisallowedTools)

	// 4. Determine model (inherit from parent or use agent override)
	modelName := r.selectModel(def)

	// 5. Create child runtime with filtered tools
	child := engine.New(r.ParentRuntime.Client, modelName, r.ParentRuntime.Executor, filteredTools...)
	// Copy key configuration from parent
	child.MaxToolIterations = r.resolveMaxTurns(def)
	child.ApprovalService = r.ParentRuntime.ApprovalService
	child.EnablePromptCaching = r.ParentRuntime.EnablePromptCaching
	child.RetryPolicy = r.ParentRuntime.RetryPolicy
	child.MaxConcurrentToolCalls = r.ParentRuntime.MaxConcurrentToolCalls

	// 6. Build and run request
	req := conversation.RunRequest{
		SessionID: fmt.Sprintf("agent-%s-%d", def.AgentType, time.Now().Unix()),
		Messages:  buildAgentMessages(def.InitialPrompt, input.Prompt),
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

	logger.DebugCF("agent.runner", "agent run completed", map[string]any{
		"agent_type":   def.AgentType,
		"duration_ms":  output.TotalDurationMs,
		"tool_uses":    output.TotalToolUseCount,
		"total_tokens": output.TotalTokens,
	})

	return output, nil
}

// buildSystemPrompt generates the system prompt for the given agent definition.
func (r *Runner) buildSystemPrompt(def agent.Definition, toolCtx coretool.UseContext) string {
	if def.SystemPromptProvider != nil {
		return def.SystemPromptProvider.GetSystemPrompt(toolCtx)
	}
	if strings.TrimSpace(def.SystemPrompt) != "" {
		return strings.TrimSpace(def.SystemPrompt)
	}
	return ""
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

// filterToolCatalog removes tools whose names appear in the disallowed list.
func filterToolCatalog(catalog []model.ToolDefinition, disallowed []string) []model.ToolDefinition {
	if len(disallowed) == 0 {
		return append([]model.ToolDefinition(nil), catalog...)
	}

	disallowedSet := make(map[string]struct{}, len(disallowed))
	for _, name := range disallowed {
		disallowedSet[name] = struct{}{}
	}

	filtered := make([]model.ToolDefinition, 0, len(catalog))
	for _, tool := range catalog {
		if _, ok := disallowedSet[tool.Name]; !ok {
			filtered = append(filtered, tool)
		}
	}

	logger.DebugCF("agent.runner", "filtered tools", map[string]any{
		"before": len(catalog),
		"after":  len(filtered),
	})

	return filtered
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
