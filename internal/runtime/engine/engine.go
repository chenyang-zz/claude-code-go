package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/compact"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/core/transcript"
	"github.com/sheepzhao/claude-code-go/internal/platform/api/anthropic"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
	runtimehooks "github.com/sheepzhao/claude-code-go/internal/runtime/hooks"
	"github.com/sheepzhao/claude-code-go/internal/services/prompts"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Engine executes one conversation request and produces runtime events.
type Engine interface {
	Run(ctx context.Context, req conversation.RunRequest) (event.Stream, error)
}

// ToolExecutor resolves one normalized tool call for the runtime loop.
type ToolExecutor interface {
	Execute(ctx context.Context, call coretool.Call) (coretool.Result, error)
	// IsConcurrencySafe reports whether the named tool may run in parallel with other safe tools.
	IsConcurrencySafe(toolName string) bool
}

// HookRunner executes lifecycle hooks and returns the aggregated results.
type HookRunner interface {
	// RunStopHooks executes all command hooks for the given stop event.
	RunStopHooks(ctx context.Context, config hook.HooksConfig, event hook.HookEvent, input any, cwd string) []hook.HookResult
	// RunHooksForTool executes hooks filtered by tool name for tool lifecycle events.
	RunHooksForTool(ctx context.Context, config hook.HooksConfig, event hook.HookEvent, input any, cwd string, toolName string) []hook.HookResult
}

// Runtime is the minimum single-turn text engine used by batch-07.
type Runtime struct {
	// Client sends the provider request and returns a model event stream.
	Client model.Client
	// DefaultModel is used when the caller does not override the model.
	DefaultModel string
	// FallbackModel is an optional secondary model used when the primary model fails after exhausting retries.
	FallbackModel string
	// FallbackAfterAttempts is the number of retry attempts after which fallback is triggered.
	// Zero (default) means fallback only happens after all retries are exhausted.
	FallbackAfterAttempts int
	// RetryPolicy controls exponential backoff retry for transient provider errors.
	RetryPolicy RetryPolicy
	// ToolCatalog stores the provider-facing tool declarations attached to each request by default.
	ToolCatalog []model.ToolDefinition
	// Executor runs normalized tool invocations when the model emits tool_use blocks.
	Executor ToolExecutor
	// ApprovalService resolves runtime approval prompts for guarded tool operations.
	ApprovalService approval.Service
	// MaxToolIterations caps the number of tool-result feedback loops per runtime turn.
	MaxToolIterations int
	// MaxConcurrentToolCalls caps the number of concurrency-safe tool calls that may execute in parallel within one batch.
	MaxConcurrentToolCalls int
	// AutoCompact controls whether auto-compaction is enabled in the engine loop.
	// When true, the engine checks before each API request whether the conversation
	// token count exceeds the auto-compact threshold.
	AutoCompact bool
	// TranscriptPath is the optional file path for full conversation transcripts,
	// referenced in post-compact summary messages.
	TranscriptPath string
	// sessionID is set per-run and used by hook inputs.
	sessionID string
	// sessionStartSessionID tracks which logical session has already received
	// SessionStart hooks so reused Runtime instances still fire on new sessions.
	sessionStartSessionID string
	// Hooks stores the hook configuration loaded from settings.
	Hooks hook.HooksConfig
	// DisableAllHooks disables all hook execution when set via policy settings.
	DisableAllHooks bool
	// HookRunner executes command hooks during the engine lifecycle.
	HookRunner HookRunner
	// EnablePromptCaching tells the Anthropic provider to attach cache_control
	// markers on API requests. Default is true; it can be disabled via the
	// DISABLE_PROMPT_CACHING environment variable.
	EnablePromptCaching bool
	// CacheBreakDetector tracks prompt state and detects unexpected Anthropic
	// prompt cache breaks. When nil, cache break detection is disabled.
	CacheBreakDetector *anthropic.CacheBreakDetector
	// Source identifies the query source for cache break tracking
	// (e.g. "repl_main_thread", "sdk"). Empty means no tracking.
	Source string
	// AgentID isolates tracking state for subagents.
	AgentID string

	// --- OpenAI Responses API advanced parameters ---

	// DefaultTemperature controls sampling randomness when the caller does not
	// override it. When nil the provider default (1.0) is used.
	DefaultTemperature *float64

	// DefaultTopP controls nucleus sampling when the caller does not override it.
	// When nil the provider default (1.0) is used.
	DefaultTopP *float64

	// DefaultStore controls whether responses are stored server-side.
	// When nil the provider default applies.
	DefaultStore *bool

	// DefaultReasoningEffort controls reasoning behaviour for supported models.
	// Accepted values: "low", "medium", "high".
	DefaultReasoningEffort *string

	// DefaultToolChoice controls how the model selects tools.
	// Supported values: "auto", "none", "required", or "function:<name>".
	DefaultToolChoice *string

	// DefaultMetadata is a map of custom key-value pairs attached to every request.
	DefaultMetadata map[string]string

	// DefaultUser is the end-user identifier for monitoring and abuse detection.
	DefaultUser *string

	// DefaultInstructions is an alternative to System for the Responses API.
	DefaultInstructions *string

	// AgentRegistry holds agent definitions available for agent tool dispatch.
	// When nil, agent tool lookups fall back to a default empty registry.
	AgentRegistry agent.Registry

	// MainThreadAgentType stores the selected main-thread agent type from settings.
	MainThreadAgentType string

	// SessionConfig carries the current session configuration snapshot for dynamic prompt rendering.
	SessionConfig coretool.SessionConfigSnapshot

	// PromptBuilder generates the system prompt injected into each model request.
	// When nil, the System field is left empty.
	PromptBuilder *prompts.PromptBuilder
}

// New builds the minimum single-turn engine.
func New(client model.Client, defaultModel string, executor ToolExecutor, tools ...model.ToolDefinition) *Runtime {
	return &Runtime{
		Client:                 client,
		DefaultModel:           defaultModel,
		ToolCatalog:            append([]model.ToolDefinition(nil), tools...),
		Executor:               executor,
		RetryPolicy:            DefaultRetryPolicy(),
		MaxToolIterations:      8,
		MaxConcurrentToolCalls: 10,
	}
}

// Run converts a single input turn into a provider stream and maps it back into runtime events.
func (e *Runtime) Run(ctx context.Context, req conversation.RunRequest) (event.Stream, error) {
	history, err := buildInitialHistory(req)
	if err != nil {
		return nil, err
	}

	// When the TOKEN_BUDGET feature flag is enabled and no explicit budget was
	// provided, parse the user input for a token budget directive (e.g. "+500k").
	turnTokenBudget := req.TurnTokenBudget
	if turnTokenBudget <= 0 && featureflag.IsEnabled(featureflag.FlagTokenBudget) {
		if budget, ok := parseTokenBudgetFromHistory(history.Messages); ok {
			turnTokenBudget = budget
		}
	}

	logger.DebugCF("engine", "starting single-turn run", map[string]any{
		"session_id":    req.SessionID,
		"message_count": len(history.Messages),
		"model":         e.DefaultModel,
	})

	out := make(chan event.Event)
	go func() {
		defer close(out)
		if err := e.runLoop(ctx, req.SessionID, req.CWD, turnTokenBudget, req.SessionStartSource, history, out, req.System); err != nil {
			out <- event.Event{
				Type:      event.TypeError,
				Timestamp: time.Now(),
				Payload: event.ErrorPayload{
					Message: err.Error(),
				},
			}
		}

		logger.DebugCF("engine", "single-turn run finished", map[string]any{
			"session_id": req.SessionID,
		})
	}()

	return out, nil
}

// buildInitialHistory normalizes either an explicit message list or a raw user input into the first request history.
func buildInitialHistory(req conversation.RunRequest) (conversation.History, error) {
	if len(req.Messages) > 0 {
		history := conversation.History{Messages: make([]message.Message, len(req.Messages))}
		copy(history.Messages, req.Messages)
		return history, nil
	}

	input := strings.TrimSpace(req.Input)
	if input == "" {
		return conversation.History{}, fmt.Errorf("missing user input")
	}

	return conversation.History{
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					message.TextPart(input),
				},
			},
		},
	}, nil
}

