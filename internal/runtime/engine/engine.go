package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/compact"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
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
		if err := e.runLoop(ctx, req.SessionID, history, out); err != nil {
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

// runLoop executes the minimal serial tool loop until the model returns plain text without new tool_use blocks.
func (e *Runtime) runLoop(ctx context.Context, sessionID string, history conversation.History, out chan<- event.Event) error {
	toolLoops := 0
	var cumulativeUsage model.Usage
	activeModel := e.activeModel()
	var compactTracking compact.TrackingState

	for {
		// Auto-compact check point: before building the API request,
		// check if the conversation token count exceeds the threshold.
		if e.AutoCompact && e.Client != nil {
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

		streamReq := model.Request{
			Model:    activeModel,
			Messages: history.Messages,
			Tools:    e.ToolCatalog,
		}

		result, err := e.streamAndConsume(ctx, streamReq, out)
		if err != nil {
			return err
		}
		if result.activeModel != "" {
			activeModel = result.activeModel
		}

		// Accumulate usage from this model call.
		cumulativeUsage = cumulativeUsage.Add(result.usage)

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
		logger.DebugCF("engine", "executing tool loop iteration", map[string]any{
			"session_id": sessionID,
			"tool_count": len(result.toolUses),
			"iteration":  toolLoops,
		})

		history.Append(e.executeToolUses(ctx, result.toolUses, out))
	}
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
			content, isError := renderToolResult(outcome.result, outcome.invokeErr)
			resultMessage.Content = append(resultMessage.Content, message.ToolResultPart(outcome.toolUse.ID, content, isError))
			out <- event.Event{
				Type:      event.TypeToolCallFinished,
				Timestamp: time.Now(),
				Payload: event.ToolResultPayload{
					ID:      outcome.toolUse.ID,
					Name:    outcome.toolUse.Name,
					Output:  content,
					IsError: isError,
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
	result, invokeErr := e.Executor.Execute(ctx, call)
	if invokeErr == nil {
		return result, nil
	}

	var permissionErr *corepermission.PermissionError
	if errors.As(invokeErr, &permissionErr) && permissionErr.Decision == corepermission.DecisionAsk && e.ApprovalService != nil {
		return e.executeFilesystemApproval(ctx, call, permissionErr, out)
	}

	var bashPermissionErr *corepermission.BashPermissionError
	if errors.As(invokeErr, &bashPermissionErr) && bashPermissionErr.Decision == corepermission.DecisionAsk && e.ApprovalService != nil {
		return e.executeBashApproval(ctx, call, bashPermissionErr, out)
	}

	return result, invokeErr
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
		if strings.TrimSpace(result.Error) != "" {
			return result.Error, true
		}
		return invokeErr.Error(), true
	}
	if strings.TrimSpace(result.Error) != "" {
		return result.Error, true
	}
	return result.Output, false
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
