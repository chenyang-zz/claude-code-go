package engine

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// streamingToolStatus tracks the lifecycle of one tool call during streaming execution.
type streamingToolStatus string

const (
	// streamingToolQueued means the tool has been detected but is waiting for execution.
	streamingToolQueued streamingToolStatus = "queued"
	// streamingToolExecuting means the tool is currently running.
	streamingToolExecuting streamingToolStatus = "executing"
	// streamingToolCompleted means the tool has finished and results are available.
	streamingToolCompleted streamingToolStatus = "completed"
	// streamingToolYielded means the results have already been collected by the caller.
	streamingToolYielded streamingToolStatus = "yielded"
)

// streamingTrackedTool holds the execution state for one tool call detected during streaming.
type streamingTrackedTool struct {
	// id is the unique tool call identifier from the provider.
	id string
	// toolUse holds the original tool_use block received from the model stream.
	toolUse model.ToolUse
	// status tracks the current execution phase.
	status streamingToolStatus
	// isConcurrencySafe indicates whether this tool may run in parallel with other safe tools.
	isConcurrencySafe bool
	// result stores the raw tool result returned by the executor callback.
	result coretool.Result
	// invokeErr stores any execution error from the tool invocation.
	invokeErr error
	// events stores the rendered tool result events (TypeToolCallFinished).
	events []event.Event
	// done is closed when the tool execution finishes (success or failure).
	done chan struct{}
}

// StreamingToolExecuteFunc is the callback signature for executing a single tool call.
// It encapsulates the full tool lifecycle: input validation, pre/post hooks, permission
// checks, progress reporting, and result rendering. The runtime provides this callback
// so the StreamingToolExecutor stays decoupled from hook and approval internals.
type StreamingToolExecuteFunc func(ctx context.Context, call coretool.Call, out chan<- event.Event) (coretool.Result, error)

// StreamingToolExecutor manages concurrent tool execution while the model stream is
// still being consumed. When a complete tool_use content block arrives, AddTool queues
// the tool and immediately tries to start execution (subject to concurrency constraints).
// The caller periodically calls CollectResults to retrieve completed tool result events
// in order, and calls AwaitAll after the stream ends to wait for any remaining tools.
type StreamingToolExecutor struct {
	// execute is the callback that runs one tool invocation.
	execute StreamingToolExecuteFunc
	// isConcurrencySafe reports whether the named tool may run in parallel.
	isConcurrencySafe func(toolName string) bool
	// out is the shared event output channel. Tool results are forwarded here
	// both during streaming (via CollectResults) and after streaming ends (via AwaitAll).
	out chan<- event.Event
	// tools tracks all tools added during the stream, preserving provider ordering.
	tools []streamingTrackedTool
	// mu protects concurrent access to tools, discarded, and result fields.
	mu sync.Mutex
	// discarded indicates the executor has been discarded due to stream fallback.
	// When true, AddTool and CollectResults become no-ops and AwaitAll returns immediately.
	discarded bool
	// maxConcurrent caps the number of parallel tool executions.
	maxConcurrent int
	// cascade manages the sibling error cascade: when a Bash tool errors during
	// parallel execution, all sibling tools are cancelled via the cascade context.
	cascade *SiblingCascade
}

// NewStreamingToolExecutor creates a StreamingToolExecutor ready to accept tools.
// The ctx parameter is used as the parent for the sibling cascade context, enabling
// Bash tool errors to cancel sibling tools while preserving the parent context.
func NewStreamingToolExecutor(
	ctx context.Context,
	execute StreamingToolExecuteFunc,
	isConcurrencySafe func(toolName string) bool,
	out chan<- event.Event,
	maxConcurrent int,
) *StreamingToolExecutor {
	return &StreamingToolExecutor{
		execute:           execute,
		isConcurrencySafe: isConcurrencySafe,
		out:               out,
		maxConcurrent:     maxConcurrent,
		cascade:           NewSiblingCascade(ctx),
	}
}

// AddTool registers one tool_use block detected during streaming and attempts to start
// its execution immediately if concurrency constraints allow.
func (e *StreamingToolExecutor) AddTool(ctx context.Context, toolUse model.ToolUse) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.discarded {
		return
	}

	tracked := streamingTrackedTool{
		id:                toolUse.ID,
		toolUse:           toolUse,
		status:            streamingToolQueued,
		isConcurrencySafe: e.isConcurrencySafe(toolUse.Name),
		done:              make(chan struct{}),
	}
	e.tools = append(e.tools, tracked)
	e.processQueue(ctx)
}