// resolveSystemPrompt resolves the effective system prompt for one engine turn.
// It applies the explicit request override first, then any selected main-thread
// agent prompt, and finally falls back to the configured prompt builder.
func (e *Runtime) resolveSystemPrompt(ctx context.Context, sessionID, cwd, explicit string) string {
	resolved := strings.TrimSpace(explicit)
	if resolved != "" {
		return resolved
	}

	if agentPrompt := e.resolveMainThreadAgentPrompt(cwd); agentPrompt != "" {
		return agentPrompt
	}

	if e.PromptBuilder == nil {
		return ""
	}

	builtCtx := prompts.WithRuntimeContext(ctx, prompts.RuntimeContext{
		EnabledToolNames: buildEnabledToolNameSet(e.ToolCatalog),
		WorkingDir:       cwd,
		SessionID:        sessionID,
	})
	built, err := e.PromptBuilder.Build(builtCtx)
	if err != nil {
		logger.WarnCF("engine", "failed to build system prompt", map[string]any{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return ""
	}

	return strings.TrimSpace(built)
}

// resolveMainThreadAgentPrompt resolves the selected main-thread agent prompt
// when the runtime has been configured with a main-thread agent type.
func (e *Runtime) resolveMainThreadAgentPrompt(cwd string) string {
	agentType := strings.TrimSpace(e.MainThreadAgentType)
	if agentType == "" || e.AgentRegistry == nil {
		return ""
	}

	def, ok := e.AgentRegistry.Get(agentType)
	if !ok {
		logger.WarnCF("engine", "main-thread agent not found in registry", map[string]any{
			"agent_type": agentType,
		})
		return ""
	}

	if def.SystemPromptProvider != nil {
		return strings.TrimSpace(def.SystemPromptProvider.GetSystemPrompt(coretool.UseContext{WorkingDir: cwd}))
	}
	return strings.TrimSpace(def.SystemPrompt)
}

func parseTokenBudgetFromHistory(messages []message.Message) (int, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != message.RoleUser || isToolResultOnlyMessage(msg) {
			continue
		}
		for _, part := range msg.Content {
			if part.Type != "text" || part.IsMeta {
				continue
			}
			text := strings.TrimSpace(part.Text)
			if text == "" {
				continue
			}
			if budget, ok := ParseTokenBudget(text); ok {
				return budget, true
			}
		}
	}
	return 0, false
}

// findLatestContinuationUserMessage returns the most recent real user turn
// that should survive continuation recovery compaction.
func findLatestContinuationUserMessage(messages []message.Message) *message.Message {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != message.RoleUser || isToolResultOnlyMessage(msg) {
			continue
		}
		preserved := msg
		return &preserved
	}
	return nil
}

// findContinuationRecoveryTail returns the message slice that a max_tokens
// continuation needs to resume correctly after emergency compaction.
// It starts at the latest natural-language user turn and preserves the
// following tool loop / assistant context that the truncated reply depends on.
func findContinuationRecoveryTail(messages []message.Message) []message.Message {
	start := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == message.RoleUser && !isToolResultOnlyMessage(messages[i]) {
			start = i
			break
		}
	}
	if start == -1 {
		return nil
	}
	return append([]message.Message(nil), messages[start:]...)
}

// isToolResultOnlyMessage reports whether a user message contains only
// tool_result blocks and therefore is not a natural-language prompt turn.
func isToolResultOnlyMessage(msg message.Message) bool {
	if len(msg.Content) == 0 {
		return false
	}
	for _, part := range msg.Content {
		if part.Type == "text" && part.IsMeta {
			continue
		}
		if part.Type != "tool_result" {
			return false
		}
	}
	return true
}

// containsExactMessage reports whether messages already includes target with
// identical role and content so recovery does not duplicate preserved turns.
func containsExactMessage(messages []message.Message, target message.Message) bool {
	for _, msg := range messages {
		if msg.Role != target.Role {
			continue
		}
		if reflect.DeepEqual(msg.Content, target.Content) {
			return true
		}
	}
	return false
}

// appendMissingMessageTail appends the missing suffix of tail onto messages.
// This avoids duplicating preserved tail messages when compaction already
// retained part of the recovery context.
func appendMissingMessageTail(messages []message.Message, tail []message.Message) []message.Message {
	if len(tail) == 0 {
		return messages
	}

	maxOverlap := min(len(messages), len(tail))
	for overlap := maxOverlap; overlap > 0; overlap-- {
		if reflect.DeepEqual(messages[len(messages)-overlap:], tail[:overlap]) {
			return append(messages, tail[overlap:]...)
		}
	}
	return append(messages, tail...)
}

// maxContinuationAttempts caps how many times the engine will automatically
// continue generation when the model stops due to max_tokens (output truncated).
const maxContinuationAttempts = 3

// continuationUserMessage is injected into the conversation history after a
// truncated assistant response so the model picks up where it left off.
const continuationUserMessage = "Output token limit hit. Resume directly — no apology, no recap of what you were doing. Pick up mid-thought if that is where the cut happened. Break remaining work into smaller pieces."

