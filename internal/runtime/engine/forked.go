package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/core/transcript"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// CanUseToolFn reports whether the named tool is allowed for a forked agent.
// When the function returns false, the tool call is skipped and a denial
// message is returned to the model.
type CanUseToolFn func(toolName string) bool

// AllowAllTools is a CanUseToolFn that permits every tool.
func AllowAllTools(_ string) bool { return true }

// DenyAllTools is a CanUseToolFn that denies every tool.
func DenyAllTools(_ string) bool { return false }

// CacheSafeParams carries the parameters that must be identical between the
// fork and parent API requests to share the parent's prompt cache. The
// Anthropic API cache key is composed of: system prompt, tools, model,
// messages (prefix), and thinking config.
type CacheSafeParams struct {
	// SystemPrompt is the resolved system prompt for the request.
	SystemPrompt string
	// SystemContext carries additional context appended to the system prompt.
	SystemContext map[string]string
	// Messages carries the conversation history for cache key matching.
	Messages []message.Message
	// Runtime provides the model client, tool catalog, and configuration.
	Runtime *Runtime
}

var (
	lastCacheSafeParamsMu sync.RWMutex
	lastCacheSafeParams   *CacheSafeParams
)

// SaveCacheSafeParams stores the given params as the latest cache-safe
// snapshot. Pass nil to clear the stored state.
func SaveCacheSafeParams(params *CacheSafeParams) {
	lastCacheSafeParamsMu.Lock()
	defer lastCacheSafeParamsMu.Unlock()
	lastCacheSafeParams = params
}

// GetLastCacheSafeParams returns the most recently saved cache-safe params,
// or nil if none has been saved.
func GetLastCacheSafeParams() *CacheSafeParams {
	lastCacheSafeParamsMu.RLock()
	defer lastCacheSafeParamsMu.RUnlock()
	return lastCacheSafeParams
}

// ForkedAgentParams configures a forked agent execution.
type ForkedAgentParams struct {
	// PromptMessages are the initial messages for the forked query loop.
	PromptMessages []message.Message
	// CacheSafeParams carries the cache-critical parameters from the parent.
	CacheSafeParams CacheSafeParams
	// CanUseTool controls which tools the forked agent may invoke.
	CanUseTool CanUseToolFn
	// ForkLabel identifies this fork for logging (e.g. "agent_summary").
	ForkLabel string
	// MaxOutputTokens optionally caps the output token count.
	MaxOutputTokens int
	// MaxTurns optionally caps the number of API round-trips.
	MaxTurns int
	// SkipTranscript disables sidechain transcript recording when true.
	SkipTranscript bool
}

// ForkedAgentResult carries the output of a forked agent execution.
type ForkedAgentResult struct {
	// Messages contains all messages yielded during the query loop.
	Messages []message.Message
	// TotalUsage accumulates usage across all API calls in the loop.
	TotalUsage model.Usage
}

// SubagentContextOverrides controls how the forked agent's Runtime is
// isolated from the parent. By default, all mutable state is isolated.
type SubagentContextOverrides struct {
	// Model optionally overrides the model used by the forked agent.
	Model string
	// ToolCatalog optionally overrides the tool definitions.
	ToolCatalog []model.ToolDefinition
}

// newAgentID generates a random hex string suitable for use as an agent ID.
func newAgentID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateSubagentContext creates an isolated Runtime for a forked agent.
// The fork shares the parent's model client but gets its own tool catalog
// and configuration. The parent's PromptBuilder, Hooks, and Executor are
// preserved so the fork can use the same tools and hook configuration.
func CreateSubagentContext(parent *Runtime, overrides *SubagentContextOverrides) *Runtime {
	if overrides == nil {
		overrides = &SubagentContextOverrides{}
	}

	// Start with a copy of the parent's essential fields.
	forked := &Runtime{
		Client:                 parent.Client,
		DefaultModel:           parent.DefaultModel,
		FallbackModel:          parent.FallbackModel,
		FallbackClients:        parent.FallbackClients,
		FallbackAfterAttempts:  parent.FallbackAfterAttempts,
		RetryPolicy:            parent.RetryPolicy,
		StatsCollector:         parent.StatsCollector,
		ToolCatalog:            append([]model.ToolDefinition(nil), parent.ToolCatalog...),
		Executor:               parent.Executor,
		ApprovalService:        parent.ApprovalService,
		MaxToolIterations:      parent.MaxToolIterations,
		MaxConcurrentToolCalls: parent.MaxConcurrentToolCalls,
		EnablePromptCaching:    parent.EnablePromptCaching,
		Hooks:                  parent.Hooks,
		DisableAllHooks:        parent.DisableAllHooks,
		HookRunner:             parent.HookRunner,
		PromptBuilder:          parent.PromptBuilder,
		AgentRegistry:          parent.AgentRegistry,
		SessionConfig:          parent.SessionConfig,
	}

	// Apply overrides.
	if overrides.Model != "" {
		forked.DefaultModel = overrides.Model
	}
	if len(overrides.ToolCatalog) > 0 {
		forked.ToolCatalog = append([]model.ToolDefinition(nil), overrides.ToolCatalog...)
	}

	// Disable auto-compact for forked agents to avoid interference.
	forked.AutoCompact = false

	return forked
}