// Discard marks the executor as discarded. Subsequent AddTool and CollectResults calls
// become no-ops. Tools that are already executing will still finish but their results
// are silently dropped.
func (e *StreamingToolExecutor) Discard() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.discarded = true
}

// IsDiscarded reports whether the executor has been discarded.
func (e *StreamingToolExecutor) IsDiscarded() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.discarded
}

// CollectResults returns events for all tools that have completed since the last call.
// Results are returned in provider order (preserving the original tool_use sequence).
// Non-concurrency-safe tools that are still executing will block subsequent results
// from being yielded, ensuring ordering guarantees.
func (e *StreamingToolExecutor) CollectResults() []event.Event {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.discarded {
		return nil
	}

	var collected []event.Event
	for i := range e.tools {
		tool := &e.tools[i]
		if tool.status == streamingToolCompleted {
			collected = append(collected, tool.events...)
			tool.events = nil
			tool.status = streamingToolYielded
		}
		// A non-concurrency-safe tool that is still executing blocks further yielding.
		if tool.status == streamingToolExecuting && !tool.isConcurrencySafe {
			break
		}
	}
	return collected
}

// AwaitAll waits for all queued and executing tools to finish, then returns any
// remaining uncollected result events. If the executor is discarded, it returns
// immediately with an empty slice.
func (e *StreamingToolExecutor) AwaitAll(ctx context.Context) []event.Event {
	if e.IsDiscarded() {
		return nil
	}

	// Wait for all tools to complete.
	for i := range e.tools {
		select {
		case <-e.tools[i].done:
		case <-ctx.Done():
			return nil
		}
	}

	return e.CollectResults()
}

// ToolUseCount returns the number of tools added so far during streaming.
func (e *StreamingToolExecutor) ToolUseCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.tools)
}

// processQueue attempts to start execution of queued tools, respecting concurrency
// constraints and the sibling cascade state. The caller must hold e.mu.
// When the cascade has been triggered (a Bash tool errored), remaining queued tools
// are immediately marked as completed with a synthetic cascade error instead of being
// started, matching the TS getAbortReason → createSyntheticErrorMessage flow.
func (e *StreamingToolExecutor) processQueue(ctx context.Context) {
	for i := range e.tools {
		if e.tools[i].status != streamingToolQueued {
			continue
		}
		// If sibling cascade has been triggered, cancel remaining queued tools
		// with synthetic error messages instead of starting them.
		if e.cascade.IsErrored() {
			desc := e.cascade.ErroredToolDesc()
			syntheticMsg := FormatCascadeErrorMessage(desc)
			e.tools[i].result = coretool.Result{Error: syntheticMsg}
			e.tools[i].events = renderToolResultEvents(e.tools[i].toolUse, coretool.Result{Error: syntheticMsg}, nil)
			e.tools[i].status = streamingToolCompleted
			close(e.tools[i].done)
			continue
		}
		if e.canExecuteTool(e.tools[i].isConcurrencySafe) {
			e.tools[i].status = streamingToolExecuting
			go e.runTool(ctx, i)
		} else {
			// A non-concurrency-safe queued tool blocks all subsequent tools.
			if !e.tools[i].isConcurrencySafe {
				break
			}
		}
	}
}

// canExecuteTool reports whether a tool with the given concurrency safety can start
// given the current set of executing tools. The caller must hold e.mu.
func (e *StreamingToolExecutor) canExecuteTool(safe bool) bool {
	executing := 0
	allSafe := true
	for i := range e.tools {
		if e.tools[i].status == streamingToolExecuting {
			executing++
			if !e.tools[i].isConcurrencySafe {
				allSafe = false
			}
		}
	}
	if executing == 0 {
		return true
	}
	if e.maxConcurrent > 0 && executing >= e.maxConcurrent {
		return false
	}
	return safe && allSafe
}