// runLoop executes the minimal serial tool loop until the model returns plain text without new tool_use blocks.
func (e *Runtime) runLoop(ctx context.Context, sessionID string, cwd string, turnTokenBudget int, sessionStartSource string, history conversation.History, out chan<- event.Event, systemPrompt string) error {
	e.sessionID = sessionID
	e.resolveTranscriptPath(sessionID, cwd)

	transcriptWriter := e.openTranscriptWriter()
	if transcriptWriter != nil {
		defer func() {
			if err := transcriptWriter.Close(); err != nil {
				logger.WarnCF("engine", "failed to close transcript writer", map[string]any{
					"session_id": sessionID,
					"error":      err.Error(),
				})
			}
		}()
	}

	if latestUser := findLatestContinuationUserMessage(history.Messages); latestUser != nil {
		e.writeTranscriptMessage(transcriptWriter, *latestUser)
	}

	// Dispatch SessionStart hooks once per logical session ID (non-blocking).
	if e.sessionStartSessionID != sessionID {
		e.sessionStartSessionID = sessionID
		if e.shouldRunStopHooks(hook.EventSessionStart) {
			if strings.TrimSpace(sessionStartSource) == "" {
				sessionStartSource = "startup"
			}
			input := hook.SessionStartHookInput{
				BaseHookInput: hook.BaseHookInput{
					SessionID:      sessionID,
					TranscriptPath: e.TranscriptPath,
					CWD:            e.workingDir(cwd),
					AgentType:      "",
				},
				HookEventName: string(hook.EventSessionStart),
				Source:        sessionStartSource,
				Model:         e.activeModel(),
			}
			e.runHooks(ctx, hook.EventSessionStart, input, cwd)
		}
	}

	toolLoops := 0
	continuationAttempts := 0
	var cumulativeUsage model.Usage
	activeModel := e.activeModel()
	var compactTracking compact.TrackingState
	var hasAttemptedRecovery bool
	var transientMessages []message.Message
	// responseOutputTokens tracks only user-visible model output for the
	// current response phase. Internal compact-summary tokens are excluded
	// so budget continuation decisions reflect the answer, not recovery work.
	responseOutputTokens := 0
	var budgetTracker BudgetTracker
	if featureflag.IsEnabled(featureflag.FlagTokenBudget) && turnTokenBudget > 0 {
		budgetTracker = NewBudgetTracker()
	}
	var stopHookActive bool
	// lastResponseID carries the OpenAI Responses API response identifier
	// across model calls so that subsequent turns can use previous_response_id
	// for stateful conversation tracking.
	var lastResponseID string
	// taskBudgetRemaining tracks the remaining API-side task budget across
	// compaction boundaries. Undefined (0) until the first compact fires;
	// while context is uncompacted the server can see the full history and
	// handles the countdown from {total} itself. After compaction the server
	// sees only the summary and would under-count, so remaining tells it the
	// pre-compact window that was summarized away.
	taskBudgetRemaining := 0
	// hasTaskBudget tracks whether an API-side task_budget should be sent.
	hasTaskBudget := featureflag.IsEnabled(featureflag.FlagTokenBudget) && turnTokenBudget > 0

	for {
		// Auto-compact check point: before building the API request,
		// check if the conversation token count exceeds the threshold.
		// Skip auto-compact when transient continuation messages are pending:
		// the compactor only sees history.Messages and may summarize away the
		// truncated assistant turn that the continuation prompt refers to.
		// If the continuation request is too large, the emergency compact
		// path below will handle it.
		if e.AutoCompact && e.Client != nil && len(transientMessages) == 0 {
			compactResult, compactErr := compact.AutoCompactIfNeeded(
				ctx,
				e.Client,
				history.Messages,
				activeModel,
				&compactTracking,
				e.TranscriptPath,
			)
			if compactErr != nil {
				logger.DebugCF("engine", "auto-compact error (continuing)", map[string]any{
					"session_id": sessionID,
					"error":      compactErr.Error(),
				})
			}
			if compactResult != nil {
				if e.shouldRunStopHooks(hook.EventPreCompact) {
					preInput := hook.PreCompactHookInput{
						BaseHookInput: hook.BaseHookInput{
							SessionID:      sessionID,
							TranscriptPath: e.TranscriptPath,
							CWD:            e.workingDir(cwd),
						},
						HookEventName: string(hook.EventPreCompact),
						Trigger:       string(compact.TriggerAuto),
					}
					e.runHooks(ctx, hook.EventPreCompact, preInput, cwd)
				}

				// Replace message history with post-compact messages.
				history.Messages = append(
					[]message.Message(nil),
					compactResult.Boundary,
				)
				history.Messages = append(history.Messages, compactResult.SummaryMessages...)
				e.writeCompactTranscriptEntries(transcriptWriter, compactResult, string(compact.TriggerAuto))

				// Update task_budget remaining after compaction: the server's
				// budget countdown is context-based, so remaining decrements
				// by the pre-compact context window that got summarized away.
				if hasTaskBudget && !compactResult.Usage.IsZero() {
					taskBudgetRemaining = ComputeTaskBudgetRemaining(
						taskBudgetRemaining, turnTokenBudget,
						compactResult.Usage.InputTokens, compactResult.Usage.OutputTokens,
					)
				}

				// Dispatch PostCompact hooks after successful compaction (non-blocking).
				if e.shouldRunStopHooks(hook.EventPostCompact) {
					postInput := hook.PostCompactHookInput{
						BaseHookInput: hook.BaseHookInput{
							SessionID:      sessionID,
							TranscriptPath: e.TranscriptPath,
							CWD:            e.workingDir(cwd),
						},
						HookEventName:  string(hook.EventPostCompact),
						Trigger:        string(compact.TriggerAuto),
						CompactSummary: compactResult.Summary,
					}
					e.runHooks(ctx, hook.EventPostCompact, postInput, cwd)
				}

				out <- event.Event{
					Type:      event.TypeCompactDone,
					Timestamp: time.Now(),
					Payload: event.CompactDonePayload{
						PreTokenCount:  compactResult.PreTokenCount,
						PostTokenCount: compactResult.PostTokenCount,
					},
				}
				if !compactResult.Usage.IsZero() {
					cumulativeUsage = cumulativeUsage.Add(compactResult.Usage)
					out <- event.Event{
						Type:      event.TypeUsage,
						Timestamp: time.Now(),
						Payload: event.UsagePayload{
							TurnUsage:       compactResult.Usage,
							CumulativeUsage: cumulativeUsage,
						},
					}
				}
			}
		}
		// Notify cache break detector that compaction occurred.
		if e.CacheBreakDetector != nil {
			e.CacheBreakDetector.NotifyCompaction(e.Source, e.AgentID)
		}

		requestMessages := append([]message.Message(nil), history.Messages...)
		requestMessages = append(requestMessages, transientMessages...)
		streamReq := model.Request{
			Model:               activeModel,
			Messages:            requestMessages,
			Tools:               e.ToolCatalog,
			EnablePromptCaching: e.EnablePromptCaching,
		}
		resolvedSystemPrompt := e.resolveSystemPrompt(ctx, sessionID, cwd, systemPrompt)
		if resolvedSystemPrompt != "" {
			streamReq.System = resolvedSystemPrompt
		}
		if lastResponseID != "" {
			streamReq.PreviousResponseID = &lastResponseID
		}

		// Apply default advanced parameters from runtime configuration.
		if e.DefaultTemperature != nil {
			streamReq.Temperature = e.DefaultTemperature
		}
		if e.DefaultTopP != nil {
			streamReq.TopP = e.DefaultTopP
		}
		if e.DefaultStore != nil {
			streamReq.Store = e.DefaultStore
		}
		if e.DefaultReasoningEffort != nil {
			streamReq.ReasoningEffort = e.DefaultReasoningEffort
		}
		if e.DefaultToolChoice != nil {
			streamReq.ToolChoice = e.DefaultToolChoice
		}
		if len(e.DefaultMetadata) > 0 {
			streamReq.Metadata = e.DefaultMetadata
		}
		if e.DefaultUser != nil {
			streamReq.User = e.DefaultUser
		}
		if e.DefaultInstructions != nil {
			streamReq.Instructions = e.DefaultInstructions
		}

		// Attach API-side task_budget when the feature flag is enabled and
		// a token budget is active. The remaining field is only set after
		// the first compaction.
		if hasTaskBudget {
			var remaining *int
			if taskBudgetRemaining > 0 {
				r := taskBudgetRemaining
				remaining = &r
			}
			streamReq.TaskBudget = &model.TaskBudgetParam{
				Type:      "tokens",
				Total:     turnTokenBudget,
				Remaining: remaining,
			}
		}

		// Phase 1: Record prompt state before the API call for cache break detection.
		if e.CacheBreakDetector != nil {
			e.CacheBreakDetector.RecordPromptState(anthropic.PromptStateSnapshot{
				System:              streamReq.System,
				Tools:               streamReq.Tools,
				Source:              e.Source,
				Model:               streamReq.Model,
				AgentID:             e.AgentID,
				EnablePromptCaching: streamReq.EnablePromptCaching,
			})
		}

		result, err := e.streamAndConsume(ctx, streamReq, out)
		if err != nil {
			// When the API rejects the request because the prompt is too long,
			// attempt emergency compaction once and retry with the compressed history.
			if isPromptTooLongError(err) && !hasAttemptedRecovery && e.AutoCompact && e.Client != nil {
				hasAttemptedRecovery = true
				logger.DebugCF("engine", "prompt-too-long recovery: attempting emergency compact", map[string]any{
					"session_id": sessionID,
				})
				// Preserve the last assistant message (the truncated one)
				// so it survives compact and the model sees the exact
				// cut-off point when resuming.
				recoveryTail := findContinuationRecoveryTail(history.Messages)
				latestRecoverableUser := findLatestContinuationUserMessage(history.Messages)
				originalLastMessage := history.Messages[len(history.Messages)-1]

				compactResult, compactErr := compact.CompactConversation(ctx, e.Client, compact.CompactRequest{
					// Use history.Messages (which includes the truncated
					// assistant but NOT the synthetic continuation prompt)
					// so the compactor preserves the real last user message.
					// The continuation prompt in transientMessages is added
					// back when the next request is built.
					Messages:       history.Messages,
					Model:          activeModel,
					TranscriptPath: e.TranscriptPath,
				})
				if compactErr == nil && compactResult != nil {
					if e.shouldRunStopHooks(hook.EventPreCompact) {
						preInput := hook.PreCompactHookInput{
							BaseHookInput: hook.BaseHookInput{
								SessionID:      sessionID,
								TranscriptPath: e.TranscriptPath,
								CWD:            e.workingDir(cwd),
							},
							HookEventName: string(hook.EventPreCompact),
							Trigger:       string(compact.TriggerAuto),
						}
						e.runHooks(ctx, hook.EventPreCompact, preInput, cwd)
					}

					// Dispatch PostCompact hooks after emergency compaction (non-blocking).
					if e.shouldRunStopHooks(hook.EventPostCompact) {
						postInput := hook.PostCompactHookInput{
							BaseHookInput: hook.BaseHookInput{
								SessionID:      sessionID,
								TranscriptPath: e.TranscriptPath,
								CWD:            e.workingDir(cwd),
							},
							HookEventName:  string(hook.EventPostCompact),
							Trigger:        string(compact.TriggerAuto),
							CompactSummary: compactResult.Summary,
						}
						e.runHooks(ctx, hook.EventPostCompact, postInput, cwd)
					}

					history.Messages = append(
						[]message.Message(nil),
						compactResult.Boundary,
					)
					history.Messages = append(history.Messages, compactResult.SummaryMessages...)
					e.writeCompactTranscriptEntries(transcriptWriter, compactResult, string(compact.TriggerAuto))

					// Update task_budget remaining after emergency compaction
					// (same carryover as the proactive path above).
					if hasTaskBudget && !compactResult.Usage.IsZero() {
						taskBudgetRemaining = ComputeTaskBudgetRemaining(
							taskBudgetRemaining, turnTokenBudget,
							compactResult.Usage.InputTokens, compactResult.Usage.OutputTokens,
						)
					}

					// Re-append the truncated assistant turn only when this
					// is a max_tokens continuation that hit prompt-too-long.
					// Without transientMessages this is a plain oversized
					// request and the compactor already preserves the
					// trailing user prompt — re-appending the last assistant
					// would corrupt message ordering.
					if len(transientMessages) > 0 {
						history.Messages = appendMissingMessageTail(history.Messages, recoveryTail)
					} else if latestRecoverableUser != nil &&
						originalLastMessage.Role == message.RoleUser &&
						!isToolResultOnlyMessage(originalLastMessage) &&
						!containsExactMessage(history.Messages, *latestRecoverableUser) {
						// CompactConversation strips media blocks before summarizing,
						// so a preserved multimodal tail user turn may survive only
						// as placeholder text. Replace that compacted tail with the
						// original user message before retrying.
						if len(history.Messages) > 0 {
							lastHistoryMessage := history.Messages[len(history.Messages)-1]
							if lastHistoryMessage.Role == message.RoleUser && !isToolResultOnlyMessage(lastHistoryMessage) {
								history.Messages = history.Messages[:len(history.Messages)-1]
							}
						}
						history.Messages = append(history.Messages, *latestRecoverableUser)
					}
					// Do NOT clear transientMessages here: the continuation
					// instruction must survive emergency compact so the retry
					// still asks the model to resume mid-thought.
					out <- event.Event{
						Type:      event.TypeCompactDone,
						Timestamp: time.Now(),
						Payload: event.CompactDonePayload{
							PreTokenCount:  compactResult.PreTokenCount,
							PostTokenCount: compactResult.PostTokenCount,
						},
					}
					if !compactResult.Usage.IsZero() {
						cumulativeUsage = cumulativeUsage.Add(compactResult.Usage)
						out <- event.Event{
							Type:      event.TypeUsage,
							Timestamp: time.Now(),
							Payload: event.UsagePayload{
								TurnUsage:       compactResult.Usage,
								CumulativeUsage: cumulativeUsage,
							},
						}
					}
					// Notify cache break detector that emergency compaction occurred.
					if e.CacheBreakDetector != nil {
						e.CacheBreakDetector.NotifyCompaction(e.Source, e.AgentID)
					}
					continue
				}
				logger.DebugCF("engine", "prompt-too-long recovery: compact failed, surfacing error", map[string]any{
					"session_id":    sessionID,
					"compact_error": compactErr,
				})
			}
			return err
		}
		transientMessages = nil
		// A successful model call means the prompt fit within the context
		// window, so allow a future prompt-too-long recovery if the
		// conversation grows again.
		hasAttemptedRecovery = false

		// Phase 2: Check for cache break using the API response's cache tokens.
		if e.CacheBreakDetector != nil && result.usage.CacheReadInputTokens > 0 {
			e.CacheBreakDetector.CheckResponseForCacheBreak(
				e.Source,
				result.usage.CacheReadInputTokens,
				result.usage.CacheCreationInputTokens,
				requestMessages,
				e.AgentID,
				"",
			)
		}
		if result.activeModel != "" {
			activeModel = result.activeModel
		}
		// Carry forward the Responses API response ID for stateful tracking.
		if result.responseID != "" {
			lastResponseID = result.responseID
		}

		// Accumulate usage from this model call.
		cumulativeUsage = cumulativeUsage.Add(result.usage)
		responseOutputTokens += result.usage.OutputTokens

		// Emit per-call usage event.
		if !result.usage.IsZero() {
			out <- event.Event{
				Type:      event.TypeUsage,
				Timestamp: time.Now(),
				Payload: event.UsagePayload{
					TurnUsage:       result.usage,
					CumulativeUsage: cumulativeUsage,
					StopReason:      string(result.stopReason),
				},
			}
		}

		if len(result.assistant.Content) > 0 {
			e.appendHistoryWithTranscript(&history, result.assistant, transcriptWriter)
		}
		if len(result.toolUses) == 0 {
			// When the model stops due to max_tokens (output truncated),
			// automatically continue by injecting a recovery user message
			// and sending another request so the model picks up where it left off.
			if result.stopReason == model.StopReasonMaxTokens && continuationAttempts < maxContinuationAttempts {
				continuationAttempts++
				logger.DebugCF("engine", "max_tokens continuation", map[string]any{
					"session_id":           sessionID,
					"continuation_attempt": continuationAttempts,
					"max_continuation":     maxContinuationAttempts,
				})
				// The truncated assistant message was already appended above.
				// Inject a continuation prompt only into the next model request so
				// internal recovery instructions are not persisted into session history.
				transientMessages = []message.Message{{
					Role: message.RoleUser,
					Content: []message.ContentPart{
						message.TextPart(continuationUserMessage),
					},
				}}
				continue
			}
			// Token budget continuation: when the model stops normally
			// (not max_tokens) and a token budget is set, check whether
			// the model should be nudged to keep producing tokens.
			// Gated by the TOKEN_BUDGET feature flag.
			if featureflag.IsEnabled(featureflag.FlagTokenBudget) && turnTokenBudget > 0 && result.stopReason != model.StopReasonMaxTokens {
				decision := checkTokenBudget(&budgetTracker, "", turnTokenBudget, responseOutputTokens)
				if decision.Action == "continue" {
					logger.DebugCF("engine", "token budget continuation", map[string]any{
						"session_id":         sessionID,
						"continuation_count": decision.ContinuationCount,
						"pct":                decision.Pct,
						"turn_output_tokens": responseOutputTokens,
						"turn_token_budget":  turnTokenBudget,
					})
					transientMessages = []message.Message{{
						Role: message.RoleUser,
						Content: []message.ContentPart{
							message.TextPart(decision.NudgeMessage),
						},
					}}
					continue
				}
				if decision.CompletionEvent != nil {
					logger.DebugCF("engine", "token budget completed", map[string]any{
						"session_id":          sessionID,
						"pct":                 decision.CompletionEvent.Pct,
						"tokens":              decision.CompletionEvent.Tokens,
						"diminishing_returns": decision.CompletionEvent.DiminishingReturns,
					})
				}
			}
			// Execute stop hooks before emitting ConversationDone.
			// If a stop hook returns exit code 2 (blocking), the error
			// message is injected into the conversation and the loop
			// continues with stopHookActive=true so hooks can detect
			// re-entry and avoid infinite recursion.
			stopEvent := hook.EventStop
			if e.shouldRunStopHooks(stopEvent) {
				lastAssistantText := extractLastAssistantText(history.Messages)
				input := hook.StopHookInput{
					BaseHookInput: hook.BaseHookInput{
						SessionID:      sessionID,
						TranscriptPath: e.TranscriptPath,
						CWD:            e.workingDir(cwd),
					},
					HookEventName:        string(stopEvent),
					StopHookActive:       stopHookActive,
					LastAssistantMessage: lastAssistantText,
				}
				results := e.runStopHooks(ctx, stopEvent, input, cwd)
				// Check for preventContinuation: if any hook returned
				// stdout JSON with continue:false, terminate immediately.
				if hasPreventContinuation(results) {
					logger.DebugCF("engine", "stop hook preventContinuation", map[string]any{
						"session_id": sessionID,
					})
					out <- event.Event{
						Type:      event.TypeConversationDone,
						Timestamp: time.Now(),
						Payload: event.ConversationDonePayload{
							History:    history.Clone(),
							Usage:      cumulativeUsage,
							StopReason: string(result.stopReason),
						},
					}
					return nil
				}
				if len(results) > 0 && hasBlockingHookResult(results) {
					blockingStderrs := blockingStderrMessages(results)
					logger.DebugCF("engine", "stop hook blocking, continuing conversation", map[string]any{
						"session_id":     sessionID,
						"blocking_count": len(blockingStderrs),
					})
					stopHookActive = true
					for _, stderr := range blockingStderrs {
						e.appendHistoryWithTranscript(&history, message.Message{
							Role: message.RoleUser,
							Content: []message.ContentPart{
								message.TextPart(stderr),
							},
						}, transcriptWriter)
					}
					continue
				}
			}
			out <- event.Event{
				Type:      event.TypeConversationDone,
				Timestamp: time.Now(),
				Payload: event.ConversationDonePayload{
					History:    history.Clone(),
					Usage:      cumulativeUsage,
					StopReason: string(result.stopReason),
				},
			}
			return nil
		}
		if e.Executor == nil {
			return fmt.Errorf("tool loop requested but executor is not configured")
		}
		if toolLoops >= e.maxToolIterations() {
			return fmt.Errorf("tool loop exceeded max iterations (%d)", e.maxToolIterations())
		}

		toolLoops++
		// Reset continuation budget for the next response phase:
		// the model will generate a fresh answer after tool results
		// are injected, so a new truncation chain may begin.
		continuationAttempts = 0
		// Reset budget tracker for the next response phase: the model
		// starts a fresh generation after tool results, so budget state
		// from the previous response phase should not carry over.
		if featureflag.IsEnabled(featureflag.FlagTokenBudget) && turnTokenBudget > 0 {
			responseOutputTokens = 0
			budgetTracker = NewBudgetTracker()
		}
		logger.DebugCF("engine", "executing tool loop iteration", map[string]any{
			"session_id": sessionID,
			"tool_count": len(result.toolUses),
			"iteration":  toolLoops,
		})

		// When the streaming executor was used, tools already ran during
		// streaming. Build the tool_result message from its tracked state
		// instead of executing tools again.
		if result.streamingExec != nil {
			e.appendHistoryWithTranscript(&history, result.streamingExec.BuildToolResultMessage(), transcriptWriter)
		} else {
			e.appendHistoryWithTranscript(&history, e.executeToolUses(ctx, result.toolUses, out), transcriptWriter)
		}
	}
}