// RunForked executes a forked agent query loop with usage tracking.
// It reuses the parent's model client but operates on an isolated message
// history and accumulates its own usage metrics.
func RunForked(ctx context.Context, params ForkedAgentParams) (*ForkedAgentResult, error) {
	startTime := time.Now()
	forkLabel := params.ForkLabel
	if forkLabel == "" {
		forkLabel = "unknown"
	}

	cacheParams := params.CacheSafeParams
	runtime := cacheParams.Runtime
	if runtime == nil {
		return nil, fmt.Errorf("forked agent [%s]: runtime is nil", forkLabel)
	}
	if runtime.Client == nil {
		return nil, fmt.Errorf("forked agent [%s]: model client is nil", forkLabel)
	}

	// Create isolated Runtime for this fork.
	forkedRuntime := CreateSubagentContext(runtime, nil)

	// Build initial messages: parent context + prompt messages.
	initialMessages := append([]message.Message(nil), cacheParams.Messages...)
	initialMessages = append(initialMessages, params.PromptMessages...)

	var totalUsage model.Usage
	var outputMessages []message.Message

	// Transcript recording (unless skipped).
	var transcriptWriter *transcript.Writer
	agentID := newAgentID()
	if !params.SkipTranscript && runtime.TranscriptPath != "" {
		sidechainPath := runtime.TranscriptPath + "." + agentID + ".jsonl"
		tw, err := transcript.NewWriter(sidechainPath)
		if err != nil {
			logger.WarnCF("engine", "failed to create sidechain transcript writer", map[string]any{
				"fork_label": forkLabel,
				"error":      err.Error(),
			})
		} else {
			transcriptWriter = tw
			defer func() {
				if err := transcriptWriter.Close(); err != nil {
					logger.WarnCF("engine", "failed to close sidechain transcript", map[string]any{
						"fork_label": forkLabel,
						"error":      err.Error(),
					})
				}
			}()
		}
	}

	turns := 0
	history := conversation.History{Messages: append([]message.Message(nil), initialMessages...)}

	for {
		// Check max turns limit.
		if params.MaxTurns > 0 && turns >= params.MaxTurns {
			logger.DebugCF("engine", "forked agent max turns reached", map[string]any{
				"fork_label": forkLabel,
				"turns":      turns,
			})
			break
		}

		// Build the model request.
		streamReq := model.Request{
			Model:               forkedRuntime.DefaultModel,
			Messages:            history.Messages,
			Tools:               forkedRuntime.ToolCatalog,
			EnablePromptCaching: forkedRuntime.EnablePromptCaching,
		}
		if cacheParams.SystemPrompt != "" {
			streamReq.System = cacheParams.SystemPrompt
		}
		if params.MaxOutputTokens > 0 {
			streamReq.MaxOutputTokens = params.MaxOutputTokens
		}

		// Stream the model response.
		stream, err := forkedRuntime.Client.Stream(ctx, streamReq)
		if err != nil {
			return nil, fmt.Errorf("forked agent [%s] stream error: %w", forkLabel, err)
		}

		// Consume the stream events.
		var assistantContent []message.ContentPart
		var turnUsage model.Usage
		var stopReason string

		for evt := range stream {
			switch evt.Type {
			case model.EventTypeTextDelta:
				assistantContent = append(assistantContent, message.TextPart(evt.Text))
			case model.EventTypeThinking:
				assistantContent = append(assistantContent, message.ThinkingPart(evt.Thinking, evt.Signature))
			case model.EventTypeToolUse:
				if evt.ToolUse != nil {
					assistantContent = append(assistantContent, message.ToolUsePart(evt.ToolUse.ID, evt.ToolUse.Name, evt.ToolUse.Input))
				}
			case model.EventTypeDone:
				if evt.Usage != nil {
					turnUsage = *evt.Usage
				}
				stopReason = string(evt.StopReason)
			case model.EventTypeError:
				return nil, fmt.Errorf("forked agent [%s] stream event error: %s", forkLabel, evt.Error)
			}
		}

		// Accumulate usage.
		totalUsage = totalUsage.Add(turnUsage)
		turns++

		// Build and append the assistant message.
		if len(assistantContent) > 0 {
			assistantMsg := message.Message{
				Role:    message.RoleAssistant,
				Content: assistantContent,
			}
			history.Messages = append(history.Messages, assistantMsg)
			outputMessages = append(outputMessages, assistantMsg)

			// Write to transcript if available.
			if transcriptWriter != nil {
				_ = transcriptWriter.WriteEntry(assistantMsg)
			}
		}

		// If the model finished without tool use, we're done.
		if stopReason == string(model.StopReasonEndTurn) || stopReason == string(model.StopReasonStopSequence) {
			// Check if there are tool_use blocks that need responses.
			hasToolUse := false
			for _, part := range assistantContent {
				if part.ToolName != "" {
					hasToolUse = true
					break
				}
			}
			if !hasToolUse {
				break
			}
		}

		// Process tool calls if any.
		var toolResults []message.ContentPart
		for _, part := range assistantContent {
			if part.ToolName == "" {
				continue
			}
			toolName := part.ToolName
			toolID := part.ToolUseID

			// Check permission via CanUseTool.
			if params.CanUseTool != nil && !params.CanUseTool(toolName) {
				toolResults = append(toolResults, message.ToolResultPart(toolID, "Tool not available in this context", true))
				continue
			}

			// Execute the tool if we have an executor.
			if forkedRuntime.Executor != nil {
				call := tool.Call{
					ID:    toolID,
					Name:  toolName,
					Input: part.ToolInput,
				}
				result, execErr := forkedRuntime.Executor.Execute(ctx, call)
				if execErr != nil {
					toolResults = append(toolResults, message.ToolResultPart(toolID, execErr.Error(), true))
				} else if result.Error != "" {
					toolResults = append(toolResults, message.ToolResultPart(toolID, result.Error, true))
				} else {
					toolResults = append(toolResults, message.ToolResultPart(toolID, result.Output, false))
				}
			} else {
				toolResults = append(toolResults, message.ToolResultPart(toolID, "No tool executor available", true))
			}
		}

		// Append tool results to history if any.
		if len(toolResults) > 0 {
			toolResultMsg := message.Message{
				Role:    message.RoleUser,
				Content: toolResults,
			}
			history.Messages = append(history.Messages, toolResultMsg)

			if transcriptWriter != nil {
				_ = transcriptWriter.WriteEntry(toolResultMsg)
			}
		} else if stopReason != string(model.StopReasonToolUse) {
			// No tool results and model is done.
			break
		}
	}

	durationMs := time.Since(startTime).Milliseconds()
	logger.DebugCF("engine", "forked agent finished", map[string]any{
		"fork_label":    forkLabel,
		"duration_ms":   durationMs,
		"message_count": len(outputMessages),
		"input_tokens":  totalUsage.InputTokens,
		"output_tokens": totalUsage.OutputTokens,
		"turns":         turns,
	})

	return &ForkedAgentResult{
		Messages:  outputMessages,
		TotalUsage: totalUsage,
	}, nil
}

// CreateCacheSafeParams extracts the cache-critical parameters from a Runtime
// and message history for use in forked agent execution.
func CreateCacheSafeParams(runtime *Runtime, systemPrompt string, systemContext map[string]string, messages []message.Message) CacheSafeParams {
	return CacheSafeParams{
		SystemPrompt: systemPrompt,
		SystemContext: systemContext,
		Messages:     append([]message.Message(nil), messages...),
		Runtime:      runtime,
	}
}

// ExtractResultText extracts the text content from the last assistant message
// in the given messages. Returns defaultText if no assistant message is found.
func ExtractResultText(messages []message.Message, defaultText string) string {
	if defaultText == "" {
		defaultText = "Execution completed"
	}

	// Find the last assistant message.
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == message.RoleAssistant {
			for _, part := range messages[i].Content {
				if part.Text != "" {
					return part.Text
				}
			}
		}
	}

	return defaultText
}