// runTool executes one tracked tool and stores the result. Intended to run in a
// goroutine. It closes the tool's done channel when finished.
//
// The tool is executed using the cascade context so that when a sibling Bash tool
// errors, this tool receives a cancellation signal. After execution completes:
//   - If the tool is Bash and errored, the cascade is triggered to cancel siblings.
//   - If the tool was cancelled by the cascade (not its own error), a synthetic
//     cascade error message replaces the real result.
func (e *StreamingToolExecutor) runTool(ctx context.Context, index int) {
	e.mu.Lock()
	if index >= len(e.tools) {
		e.mu.Unlock()
		return
	}
	toolUse := e.tools[index].toolUse
	done := e.tools[index].done
	e.mu.Unlock()

	call := coretool.Call{
		ID:     toolUse.ID,
		Name:   toolUse.Name,
		Input:  toolUse.Input,
		Source: "model",
	}

	// Use cascade context for execution so sibling cancellation propagates.
	// When TriggerBashError is called, this context is cancelled, causing
	// the tool execution to return with context.Canceled.
	result, invokeErr := e.execute(e.cascade.Context(), call, e.out)

	e.mu.Lock()
	if e.discarded {
		e.mu.Unlock()
		close(done)
		return
	}

	// Check if this is a Bash tool error that should trigger the cascade.
	// This must happen before the synthetic error check so the triggering
	// Bash tool keeps its real error rather than getting a synthetic one.
	if triggered := e.cascade.TriggerOnBashError(toolUse.Name, toolUse.Input, result, invokeErr); triggered {
		logger.DebugCF("engine", "Bash tool error triggered sibling cascade", map[string]any{
			"tool": toolUse.Name,
			"desc": FormatToolDescription(toolUse.Name, toolUse.Input),
		})
	}

	if index < len(e.tools) {
		// If the cascade was triggered and this tool was cancelled by it (context.Canceled
		// from cascade context, not the tool's own error), replace with synthetic error.
		if e.cascade.IsErrored() && errors.Is(invokeErr, context.Canceled) {
			desc := e.cascade.ErroredToolDesc()
			syntheticMsg := FormatCascadeErrorMessage(desc)
			e.tools[index].result = coretool.Result{Error: syntheticMsg}
			e.tools[index].invokeErr = nil
			e.tools[index].events = renderToolResultEvents(toolUse, coretool.Result{Error: syntheticMsg}, nil)
		} else {
			e.tools[index].result = result
			e.tools[index].invokeErr = invokeErr
			e.tools[index].events = renderToolResultEvents(toolUse, result, invokeErr)
		}
		e.tools[index].status = streamingToolCompleted
	}
	e.mu.Unlock()

	close(done)

	// After one tool completes, try to start queued tools.
	e.mu.Lock()
	e.processQueue(ctx)
	e.mu.Unlock()
}

// renderToolResultEvents builds the tool result events for one completed tool call,
// mirroring the event emission logic in executeToolUses.
func renderToolResultEvents(toolUse model.ToolUse, result coretool.Result, invokeErr error) []event.Event {
	content, isError := renderToolResult(result, invokeErr)
	additionalContext := toolAdditionalContext(result)

	var events []event.Event
	events = append(events, event.Event{
		Type:      event.TypeToolCallFinished,
		Timestamp: time.Now(),
		Payload: event.ToolResultPayload{
			ID:                toolUse.ID,
			Name:              toolUse.Name,
			Output:            content,
			AdditionalContext: additionalContext,
			IsError:           isError,
		},
	})
	return events
}

// BuildToolResultMessage constructs a user message containing tool_result content
// parts for all completed tools. This mirrors executeToolUses but reads from the
// streaming executor's tracked tools instead of running tools from scratch.
func (e *StreamingToolExecutor) BuildToolResultMessage() message.Message {
	e.mu.Lock()
	defer e.mu.Unlock()

	resultMessage := message.Message{Role: message.RoleUser}
	for i := range e.tools {
		tool := &e.tools[i]
		if tool.status != streamingToolCompleted && tool.status != streamingToolYielded {
			continue
		}
		additionalContext := toolAdditionalContext(tool.result)
		if additionalContext != "" {
			resultMessage.Content = append(resultMessage.Content, message.MetaTextPart(additionalContext))
		}
		content, isError := renderToolResult(tool.result, tool.invokeErr)
		resultMessage.Content = append(resultMessage.Content, message.ToolResultPart(tool.toolUse.ID, content, isError))
		if !isError {
			if img := toolResultImage(tool.result); img != nil {
				resultMessage.Content = append(resultMessage.Content, message.ImagePart(img.MediaType, img.Base64))
			}
			for _, img := range toolResultImages(tool.result) {
				resultMessage.Content = append(resultMessage.Content, message.ImagePart(img.MediaType, img.Base64))
			}
			if doc := toolResultDocument(tool.result); doc != nil {
				resultMessage.Content = append(resultMessage.Content, message.DocumentPart(doc.MediaType, doc.Base64))
			}
		}
	}
	return resultMessage
}