// buildEnabledToolNameSet converts the provider-facing tool catalog into a
// quick lookup set for prompt sections that need runtime tool availability.
func buildEnabledToolNameSet(toolDefs []model.ToolDefinition) map[string]struct{} {
	if len(toolDefs) == 0 {
		return nil
	}

	names := make(map[string]struct{}, len(toolDefs))
	for _, def := range toolDefs {
		if strings.TrimSpace(def.Name) == "" {
			continue
		}
		names[def.Name] = struct{}{}
	}
	return names
}

// resolveTranscriptPath lazily computes the transcript file path when runtime
// wiring did not inject one explicitly.
func (e *Runtime) resolveTranscriptPath(sessionID string, cwd string) {
	if e == nil || strings.TrimSpace(e.TranscriptPath) != "" {
		return
	}
	trimmedSessionID := strings.TrimSpace(sessionID)
	trimmedCWD := strings.TrimSpace(cwd)
	if trimmedSessionID == "" || trimmedCWD == "" {
		return
	}
	e.TranscriptPath = transcript.GetTranscriptPath(trimmedSessionID, trimmedCWD)
}

// openTranscriptWriter opens the configured transcript path and returns nil when
// transcript persistence is unavailable.
func (e *Runtime) openTranscriptWriter() *transcript.Writer {
	if e == nil {
		return nil
	}
	path := strings.TrimSpace(e.TranscriptPath)
	if path == "" {
		return nil
	}
	writer, err := transcript.NewWriter(path)
	if err != nil {
		logger.WarnCF("engine", "failed to open transcript writer", map[string]any{
			"path":  path,
			"error": err.Error(),
		})
		return nil
	}
	return writer
}

