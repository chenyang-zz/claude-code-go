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

	"github.com/sheepzhao/claude-code-go/internal/core/compact"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
	runtimehooks "github.com/sheepzhao/claude-code-go/internal/runtime/hooks"
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
	// Hooks stores the hook configuration loaded from settings.
	Hooks hook.HooksConfig
	// DisableAllHooks disables all hook execution when set via policy settings.
	DisableAllHooks bool
	// HookRunner executes command hooks during the engine lifecycle.
	HookRunner HookRunner
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

	logger.DebugCF("engine", "starting single-turn run", map[string]any{
		"session_id":    req.SessionID,
		"message_count": len(history.Messages),
		"model":         e.DefaultModel,
	})

	out := make(chan event.Event)
	go func() {
		defer close(out)
		if err := e.runLoop(ctx, req.SessionID, req.CWD, req.TurnTokenBudget, history, out); err != nil {
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
func (e *Runtime) runLoop(ctx context.Context, sessionID string, cwd string, turnTokenBudget int, history conversation.History, out chan<- event.Event) error {
	e.sessionID = sessionID
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
	if turnTokenBudget > 0 {
		budgetTracker = NewBudgetTracker()
	}
	var stopHookActive bool

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
				// Replace message history with post-compact messages.
				history.Messages = append(
					[]message.Message(nil),
					compactResult.Boundary,
				)
				history.Messages = append(history.Messages, compactResult.SummaryMessages...)
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

		requestMessages := append([]message.Message(nil), history.Messages...)
		requestMessages = append(requestMessages, transientMessages...)
		streamReq := model.Request{
			Model:    activeModel,
			Messages: requestMessages,
			Tools:    e.ToolCatalog,
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
					history.Messages = append(
						[]message.Message(nil),
						compactResult.Boundary,
					)
					history.Messages = append(history.Messages, compactResult.SummaryMessages...)
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
		if result.activeModel != "" {
			activeModel = result.activeModel
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
			history.Append(result.assistant)
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
			if turnTokenBudget > 0 && result.stopReason != model.StopReasonMaxTokens {
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
							History: history.Clone(),
							Usage:   cumulativeUsage,
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
						history.Append(message.Message{
							Role: message.RoleUser,
							Content: []message.ContentPart{
								message.TextPart(stderr),
							},
						})
					}
					continue
				}
			}
			out <- event.Event{
				Type:      event.TypeConversationDone,
				Timestamp: time.Now(),
				Payload: event.ConversationDonePayload{
					History: history.Clone(),
					Usage:   cumulativeUsage,
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
		if turnTokenBudget > 0 {
			responseOutputTokens = 0
			budgetTracker = NewBudgetTracker()
		}
		logger.DebugCF("engine", "executing tool loop iteration", map[string]any{
			"session_id": sessionID,
			"tool_count": len(result.toolUses),
			"iteration":  toolLoops,
		})

		history.Append(e.executeToolUses(ctx, result.toolUses, out))
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
	assistant   message.Message
	toolUses    []model.ToolUse
	stopReason  model.StopReason
	usage       model.Usage
	activeModel string        // non-empty if fallback was triggered
	events      []event.Event // collected events, forwarded only on success
}

// streamAndConsume opens a model stream, consumes it, and handles both connection errors
// and mid-stream errors through the retry and fallback paths.
// MaxAttempts is the number of extra retries beyond the initial attempt (0 = single attempt, no retry).
// Partial events from failed attempts are discarded — only successful attempt events are forwarded.
func (e *Runtime) streamAndConsume(ctx context.Context, req model.Request, out chan<- event.Event) (streamResult, error) {
	policy := e.RetryPolicy
	retries := policy.MaxAttempts
	if retries < 0 {
		retries = 0
	}

	lastErr := error(nil)
	for attempt := 0; attempt <= retries; attempt++ {
		modelStream, connErr := e.Client.Stream(ctx, req)
		if connErr != nil {
			lastErr = connErr
			if !isRetriableError(connErr) {
				break
			}
			if attempt < retries {
				backoff := policy.backoffDuration(attempt + 1)
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

		result, streamErr := e.consumeModelStream(modelStream)
		if streamErr == nil {
			// Success — forward collected events to caller.
			for _, evt := range result.events {
				out <- evt
			}
			return result, nil
		}
		// Stream error — discard partial events, retry if retriable.
		if !isRetriableError(streamErr) {
			return streamResult{}, streamErr
		}
		lastErr = streamErr

		if attempt < retries {
			backoff := policy.backoffDuration(attempt + 1)
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
		fbResult, fbErr := e.consumeModelStream(fb.stream)
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
		fbResult.activeModel = fb.model
		return fbResult, nil
	}
	return streamResult{}, fmt.Errorf("stream failed after %d retries: %w", retries, lastErr)
}

// consumeModelStream aggregates one provider response into an assistant message plus any completed tool_use blocks.
// Events are collected into the result and NOT forwarded to any channel — the caller decides whether to emit them.
func (e *Runtime) consumeModelStream(modelStream model.Stream) (streamResult, error) {
	assistant := message.Message{Role: message.RoleAssistant}
	var toolUses []model.ToolUse
	var stopReason model.StopReason
	var usage model.Usage
	var events []event.Event

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
		}
	}

	return streamResult{
		assistant:  assistant,
		toolUses:   toolUses,
		stopReason: stopReason,
		usage:      usage,
		events:     events,
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
			result, invokeErr := e.executeToolUse(ctx, call, out)
			outcomes[index] = toolExecutionOutcome{
				toolUse:   pending,
				result:    result,
				invokeErr: invokeErr,
			}
		}(idx, toolUse)
	}
	wg.Wait()
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

	result, invokeErr := e.Executor.Execute(ctx, call)
	var permissionErr *corepermission.PermissionError
	if errors.As(invokeErr, &permissionErr) && permissionErr.Decision == corepermission.DecisionAsk && e.ApprovalService != nil {
		return e.executeFilesystemApproval(ctx, call, permissionErr, out)
	}

	var bashPermissionErr *corepermission.BashPermissionError
	if errors.As(invokeErr, &bashPermissionErr) && bashPermissionErr.Decision == corepermission.DecisionAsk && e.ApprovalService != nil {
		return e.executeBashApproval(ctx, call, bashPermissionErr, out)
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
		descriptions = append(descriptions, model.ToolDefinition{
			Name:        item.Name(),
			Description: item.Description(),
			InputSchema: item.InputSchema().JSONSchema(),
		})
	}
	return descriptions
}