// appendHistoryWithTranscript appends a message to runtime history and mirrors it
// into transcript JSONL entries.
func (e *Runtime) appendHistoryWithTranscript(history *conversation.History, msg message.Message, writer *transcript.Writer) {
	if history == nil {
		return
	}
	history.Append(msg)
	e.writeTranscriptMessage(writer, msg)
}

// writeTranscriptMessage writes transcript entries derived from one normalized
// conversation message. Write failures are logged and ignored.
func (e *Runtime) writeTranscriptMessage(writer *transcript.Writer, msg message.Message) {
	if writer == nil {
		return
	}
	timestamp := time.Now().UTC()
	for _, entry := range transcript.EntriesFromMessage(timestamp, msg) {
		if err := writer.WriteEntry(entry); err != nil {
			logger.WarnCF("engine", "failed to write transcript message entry", map[string]any{
				"type":  fmt.Sprintf("%T", entry),
				"error": err.Error(),
			})
			return
		}
	}
}

// writeCompactTranscriptEntries writes the summary + compact boundary records
// emitted by a successful compaction.
func (e *Runtime) writeCompactTranscriptEntries(writer *transcript.Writer, result *compact.CompactionResult, trigger string) {
	if writer == nil || result == nil {
		return
	}
	timestamp := time.Now().UTC()
	if err := writer.WriteEntry(transcript.NewSummaryEntry(timestamp, result.Summary)); err != nil {
		logger.WarnCF("engine", "failed to write compact summary transcript entry", map[string]any{
			"error": err.Error(),
		})
		return
	}
	if err := writer.WriteEntry(transcript.NewCompactBoundaryEntry(timestamp, trigger, result.PreTokenCount, result.PostTokenCount)); err != nil {
		logger.WarnCF("engine", "failed to write compact boundary transcript entry", map[string]any{
			"error": err.Error(),
		})
	}
}

// shouldRunStopHooks reports whether stop hooks should execute for the given event.
func (e *Runtime) shouldRunStopHooks(event hook.HookEvent) bool {
	if e == nil {
		return false
	}
	if e.DisableAllHooks {
		return false
	}
	if e.HookRunner == nil {
		return false
	}
	return e.Hooks.HasEvent(event)
}

// shouldRunToolHooks reports whether tool-level hooks should execute for the given event.
func (e *Runtime) shouldRunToolHooks(event hook.HookEvent) bool {
	return e.shouldRunStopHooks(event)
}

// runStopHooks executes stop hooks for the given event via the configured runner.
func (e *Runtime) runStopHooks(ctx context.Context, event hook.HookEvent, input hook.StopHookInput, cwd string) []hook.HookResult {
	if e.HookRunner == nil {
		return nil
	}
	return e.HookRunner.RunStopHooks(ctx, e.Hooks, event, input, e.workingDir(cwd))
}

// runHooks executes hooks for the given event with arbitrary input type.
// Unlike runStopHooks, it accepts any hook input struct (not just StopHookInput).
func (e *Runtime) runHooks(ctx context.Context, event hook.HookEvent, input any, cwd string) []hook.HookResult {
	if e.HookRunner == nil {
		return nil
	}
	return e.HookRunner.RunStopHooks(ctx, e.Hooks, event, input, e.workingDir(cwd))
}

// RunSessionEndHooks dispatches SessionEnd hooks. This is a non-blocking event
// intended to be called by the REPL/app layer when the session terminates.
func (e *Runtime) RunSessionEndHooks(ctx context.Context, reason string, cwd string) {
	if !e.shouldRunStopHooks(hook.EventSessionEnd) {
		return
	}
	input := hook.SessionEndHookInput{
		BaseHookInput: hook.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.TranscriptPath,
			CWD:            e.workingDir(cwd),
		},
		HookEventName: string(hook.EventSessionEnd),
		Reason:        reason,
	}
	e.runHooks(ctx, hook.EventSessionEnd, input, cwd)
}

// RunNotificationHooks dispatches Notification hooks. This is a non-blocking,
// fire-and-forget event intended to be called from notification senders.
func (e *Runtime) RunNotificationHooks(ctx context.Context, message string, title string, notificationType string, cwd string) {
	if !e.shouldRunStopHooks(hook.EventNotification) {
		return
	}
	input := hook.NotificationHookInput{
		BaseHookInput: hook.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.TranscriptPath,
			CWD:            e.workingDir(cwd),
		},
		HookEventName:    string(hook.EventNotification),
		Message:          message,
		Title:            title,
		NotificationType: notificationType,
	}
	e.runHooks(ctx, hook.EventNotification, input, cwd)
}

// workingDir returns the working directory used for hook execution context.
func (e *Runtime) workingDir(requestCWD string) string {
	if e == nil {
		return ""
	}
	if trimmed := strings.TrimSpace(requestCWD); trimmed != "" {
		return trimmed
	}
	if transcriptPath := strings.TrimSpace(e.TranscriptPath); transcriptPath != "" {
		return filepath.Dir(transcriptPath)
	}
	return ""
}

// extractLastAssistantText returns the text content of the last assistant message.
func extractLastAssistantText(messages []message.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == message.RoleAssistant {
			var parts []string
			for _, part := range messages[i].Content {
				if part.Type == "text" && part.Text != "" {
					parts = append(parts, part.Text)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "\n")
			}
		}
	}
	return ""
}

// hasBlockingHookResult reports whether any hook result indicates a blocking error (exit code 2).
func hasBlockingHookResult(results []hook.HookResult) bool {
	for _, r := range results {
		if r.IsBlocking() {
			return true
		}
	}
	return false
}

// blockingStderrMessages collects stderr from all blocking hook results (exit code 2).
func blockingStderrMessages(results []hook.HookResult) []string {
	var msgs []string
	for _, r := range results {
		if r.IsBlocking() && strings.TrimSpace(r.Stderr) != "" {
			msgs = append(msgs, r.Stderr)
		}
	}
	return msgs
}

// hasPreventContinuation reports whether any hook result requests conversation termination.
func hasPreventContinuation(results []hook.HookResult) bool {
	for _, r := range results {
		if r.PreventContinuation {
			return true
		}
	}
	return false
}

// activeModel returns the configured model, falling back to DefaultModel.
func (e *Runtime) activeModel() string {
	if e != nil && e.DefaultModel != "" {
		return e.DefaultModel
	}
	return "claude-sonnet-4-20250514"
}

// streamResult holds the aggregated output from one model stream consumption.
type streamResult struct {
	assistant     message.Message
	toolUses      []model.ToolUse
	stopReason    model.StopReason
	usage         model.Usage
	activeModel   string                 // non-empty if fallback was triggered
	events        []event.Event          // collected events, forwarded only on success
	streamingExec *StreamingToolExecutor // non-nil when streaming tool execution was used
	responseID    string                 // OpenAI Responses API response identifier
}

// streamAndConsume opens a model stream, consumes it, and handles both connection errors
// and mid-stream errors through the retry and fallback paths.
// MaxAttempts is the number of extra retries beyond the initial attempt (0 = single attempt, no retry).
// Partial events from failed attempts are discarded — only successful attempt events are forwarded.
// When a ToolExecutor is configured, a StreamingToolExecutor is created so tools begin
// executing as soon as their tool_use blocks arrive during streaming.
func (e *Runtime) streamAndConsume(ctx context.Context, req model.Request, out chan<- event.Event) (streamResult, error) {
	policy := e.RetryPolicy
	retries := policy.MaxAttempts
	if retries < 0 {
		retries = 0
	}

	// Create a streaming tool executor if a tool executor is configured.
	// The streaming executor starts tool invocations immediately when
	// complete tool_use blocks are detected during stream consumption.
	var streamingExec *StreamingToolExecutor
	if e.Executor != nil {
		streamingExec = e.newStreamingExecutor(ctx, out)
	}

	lastErr := error(nil)
	for attempt := 0; attempt <= retries; attempt++ {
		modelStream, connErr := e.Client.Stream(ctx, req)
		if connErr != nil {
			lastErr = connErr
			if !isRetriableError(connErr) {
				break
			}
			// Check if FallbackAfterAttempts triggers before retry.
			if e.shouldFallbackAfterAttempts(attempt + 1) {
				if fb := e.tryFallback(ctx, req, lastErr); fb != nil {
					return e.runFallback(ctx, req, fb, streamingExec, out)
				}
			}
			if attempt < retries {
				backoff := e.computeBackoff(connErr, attempt+1, policy)
				out <- event.Event{
					Type:      event.TypeRetryAttempted,
					Timestamp: time.Now(),
					Payload: event.RetryAttemptedPayload{
						Attempt:     attempt + 1,
						MaxAttempts: retries,
						BackoffMs:   backoff.Milliseconds(),
						Error:       connErr.Error(),
					},
				}
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return streamResult{}, ctx.Err()
				}
			}
			continue
		}

		result, streamErr := e.consumeModelStream(modelStream, streamingExec, ctx)
		if streamErr == nil {
			// Success — forward collected events to caller.
			for _, evt := range result.events {
				out <- evt
			}
			// Only start tool execution after the provider stream succeeds so
			// discarded retry/fallback attempts cannot run tools.
			if streamingExec != nil {
				for _, toolUse := range result.toolUses {
					streamingExec.AddTool(ctx, toolUse)
				}
				for _, evt := range streamingExec.AwaitAll(ctx) {
					out <- evt
				}
			}
			return result, nil
		}
		// Stream error — discard partial events and running tools, retry if retriable.
		if streamingExec != nil {
			streamingExec.Discard()
			streamingExec = e.newStreamingExecutor(ctx, out)
		}
		if !isRetriableError(streamErr) {
			return streamResult{}, streamErr
		}
		lastErr = streamErr

		// Check if FallbackAfterAttempts triggers before retry.
		if e.shouldFallbackAfterAttempts(attempt + 1) {
			if fb := e.tryFallback(ctx, req, lastErr); fb != nil {
				return e.runFallback(ctx, req, fb, streamingExec, out)
			}
		}

		if attempt < retries {
			backoff := e.computeBackoff(streamErr, attempt+1, policy)
			out <- event.Event{
				Type:      event.TypeRetryAttempted,
				Timestamp: time.Now(),
				Payload: event.RetryAttemptedPayload{
					Attempt:     attempt + 1,
					MaxAttempts: retries,
					BackoffMs:   backoff.Milliseconds(),
					Error:       streamErr.Error(),
				},
			}
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return streamResult{}, ctx.Err()
			}
		}
	}

	// All retries exhausted — try fallback.
	if fb := e.tryFallback(ctx, req, lastErr); fb != nil {
		return e.runFallback(ctx, req, fb, streamingExec, out)
	}
	return streamResult{}, fmt.Errorf("stream failed after %d retries: %w", retries, lastErr)
}

// computeBackoff determines the backoff duration for a retry attempt.
// If the error implements model.RetryableError and provides a positive RetryAfter,
// that value is used; otherwise the policy's exponential backoff is used.
func (e *Runtime) computeBackoff(err error, attempt int, policy RetryPolicy) time.Duration {
	var retryable model.RetryableError
	if errors.As(err, &retryable) {
		if d := retryable.RetryAfter(); d > 0 {
			return d
		}
	}
	return policy.backoffDuration(attempt)
}

// runFallback executes a fallback model attempt and returns the result.
func (e *Runtime) runFallback(ctx context.Context, req model.Request, fb *fallbackResult, streamingExec *StreamingToolExecutor, out chan<- event.Event) (streamResult, error) {
	// Create a fresh streaming executor for the fallback attempt.
	if streamingExec != nil {
		streamingExec.Discard()
		streamingExec = e.newStreamingExecutor(ctx, out)
	}
	fbResult, fbErr := e.consumeModelStream(fb.stream, streamingExec, ctx)
	if fbErr != nil {
		return streamResult{}, fbErr
	}
	out <- event.Event{
		Type:      event.TypeModelFallback,
		Timestamp: time.Now(),
		Payload: event.ModelFallbackPayload{
			OriginalModel: req.Model,
			FallbackModel: fb.model,
		},
	}
	for _, evt := range fbResult.events {
		out <- evt
	}
	if streamingExec != nil {
		for _, toolUse := range fbResult.toolUses {
			streamingExec.AddTool(ctx, toolUse)
		}
		for _, evt := range streamingExec.AwaitAll(ctx) {
			out <- evt
		}
	}
	fbResult.activeModel = fb.model
	return fbResult, nil
}

// newStreamingExecutor creates a StreamingToolExecutor wired to this runtime's tool execution pipeline.
// The ctx parameter is used as the parent for the sibling cascade context.
func (e *Runtime) newStreamingExecutor(ctx context.Context, out chan<- event.Event) *StreamingToolExecutor {
	return NewStreamingToolExecutor(
		ctx,
		func(ctx context.Context, call coretool.Call, evCh chan<- event.Event) (coretool.Result, error) {
			return e.executeToolUse(ctx, call, evCh)
		},
		func(toolName string) bool {
			return e.Executor.IsConcurrencySafe(toolName)
		},
		out,
		e.maxConcurrentToolCalls(),
	)
}

// consumeModelStream aggregates one provider response into an assistant message plus any completed tool_use blocks.
// Events are collected into the result and NOT forwarded to any channel — the caller decides whether to emit them.
// Tool execution is deferred until the full stream succeeds so retried attempts cannot run discarded tools.
func (e *Runtime) consumeModelStream(modelStream model.Stream, streamingExec *StreamingToolExecutor, ctx context.Context) (streamResult, error) {
	assistant := message.Message{Role: message.RoleAssistant}
	var toolUses []model.ToolUse
	var stopReason model.StopReason
	var usage model.Usage
	var events []event.Event
	var responseID string

	for item := range modelStream {
		switch item.Type {
		case model.EventTypeTextDelta:
			assistant.Content = append(assistant.Content, message.TextPart(item.Text))
			events = append(events, event.Event{
				Type:      event.TypeMessageDelta,
				Timestamp: time.Now(),
				Payload: event.MessageDeltaPayload{
					Text: item.Text,
				},
			})
		case model.EventTypeError:
			errMsg := item.Error
			// Drain remaining events asynchronously so retries are not blocked on the provider closing the stream.
			go func() {
				for range modelStream {
				}
			}()
			// Discard collected events — this attempt failed.
			return streamResult{}, errors.New(errMsg)
		case model.EventTypeThinking:
			assistant.Content = append(assistant.Content, message.ThinkingPart(item.Thinking, item.Signature))
			events = append(events, event.Event{
				Type:      event.TypeThinking,
				Timestamp: time.Now(),
				Payload: event.ThinkingPayload{
					Thinking:  item.Thinking,
					Signature: item.Signature,
				},
			})
		case model.EventTypeToolUse:
			if item.ToolUse == nil {
				return streamResult{}, fmt.Errorf("tool use event missing payload")
			}
			toolUses = append(toolUses, *item.ToolUse)
			assistant.Content = append(assistant.Content, message.ToolUsePart(item.ToolUse.ID, item.ToolUse.Name, item.ToolUse.Input))
			events = append(events, event.Event{
				Type:      event.TypeToolCallStarted,
				Timestamp: time.Now(),
				Payload: event.ToolCallPayload{
					ID:    item.ToolUse.ID,
					Name:  item.ToolUse.Name,
					Input: item.ToolUse.Input,
				},
			})
		case model.EventTypeDone:
			stopReason = item.StopReason
			if item.Usage != nil {
				usage = *item.Usage
			}
			if item.ResponseID != "" {
				responseID = item.ResponseID
			}
		}
	}

	return streamResult{
		assistant:     assistant,
		toolUses:      toolUses,
		stopReason:    stopReason,
		usage:         usage,
		events:        events,
		streamingExec: streamingExec,
		responseID:    responseID,
	}, nil
}

// executeToolUses resolves one tool batch sequence and converts the results into one tool_result message.
func (e *Runtime) executeToolUses(ctx context.Context, toolUses []model.ToolUse, out chan<- event.Event) message.Message {
	resultMessage := message.Message{Role: message.RoleUser}

	batches := partitionToolUses(toolUses, e.Executor)
	for _, batch := range batches {
		outcomes := e.executeToolBatch(ctx, batch, out)
		for _, outcome := range outcomes {
			additionalContext := toolAdditionalContext(outcome.result)
			if additionalContext != "" {
				resultMessage.Content = append(resultMessage.Content, message.MetaTextPart(additionalContext))
			}
			content, isError := renderToolResult(outcome.result, outcome.invokeErr)
			resultMessage.Content = append(resultMessage.Content, message.ToolResultPart(outcome.toolUse.ID, content, isError))
			out <- event.Event{
				Type:      event.TypeToolCallFinished,
				Timestamp: time.Now(),
				Payload: event.ToolResultPayload{
					ID:                outcome.toolUse.ID,
					Name:              outcome.toolUse.Name,
					Output:            content,
					AdditionalContext: additionalContext,
					IsError:           isError,
				},
			}
		}
	}

	return resultMessage
}

// executeToolBatch resolves one partitioned batch either serially or with bounded concurrency.
func (e *Runtime) executeToolBatch(ctx context.Context, batch toolExecutionBatch, out chan<- event.Event) []toolExecutionOutcome {
	if len(batch.toolUses) == 0 {
		return nil
	}
	if !batch.concurrencySafe || len(batch.toolUses) == 1 {
		outcomes := make([]toolExecutionOutcome, 0, len(batch.toolUses))
		for _, toolUse := range batch.toolUses {
			call := coretool.Call{
				ID:     toolUse.ID,
				Name:   toolUse.Name,
				Input:  toolUse.Input,
				Source: "model",
			}
			result, invokeErr := e.executeToolUse(ctx, call, out)
			outcomes = append(outcomes, toolExecutionOutcome{
				toolUse:   toolUse,
				result:    result,
				invokeErr: invokeErr,
			})
		}
		return outcomes
	}

	logger.DebugCF("engine", "executing concurrency-safe tool batch", map[string]any{
		"tool_count":      len(batch.toolUses),
		"max_concurrency": e.maxConcurrentToolCalls(),
	})

	// Create a sibling cascade so that a Bash tool error cancels sibling tools.
	// The cascade context is derived from the parent ctx; cancelling it does not
	// affect the parent, matching the TS siblingAbortController design.
	cascade := NewSiblingCascade(ctx)

	outcomes := make([]toolExecutionOutcome, len(batch.toolUses))
	sem := make(chan struct{}, e.maxConcurrentToolCalls())
	var wg sync.WaitGroup
	for idx, toolUse := range batch.toolUses {
		wg.Add(1)
		sem <- struct{}{}

		go func(index int, pending model.ToolUse) {
			defer wg.Done()
			defer func() { <-sem }()

			call := coretool.Call{
				ID:     pending.ID,
				Name:   pending.Name,
				Input:  pending.Input,
				Source: "model",
			}
			// Use cascade context for execution so sibling cancellation propagates.
			result, invokeErr := e.executeToolUse(cascade.Context(), call, out)

			// Trigger cascade if this is a Bash tool error.
			cascade.TriggerOnBashError(pending.Name, pending.Input, result, invokeErr)

			outcomes[index] = toolExecutionOutcome{
				toolUse:   pending,
				result:    result,
				invokeErr: invokeErr,
			}
		}(idx, toolUse)
	}
	wg.Wait()

	// Replace context.Canceled errors from sibling cascade with synthetic error messages.
	// The Bash tool that triggered the cascade keeps its real error; only siblings that
	// were cancelled by the cascade context get the synthetic message.
	if cascade.IsErrored() {
		desc := cascade.ErroredToolDesc()
		for i := range outcomes {
			if errors.Is(outcomes[i].invokeErr, context.Canceled) {
				syntheticMsg := FormatCascadeErrorMessage(desc)
				outcomes[i].result = coretool.Result{Error: syntheticMsg}
				outcomes[i].invokeErr = nil
			}
		}
	}

	return outcomes
}

// executeToolUse resolves one tool call and branches into the approval flow when the tool is blocked by a permission ask.
func (e *Runtime) executeToolUse(ctx context.Context, call coretool.Call, out chan<- event.Event) (coretool.Result, error) {
	var additionalContext string

	// PreToolUse hooks: run before tool execution, blocking prevents the call.
	if e.shouldRunToolHooks(hook.EventPreToolUse) {
		toolInput, _ := json.Marshal(call.Input)
		input := hook.PreToolHookInput{
			BaseHookInput: hook.BaseHookInput{
				SessionID:      e.sessionID,
				TranscriptPath: e.TranscriptPath,
				CWD:            e.workingDir(""),
			},
			HookEventName: string(hook.EventPreToolUse),
			ToolName:      call.Name,
			ToolInput:     toolInput,
			ToolUseID:     call.ID,
		}
		results := e.HookRunner.RunHooksForTool(ctx, e.Hooks, hook.EventPreToolUse, input, e.workingDir(""), call.Name)
		if runtimehooks.HasBlockingResult(results) {
			errs := runtimehooks.BlockingErrors(results)
			return coretool.Result{Error: strings.Join(errs, "\n")}, nil
		}
		if runtimehooks.HasErrorResult(results) {
			for _, msg := range runtimehooks.ErrorMessages(results) {
				logger.DebugCF("engine", "pre-tool hook error", map[string]any{
					"tool":   call.Name,
					"error":  msg,
					"use_id": call.ID,
				})
			}
		}

		// Resolve structured permission decisions from hook output.
		permResult := hook.ResolvePreToolUsePermission(results)
		additionalContext = permResult.AdditionalContext
		if permResult.Behavior == hook.PermissionDeny {
			return attachAdditionalContext(coretool.Result{Error: permResult.DenyReason}, additionalContext), nil
		}
		// Apply updatedInput if provided by the hook.
		if len(permResult.UpdatedInput) > 0 {
			if err := json.Unmarshal(permResult.UpdatedInput, &call.Input); err != nil {
				logger.DebugCF("engine", "failed to apply hook updatedInput", map[string]any{
					"tool":   call.Name,
					"error":  err.Error(),
					"use_id": call.ID,
				})
			}
		}
		if permResult.Behavior == hook.PermissionAsk {
			proceed, approvalResult, err := e.requestHookApproval(ctx, call, permResult.Reason, out)
			if err != nil {
				return coretool.Result{}, err
			}
			if !proceed {
				return attachAdditionalContext(approvalResult, additionalContext), nil
			}
		}
	}

	// Inject progress callback so tools can emit incremental progress events.
	toolCtx := coretool.WithProgress(ctx, func(data any) {
		out <- event.Event{
			Type:      event.TypeProgress,
			Timestamp: time.Now(),
			Payload: event.ProgressPayload{
				ToolUseID:       call.ID,
				ParentToolUseID: "",
				Data:            data,
			},
		}
	})

	// Agent tool dispatch placeholder: when call.Name matches the agent tool,
	// the input can be unmarshaled into agent.Input and the output is agent.Output.
	// Full agent runtime execution (runAgent, subagent scheduling) is deferred
	// to a later batch that wires the agent lifecycle into the engine loop.
	result, invokeErr := e.Executor.Execute(toolCtx, call)
	var permissionErr *corepermission.PermissionError
	if errors.As(invokeErr, &permissionErr) && permissionErr.Decision == corepermission.DecisionAsk && e.ApprovalService != nil {
		return e.executeFilesystemApproval(ctx, call, permissionErr, out)
	}

	var bashPermissionErr *corepermission.BashPermissionError
	if errors.As(invokeErr, &bashPermissionErr) && bashPermissionErr.Decision == corepermission.DecisionAsk && e.ApprovalService != nil {
		return e.executeBashApproval(ctx, call, bashPermissionErr, out)
	}

	var webFetchPermissionErr *corepermission.WebFetchPermissionError
	if errors.As(invokeErr, &webFetchPermissionErr) && webFetchPermissionErr.Decision == corepermission.DecisionAsk && e.ApprovalService != nil {
		return e.executeWebFetchApproval(ctx, call, webFetchPermissionErr, out)
	}

	if invokeErr == nil && strings.TrimSpace(result.Error) == "" {
		// PostToolUse hooks: run after successful tool execution.
		if e.shouldRunToolHooks(hook.EventPostToolUse) {
			toolInput, _ := json.Marshal(call.Input)
			toolResponse, _ := json.Marshal(result.Output)
			input := hook.PostToolHookInput{
				BaseHookInput: hook.BaseHookInput{
					SessionID:      e.sessionID,
					TranscriptPath: e.TranscriptPath,
					CWD:            e.workingDir(""),
				},
				HookEventName: string(hook.EventPostToolUse),
				ToolName:      call.Name,
				ToolInput:     toolInput,
				ToolResponse:  toolResponse,
				ToolUseID:     call.ID,
			}
			results := e.HookRunner.RunHooksForTool(ctx, e.Hooks, hook.EventPostToolUse, input, e.workingDir(""), call.Name)
			if runtimehooks.HasBlockingResult(results) {
				errs := runtimehooks.BlockingErrors(results)
				return attachAdditionalContext(coretool.Result{Error: strings.Join(errs, "\n")}, additionalContext), nil
			}
		}
		return attachAdditionalContext(result, additionalContext), nil
	}

	if e.shouldRunToolHooks(hook.EventPostToolUseFailure) {
		toolInput, _ := json.Marshal(call.Input)
		toolResponse, _ := json.Marshal(result.Output)
		input := hook.PostToolFailureHookInput{
			BaseHookInput: hook.BaseHookInput{
				SessionID:      e.sessionID,
				TranscriptPath: e.TranscriptPath,
				CWD:            e.workingDir(""),
			},
			HookEventName: string(hook.EventPostToolUseFailure),
			ToolName:      call.Name,
			ToolInput:     toolInput,
			ToolResponse:  toolResponse,
			Error:         renderToolFailure(result, invokeErr),
			IsInterrupt:   isToolInterrupt(result, invokeErr),
			ToolUseID:     call.ID,
		}
		results := e.HookRunner.RunHooksForTool(ctx, e.Hooks, hook.EventPostToolUseFailure, input, e.workingDir(""), call.Name)
		if runtimehooks.HasBlockingResult(results) {
			errs := runtimehooks.BlockingErrors(results)
			return attachAdditionalContext(coretool.Result{Error: strings.Join(errs, "\n")}, additionalContext), nil
		}
	}

	if invokeErr == nil {
		return attachAdditionalContext(result, additionalContext), nil
	}

	return attachAdditionalContext(result, additionalContext), invokeErr
}

const additionalContextMetaKey = "additional_context"

func attachAdditionalContext(result coretool.Result, additionalContext string) coretool.Result {
	if strings.TrimSpace(additionalContext) == "" {
		return result
	}
	if result.Meta == nil {
		result.Meta = make(map[string]any, 1)
	}
	result.Meta[additionalContextMetaKey] = additionalContext
	return result
}

func toolAdditionalContext(result coretool.Result) string {
	if result.Meta == nil {
		return ""
	}
	additionalContext, _ := result.Meta[additionalContextMetaKey].(string)
	return additionalContext
}

func (e *Runtime) requestHookApproval(ctx context.Context, call coretool.Call, reason string, out chan<- event.Event) (bool, coretool.Result, error) {
	message := fmt.Sprintf("A PreToolUse hook requested approval before executing %s.", call.Name)
	if strings.TrimSpace(reason) != "" {
		message = fmt.Sprintf("%s %s", message, strings.TrimSpace(reason))
	}

	if e.ApprovalService == nil {
		return false, coretool.Result{Error: "Approval service is not interactive in the current mode."}, nil
	}

	out <- event.Event{
		Type:      event.TypeApprovalRequired,
		Timestamp: time.Now(),
		Payload: event.ApprovalPayload{
			CallID:   call.ID,
			ToolName: call.Name,
			Path:     call.Name,
			Action:   "execute",
			Message:  message,
		},
	}

	decision, err := e.ApprovalService.Decide(ctx, approval.Request{
		CallID:   call.ID,
		ToolName: call.Name,
		Path:     call.Name,
		Action:   "execute",
		Message:  message,
	})
	if err != nil {
		return false, coretool.Result{}, err
	}
	if !decision.Approved {
		reason := strings.TrimSpace(decision.Reason)
		if reason == "" {
			reason = fmt.Sprintf("Permission to execute %s was not granted.", call.Name)
		}
		return false, coretool.Result{Error: reason}, nil
	}

	return true, coretool.Result{}, nil
}

// executeFilesystemApproval resolves one filesystem approval request through the runtime prompt and one-shot retry flow.
func (e *Runtime) executeFilesystemApproval(ctx context.Context, call coretool.Call, permissionErr *corepermission.PermissionError, out chan<- event.Event) (coretool.Result, error) {
	out <- event.Event{
		Type:      event.TypeApprovalRequired,
		Timestamp: time.Now(),
		Payload: event.ApprovalPayload{
			CallID:   call.ID,
			ToolName: call.Name,
			Path:     permissionErr.Path,
			Action:   string(permissionErr.Access),
			Message:  permissionErr.Message,
		},
	}

	decision, err := e.ApprovalService.Decide(ctx, approval.Request{
		CallID:   call.ID,
		ToolName: call.Name,
		Path:     permissionErr.Path,
		Action:   string(permissionErr.Access),
		Message:  permissionErr.Message,
	})
	if err != nil {
		return coretool.Result{}, err
	}
	if !decision.Approved {
		if strings.TrimSpace(decision.Reason) == "" {
			decision.Reason = fmt.Sprintf("Permission to %s %s was not granted.", permissionErr.Access, permissionErr.Path)
		}
		return coretool.Result{Error: decision.Reason}, nil
	}

	retryCtx := corepermission.WithFilesystemGrant(ctx, corepermission.FilesystemRequest{
		ToolName:   call.Name,
		Path:       permissionErr.Path,
		WorkingDir: call.Context.WorkingDir,
		Access:     permissionErr.Access,
	})
	return e.Executor.Execute(retryCtx, call)
}

// executeBashApproval resolves one Bash approval request through the runtime prompt and one-shot retry flow.
func (e *Runtime) executeBashApproval(ctx context.Context, call coretool.Call, permissionErr *corepermission.BashPermissionError, out chan<- event.Event) (coretool.Result, error) {
	out <- event.Event{
		Type:      event.TypeApprovalRequired,
		Timestamp: time.Now(),
		Payload: event.ApprovalPayload{
			CallID:   call.ID,
			ToolName: call.Name,
			Path:     permissionErr.Command,
			Action:   "execute",
			Message:  permissionErr.Message,
		},
	}

	decision, err := e.ApprovalService.Decide(ctx, approval.Request{
		CallID:   call.ID,
		ToolName: call.Name,
		Path:     permissionErr.Command,
		Action:   "execute",
		Message:  permissionErr.Message,
	})
	if err != nil {
		return coretool.Result{}, err
	}
	if !decision.Approved {
		if strings.TrimSpace(decision.Reason) == "" {
			decision.Reason = fmt.Sprintf("Permission to execute %q was not granted.", permissionErr.Command)
		}
		return coretool.Result{Error: decision.Reason}, nil
	}

	retryCtx := corepermission.WithBashGrant(ctx, corepermission.BashRequest{
		ToolName:   call.Name,
		Command:    permissionErr.Command,
		WorkingDir: call.Context.WorkingDir,
	})
	return e.Executor.Execute(retryCtx, call)
}

// executeWebFetchApproval resolves one WebFetch approval request through the runtime prompt and one-shot retry flow.
func (e *Runtime) executeWebFetchApproval(ctx context.Context, call coretool.Call, permissionErr *corepermission.WebFetchPermissionError, out chan<- event.Event) (coretool.Result, error) {
	out <- event.Event{
		Type:      event.TypeApprovalRequired,
		Timestamp: time.Now(),
		Payload: event.ApprovalPayload{
			CallID:   call.ID,
			ToolName: call.Name,
			Path:     permissionErr.URL,
			Action:   "fetch",
			Message:  permissionErr.Message,
		},
	}

	decision, err := e.ApprovalService.Decide(ctx, approval.Request{
		CallID:   call.ID,
		ToolName: call.Name,
		Path:     permissionErr.URL,
		Action:   "fetch",
		Message:  permissionErr.Message,
	})
	if err != nil {
		return coretool.Result{}, err
	}
	if !decision.Approved {
		if strings.TrimSpace(decision.Reason) == "" {
			decision.Reason = fmt.Sprintf("Permission to fetch %q was not granted.", permissionErr.URL)
		}
		return coretool.Result{Error: decision.Reason}, nil
	}

	retryCtx := corepermission.WithWebFetchGrant(ctx, call.Name, permissionErr.URL, call.Context.WorkingDir)
	return e.Executor.Execute(retryCtx, call)
}

// renderToolResult normalizes executor success and failure paths into the minimal tool_result payload understood by the model.
func renderToolResult(result coretool.Result, invokeErr error) (string, bool) {
	if invokeErr != nil {
		return renderToolFailure(result, invokeErr), true
	}
	if strings.TrimSpace(result.Error) != "" {
		return result.Error, true
	}
	return result.Output, false
}

func renderToolFailure(result coretool.Result, invokeErr error) string {
	if strings.TrimSpace(result.Error) != "" {
		return result.Error
	}
	if invokeErr != nil {
		return invokeErr.Error()
	}
	return ""
}

func isToolInterrupt(result coretool.Result, invokeErr error) bool {
	if errors.Is(invokeErr, context.Canceled) {
		return true
	}
	return strings.Contains(renderToolFailure(result, invokeErr), "AbortError")
}

// maxToolIterations returns the configured loop cap, falling back to the default minimum when unset.
func (e *Runtime) maxToolIterations() int {
	if e == nil || e.MaxToolIterations <= 0 {
		return 8
	}
	return e.MaxToolIterations
}

// maxConcurrentToolCalls returns the configured per-batch concurrency cap for safe tool calls.
func (e *Runtime) maxConcurrentToolCalls() int {
	if e == nil || e.MaxConcurrentToolCalls <= 0 {
		return 10
	}
	return e.MaxConcurrentToolCalls
}

// DescribeTools converts a tool registry into provider-facing tool definitions.
func DescribeTools(registry coretool.Registry) []model.ToolDefinition {
	if registry == nil {
		return nil
	}

	registered := registry.List()
	descriptions := make([]model.ToolDefinition, 0, len(registered))
	for _, item := range registered {
		if item == nil {
			continue
		}
		// Skip tools that declare themselves disabled via IsEnabled().
		if gated, ok := item.(interface{ IsEnabled() bool }); ok && !gated.IsEnabled() {
			continue
		}
		descriptions = append(descriptions, model.ToolDefinition{
			Name:        item.Name(),
			Description: item.Description(),
			InputSchema: item.InputSchema().JSONSchema(),
		})
	}
	return descriptions
}
