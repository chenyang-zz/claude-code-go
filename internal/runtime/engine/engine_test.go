package engine

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
)

type fakeModelClient struct {
	requests []model.Request
	streams  []model.Stream
	streamFn func(ctx context.Context, req model.Request) (model.Stream, error)
}

func (c *fakeModelClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	c.requests = append(c.requests, req)
	if c.streamFn != nil {
		return c.streamFn(ctx, req)
	}
	if len(c.streams) == 0 {
		return nil, errors.New("unexpected Stream call")
	}

	stream := c.streams[0]
	c.streams = c.streams[1:]
	return stream, nil
}

type fakeToolExecutor struct {
	results map[string]tool.Result
	errors  map[string]error
	calls   []tool.Call
	run     func(ctx context.Context, call tool.Call) (tool.Result, error)
	safe    map[string]bool
}

func (e *fakeToolExecutor) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	e.calls = append(e.calls, call)
	if e.run != nil {
		return e.run(ctx, call)
	}
	if err, ok := e.errors[call.Name]; ok {
		return tool.Result{}, err
	}
	if result, ok := e.results[call.Name]; ok {
		return result, nil
	}
	return tool.Result{}, nil
}

func (e *fakeToolExecutor) IsConcurrencySafe(toolName string) bool {
	return e.safe[toolName]
}

type fakeApprovalService struct {
	response approval.Response
	requests []approval.Request
}

func (s *fakeApprovalService) Decide(ctx context.Context, req approval.Request) (approval.Response, error) {
	_ = ctx
	s.requests = append(s.requests, req)
	return s.response, nil
}

type fakeStopHookRunner struct {
	results []hook.HookResult
	calls   []struct {
		event hook.HookEvent
		input any
		cwd   string
	}
}

func (r *fakeStopHookRunner) RunStopHooks(ctx context.Context, config hook.HooksConfig, event hook.HookEvent, input any, cwd string) []hook.HookResult {
	_ = ctx
	_ = config
	r.calls = append(r.calls, struct {
		event hook.HookEvent
		input any
		cwd   string
	}{
		event: event,
		input: input,
		cwd:   cwd,
	})
	return append([]hook.HookResult(nil), r.results...)
}

func newModelStream(events ...model.Event) model.Stream {
	stream := make(chan model.Event, len(events))
	for _, evt := range events {
		stream <- evt
	}
	close(stream)
	return stream
}

// TestRuntimeRunBuildsUserMessage verifies plain text input is converted into one user text message.
func TestRuntimeRunBuildsUserMessage(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(model.Event{
				Type: model.EventTypeTextDelta,
				Text: "hello",
			}),
		},
	}
	runtime := New(client, "claude-sonnet-4-5", nil)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "hello world",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	evt := <-out
	if evt.Type != event.TypeMessageDelta {
		t.Fatalf("Run() event type = %q, want message.delta", evt.Type)
	}
	for range out {
	}

	if len(client.requests) != 1 {
		t.Fatalf("Stream() call count = %d, want 1", len(client.requests))
	}

	msg := client.requests[0].Messages[0]
	if msg.Role != message.RoleUser || len(msg.Content) != 1 || msg.Content[0].Text != "hello world" {
		t.Fatalf("Stream() request message = %#v, want one user text message", msg)
	}
}

// TestRuntimeRunConvertsToolUse verifies provider tool-use events become runtime tool call events.
func TestRuntimeRunConvertsToolUse(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(model.Event{
				Type: model.EventTypeToolUse,
				ToolUse: &model.ToolUse{
					ID:   "toolu_1",
					Name: "Read",
					Input: map[string]any{
						"file_path": "main.go",
					},
				},
			}),
		},
	}
	runtime := New(client, "claude-sonnet-4-5", nil)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "hello world",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	evt := <-out
	if evt.Type != event.TypeToolCallStarted {
		t.Fatalf("Run() event type = %q, want tool.call.started", evt.Type)
	}

	payload, ok := evt.Payload.(event.ToolCallPayload)
	if !ok {
		t.Fatalf("Run() payload type = %T, want event.ToolCallPayload", evt.Payload)
	}
	if payload.ID != "toolu_1" || payload.Name != "Read" {
		t.Fatalf("Run() tool payload = %#v", payload)
	}
	if got := payload.Input["file_path"]; got != "main.go" {
		t.Fatalf("Run() tool payload input = %#v", payload.Input)
	}

	errorEvent := <-out
	if errorEvent.Type != event.TypeError {
		t.Fatalf("Run() second event type = %q, want error for missing executor", errorEvent.Type)
	}
}

func TestRuntimeRunStopHooksUseRequestCWD(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "done"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			),
		},
	}
	hookRunner := &fakeStopHookRunner{}
	runtime := New(client, "claude-sonnet-4-5", nil)
	runtime.TranscriptPath = "/tmp/transcript.jsonl"
	runtime.Hooks = hook.HooksConfig{
		hook.EventStop: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo ok"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "hello",
		CWD:       "/workspace/project",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	if len(hookRunner.calls) != 1 {
		t.Fatalf("RunStopHooks() call count = %d, want 1", len(hookRunner.calls))
	}
	if hookRunner.calls[0].cwd != "/workspace/project" {
		t.Fatalf("RunStopHooks() cwd = %q, want request cwd", hookRunner.calls[0].cwd)
	}
	input, ok := hookRunner.calls[0].input.(hook.StopHookInput)
	if !ok {
		t.Fatalf("RunStopHooks() input type = %T, want hook.StopHookInput", hookRunner.calls[0].input)
	}
	if input.CWD != "/workspace/project" {
		t.Fatalf("hook input cwd = %q, want request cwd", input.CWD)
	}
	if input.TranscriptPath != "/tmp/transcript.jsonl" {
		t.Fatalf("hook input transcript path = %q, want runtime transcript path", input.TranscriptPath)
	}
}

// TestRuntimeRunToolLoop verifies one tool_use can be executed and fed back into a second model request.
func TestRuntimeRunToolLoop(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(model.Event{
				Type: model.EventTypeToolUse,
				ToolUse: &model.ToolUse{
					ID:   "toolu_1",
					Name: "Read",
					Input: map[string]any{
						"file_path": "main.go",
					},
				},
			}),
			newModelStream(model.Event{
				Type: model.EventTypeTextDelta,
				Text: "done",
			}),
		},
	}
	executor := &fakeToolExecutor{
		results: map[string]tool.Result{
			"Read": {Output: "file contents"},
		},
	}
	runtime := New(client, "claude-sonnet-4-5", executor)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "read the file",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var events []event.Event
	for evt := range out {
		events = append(events, evt)
	}
	if len(events) != 4 {
		t.Fatalf("Run() event count = %d, want 4", len(events))
	}
	if events[0].Type != event.TypeToolCallStarted {
		t.Fatalf("Run() first event type = %q, want tool.call.started", events[0].Type)
	}
	if events[1].Type != event.TypeToolCallFinished {
		t.Fatalf("Run() second event type = %q, want tool.call.finished", events[1].Type)
	}
	if events[2].Type != event.TypeMessageDelta {
		t.Fatalf("Run() third event type = %q, want message.delta", events[2].Type)
	}
	if events[3].Type != event.TypeConversationDone {
		t.Fatalf("Run() fourth event type = %q, want conversation.done", events[3].Type)
	}

	if len(client.requests) != 2 {
		t.Fatalf("Stream() call count = %d, want 2", len(client.requests))
	}
	secondRequest := client.requests[1]
	if len(secondRequest.Messages) != 3 {
		t.Fatalf("second request message count = %d, want 3", len(secondRequest.Messages))
	}

	assistant := secondRequest.Messages[1]
	if assistant.Role != message.RoleAssistant || assistant.Content[0].Type != "tool_use" {
		t.Fatalf("second request assistant message = %#v, want tool_use assistant message", assistant)
	}
	toolResult := secondRequest.Messages[2]
	if toolResult.Role != message.RoleUser || toolResult.Content[0].Type != "tool_result" {
		t.Fatalf("second request tool result message = %#v, want tool_result user message", toolResult)
	}
	if toolResult.Content[0].ToolUseID != "toolu_1" || toolResult.Content[0].Text != "file contents" {
		t.Fatalf("second request tool result content = %#v", toolResult.Content[0])
	}
}

// TestRuntimeRunToolLoopExecutesConcurrencySafeBatchInParallel verifies consecutive concurrency-safe tools execute in parallel while preserving tool_result order.
func TestRuntimeRunToolLoopExecutesConcurrencySafeBatchInParallel(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{
					Type: model.EventTypeToolUse,
					ToolUse: &model.ToolUse{
						ID:   "toolu_read",
						Name: "Read",
					},
				},
				model.Event{
					Type: model.EventTypeToolUse,
					ToolUse: &model.ToolUse{
						ID:   "toolu_glob",
						Name: "Glob",
					},
				},
			),
			newModelStream(model.Event{
				Type: model.EventTypeTextDelta,
				Text: "done",
			}),
		},
	}

	var mu sync.Mutex
	current := 0
	maxConcurrent := 0
	started := make(chan struct{}, 2)
	barrier := make(chan struct{})
	go func() {
		<-started
		<-started
		close(barrier)
	}()

	executor := &fakeToolExecutor{
		safe: map[string]bool{
			"Read": true,
			"Glob": true,
		},
		run: func(ctx context.Context, call tool.Call) (tool.Result, error) {
			mu.Lock()
			current++
			if current > maxConcurrent {
				maxConcurrent = current
			}
			mu.Unlock()

			started <- struct{}{}
			select {
			case <-barrier:
			case <-time.After(150 * time.Millisecond):
			}

			if call.Name == "Read" {
				time.Sleep(40 * time.Millisecond)
			}
			if call.Name == "Glob" {
				time.Sleep(10 * time.Millisecond)
			}

			mu.Lock()
			current--
			mu.Unlock()

			return tool.Result{Output: call.Name + " ok"}, nil
		},
	}
	runtime := New(client, "claude-sonnet-4-5", executor)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "read and glob",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	if maxConcurrent != 2 {
		t.Fatalf("max concurrent tool executions = %d, want 2", maxConcurrent)
	}

	if len(client.requests) != 2 {
		t.Fatalf("Stream() call count = %d, want 2", len(client.requests))
	}
	secondRequest := client.requests[1]
	if len(secondRequest.Messages) != 3 {
		t.Fatalf("second request message count = %d, want 3", len(secondRequest.Messages))
	}

	toolResults := secondRequest.Messages[2].Content
	if len(toolResults) != 2 {
		t.Fatalf("tool result content count = %d, want 2", len(toolResults))
	}
	if toolResults[0].ToolUseID != "toolu_read" || toolResults[0].Text != "Read ok" {
		t.Fatalf("first tool result = %#v, want toolu_read / Read ok", toolResults[0])
	}
	if toolResults[1].ToolUseID != "toolu_glob" || toolResults[1].Text != "Glob ok" {
		t.Fatalf("second tool result = %#v, want toolu_glob / Glob ok", toolResults[1])
	}
}

// TestRuntimeRunToolLoopKeepsNonConcurrencySafeToolExclusive verifies non-safe tools do not overlap with a preceding parallel-safe batch.
func TestRuntimeRunToolLoopKeepsNonConcurrencySafeToolExclusive(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{
					Type: model.EventTypeToolUse,
					ToolUse: &model.ToolUse{
						ID:   "toolu_read",
						Name: "Read",
					},
				},
				model.Event{
					Type: model.EventTypeToolUse,
					ToolUse: &model.ToolUse{
						ID:   "toolu_glob",
						Name: "Glob",
					},
				},
				model.Event{
					Type: model.EventTypeToolUse,
					ToolUse: &model.ToolUse{
						ID:   "toolu_write",
						Name: "Write",
					},
				},
			),
			newModelStream(model.Event{
				Type: model.EventTypeTextDelta,
				Text: "done",
			}),
		},
	}

	var mu sync.Mutex
	current := 0
	writeOverlapped := false
	started := make(chan struct{}, 2)
	barrier := make(chan struct{})
	go func() {
		<-started
		<-started
		close(barrier)
	}()

	executor := &fakeToolExecutor{
		safe: map[string]bool{
			"Read":  true,
			"Glob":  true,
			"Write": false,
		},
		run: func(ctx context.Context, call tool.Call) (tool.Result, error) {
			mu.Lock()
			current++
			if call.Name == "Write" && current != 1 {
				writeOverlapped = true
			}
			mu.Unlock()

			if call.Name == "Read" || call.Name == "Glob" {
				started <- struct{}{}
				select {
				case <-barrier:
				case <-time.After(150 * time.Millisecond):
				}
			}

			time.Sleep(15 * time.Millisecond)

			mu.Lock()
			current--
			mu.Unlock()

			return tool.Result{Output: call.Name + " ok"}, nil
		},
	}
	runtime := New(client, "claude-sonnet-4-5", executor)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "read, glob, then write",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	if writeOverlapped {
		t.Fatal("Write overlapped with another running tool, want exclusive execution")
	}

	secondRequest := client.requests[1]
	toolResults := secondRequest.Messages[2].Content
	if len(toolResults) != 3 {
		t.Fatalf("tool result content count = %d, want 3", len(toolResults))
	}
	if toolResults[2].ToolUseID != "toolu_write" || toolResults[2].Text != "Write ok" {
		t.Fatalf("third tool result = %#v, want toolu_write / Write ok", toolResults[2])
	}
}

// TestRuntimeRunEmitsFinalHistory verifies successful runs emit the normalized conversation history for autosave consumers.
func TestRuntimeRunEmitsFinalHistory(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(model.Event{
				Type: model.EventTypeTextDelta,
				Text: "hello",
			}),
		},
	}
	runtime := New(client, "claude-sonnet-4-5", nil)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "session-1",
		Input:     "hi",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var donePayload event.ConversationDonePayload
	foundDone := false
	for evt := range out {
		if evt.Type != event.TypeConversationDone {
			continue
		}
		payload, ok := evt.Payload.(event.ConversationDonePayload)
		if !ok {
			t.Fatalf("Run() done payload type = %T, want event.ConversationDonePayload", evt.Payload)
		}
		donePayload = payload
		foundDone = true
	}

	if !foundDone {
		t.Fatal("Run() missing conversation.done event")
	}
	if len(donePayload.History.Messages) != 2 {
		t.Fatalf("done history message count = %d, want 2", len(donePayload.History.Messages))
	}
	if donePayload.History.Messages[0].Role != message.RoleUser {
		t.Fatalf("done history first role = %q, want user", donePayload.History.Messages[0].Role)
	}
	if donePayload.History.Messages[1].Role != message.RoleAssistant {
		t.Fatalf("done history second role = %q, want assistant", donePayload.History.Messages[1].Role)
	}
}

// TestRuntimeRunToolLoopConvertsExecutorError verifies tool execution failures become error tool_result messages instead of aborting the loop.
func TestRuntimeRunToolLoopConvertsExecutorError(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(model.Event{
				Type: model.EventTypeToolUse,
				ToolUse: &model.ToolUse{
					ID:   "toolu_1",
					Name: "Edit",
				},
			}),
			newModelStream(model.Event{
				Type: model.EventTypeTextDelta,
				Text: "handled",
			}),
		},
	}
	executor := &fakeToolExecutor{
		errors: map[string]error{
			"Edit": errors.New("tool executor: tool \"Edit\" not found"),
		},
	}
	runtime := New(client, "claude-sonnet-4-5", executor)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "edit the file",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	secondRequest := client.requests[1]
	toolResult := secondRequest.Messages[2].Content[0]
	if !toolResult.IsError {
		t.Fatalf("tool result is_error = false, want true")
	}
	if toolResult.Text != "tool executor: tool \"Edit\" not found" {
		t.Fatalf("tool result text = %q, want executor error", toolResult.Text)
	}
}

// TestRuntimeRunStopsAtLoopLimit verifies repeated tool_use responses are bounded by the configured loop cap.
func TestRuntimeRunStopsAtLoopLimit(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(model.Event{
				Type:    model.EventTypeToolUse,
				ToolUse: &model.ToolUse{ID: "toolu_1", Name: "Read"},
			}),
			newModelStream(model.Event{
				Type:    model.EventTypeToolUse,
				ToolUse: &model.ToolUse{ID: "toolu_2", Name: "Read"},
			}),
		},
	}
	executor := &fakeToolExecutor{
		results: map[string]tool.Result{
			"Read": {Output: "file contents"},
		},
	}
	runtime := New(client, "claude-sonnet-4-5", executor)
	runtime.MaxToolIterations = 1

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "loop",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var last event.Event
	for evt := range out {
		last = evt
	}
	if last.Type != event.TypeError {
		t.Fatalf("Run() last event type = %q, want error", last.Type)
	}
	payload, ok := last.Payload.(event.ErrorPayload)
	if !ok {
		t.Fatalf("Run() last payload type = %T, want event.ErrorPayload", last.Payload)
	}
	if payload.Message != "tool loop exceeded max iterations (1)" {
		t.Fatalf("Run() last error = %q, want loop limit", payload.Message)
	}
}

// TestRuntimeRunConvertsProviderErrors verifies provider error stream items become runtime error events.
func TestRuntimeRunConvertsProviderErrors(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(model.Event{
				Type:  model.EventTypeError,
				Error: "provider failed",
			}),
		},
	}
	runtime := New(client, "claude-sonnet-4-5", nil)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "hello world",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	evt := <-out
	payload, ok := evt.Payload.(event.ErrorPayload)
	if !ok {
		t.Fatalf("Run() payload type = %T, want event.ErrorPayload", evt.Payload)
	}
	if evt.Type != event.TypeError {
		t.Fatalf("Run() event type = %q, want error", evt.Type)
	}
	if payload.Message != "provider failed" {
		t.Fatalf("Run() error payload = %#v, want provider failed", payload)
	}
}

// TestRuntimeRunApprovalRetry verifies permission ask errors enter the approval flow and retry the tool after approval.
func TestRuntimeRunApprovalRetry(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(model.Event{
				Type: model.EventTypeToolUse,
				ToolUse: &model.ToolUse{
					ID:   "toolu_1",
					Name: "Read",
				},
			}),
			newModelStream(model.Event{
				Type: model.EventTypeTextDelta,
				Text: "approved",
			}),
		},
	}
	attempts := 0
	executor := &fakeToolExecutor{
		run: func(ctx context.Context, call tool.Call) (tool.Result, error) {
			attempts++
			if attempts == 1 {
				return tool.Result{}, (&corepermission.Evaluation{
					Decision: corepermission.DecisionAsk,
					Message:  "Claude requested permissions to read from /tmp/demo.txt, but you haven't granted it yet.",
				}).ToError(corepermission.FilesystemRequest{
					ToolName:   call.Name,
					Path:       "/tmp/demo.txt",
					WorkingDir: call.Context.WorkingDir,
					Access:     corepermission.AccessRead,
				})
			}
			return tool.Result{Output: "file contents"}, nil
		},
	}
	approvalService := &fakeApprovalService{
		response: approval.Response{Approved: true},
	}
	runtime := New(client, "claude-sonnet-4-5", executor)
	runtime.ApprovalService = approvalService

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "read the file",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var events []event.Event
	for evt := range out {
		events = append(events, evt)
	}
	if len(events) != 5 {
		t.Fatalf("Run() event count = %d, want 5", len(events))
	}
	if events[1].Type != event.TypeApprovalRequired {
		t.Fatalf("Run() second event type = %q, want approval.required", events[1].Type)
	}
	if events[4].Type != event.TypeConversationDone {
		t.Fatalf("Run() fifth event type = %q, want conversation.done", events[4].Type)
	}
	if attempts != 2 {
		t.Fatalf("Execute() attempts = %d, want 2", attempts)
	}
	if len(approvalService.requests) != 1 || approvalService.requests[0].Path != "/tmp/demo.txt" {
		t.Fatalf("approval requests = %#v, want one request for /tmp/demo.txt", approvalService.requests)
	}

	secondRequest := client.requests[1]
	toolResult := secondRequest.Messages[2].Content[0]
	if toolResult.Text != "file contents" || toolResult.IsError {
		t.Fatalf("tool result content = %#v, want approved tool output", toolResult)
	}
}

// TestRuntimeRunApprovalDenial verifies denied approval decisions become error tool_result blocks without retry success.
func TestRuntimeRunApprovalDenial(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(model.Event{
				Type: model.EventTypeToolUse,
				ToolUse: &model.ToolUse{
					ID:   "toolu_1",
					Name: "Write",
				},
			}),
			newModelStream(model.Event{
				Type: model.EventTypeTextDelta,
				Text: "denied",
			}),
		},
	}
	executor := &fakeToolExecutor{
		run: func(ctx context.Context, call tool.Call) (tool.Result, error) {
			return tool.Result{}, (&corepermission.Evaluation{
				Decision: corepermission.DecisionAsk,
				Message:  "Claude requested permissions to write to /tmp/demo.txt, but you haven't granted it yet.",
			}).ToError(corepermission.FilesystemRequest{
				ToolName:   call.Name,
				Path:       "/tmp/demo.txt",
				WorkingDir: call.Context.WorkingDir,
				Access:     corepermission.AccessWrite,
			})
		},
	}
	runtime := New(client, "claude-sonnet-4-5", executor)
	runtime.ApprovalService = &fakeApprovalService{
		response: approval.Response{
			Approved: false,
			Reason:   "Permission to write /tmp/demo.txt was not granted.",
		},
	}

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "write the file",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	secondRequest := client.requests[1]
	toolResult := secondRequest.Messages[2].Content[0]
	if !toolResult.IsError {
		t.Fatalf("tool result is_error = false, want true")
	}
	if toolResult.Text != "Permission to write /tmp/demo.txt was not granted." {
		t.Fatalf("tool result text = %q, want approval denial", toolResult.Text)
	}
}

// TestRuntimeRunBashApprovalRetry verifies Bash ask errors enter the runtime approval flow and retry after approval.
func TestRuntimeRunBashApprovalRetry(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(model.Event{
				Type: model.EventTypeToolUse,
				ToolUse: &model.ToolUse{
					ID:   "toolu_1",
					Name: "Bash",
				},
			}),
			newModelStream(model.Event{
				Type: model.EventTypeTextDelta,
				Text: "approved",
			}),
		},
	}
	attempts := 0
	executor := &fakeToolExecutor{
		run: func(ctx context.Context, call tool.Call) (tool.Result, error) {
			attempts++
			if attempts == 1 {
				return tool.Result{}, &corepermission.BashPermissionError{
					ToolName:   call.Name,
					Command:    "git status",
					WorkingDir: call.Context.WorkingDir,
					Decision:   corepermission.DecisionAsk,
					Message:    `Claude requested permissions to execute "git status", but you haven't granted it yet.`,
				}
			}
			if !corepermission.HasBashGrant(ctx, corepermission.BashRequest{
				ToolName:   call.Name,
				Command:    "git status",
				WorkingDir: call.Context.WorkingDir,
			}) {
				t.Fatal("retry context missing Bash grant")
			}
			return tool.Result{Output: "bash output"}, nil
		},
	}
	runtime := New(client, "claude-sonnet-4-5", executor)
	runtime.ApprovalService = &fakeApprovalService{
		response: approval.Response{Approved: true},
	}

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "run bash",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var events []event.Event
	for evt := range out {
		events = append(events, evt)
	}
	if len(events) != 5 {
		t.Fatalf("Run() event count = %d, want 5", len(events))
	}
	if events[1].Type != event.TypeApprovalRequired {
		t.Fatalf("Run() second event type = %q, want approval.required", events[1].Type)
	}
	payload, ok := events[1].Payload.(event.ApprovalPayload)
	if !ok {
		t.Fatalf("Run() approval payload type = %T, want event.ApprovalPayload", events[1].Payload)
	}
	if payload.Action != "execute" || payload.Path != "git status" {
		t.Fatalf("Run() approval payload = %#v, want execute git status", payload)
	}
	if attempts != 2 {
		t.Fatalf("Execute() attempts = %d, want 2", attempts)
	}

	secondRequest := client.requests[1]
	toolResult := secondRequest.Messages[2].Content[0]
	if toolResult.Text != "bash output" || toolResult.IsError {
		t.Fatalf("tool result content = %#v, want approved Bash output", toolResult)
	}
}

// TestRuntimeRunBashApprovalDenial verifies denied Bash approval decisions become error tool_result blocks without retry success.
func TestRuntimeRunBashApprovalDenial(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(model.Event{
				Type: model.EventTypeToolUse,
				ToolUse: &model.ToolUse{
					ID:   "toolu_1",
					Name: "Bash",
				},
			}),
			newModelStream(model.Event{
				Type: model.EventTypeTextDelta,
				Text: "denied",
			}),
		},
	}
	executor := &fakeToolExecutor{
		run: func(ctx context.Context, call tool.Call) (tool.Result, error) {
			return tool.Result{}, &corepermission.BashPermissionError{
				ToolName:   call.Name,
				Command:    "git status",
				WorkingDir: call.Context.WorkingDir,
				Decision:   corepermission.DecisionAsk,
				Message:    `Claude requested permissions to execute "git status", but you haven't granted it yet.`,
			}
		},
	}
	runtime := New(client, "claude-sonnet-4-5", executor)
	runtime.ApprovalService = &fakeApprovalService{
		response: approval.Response{
			Approved: false,
			Reason:   `Permission to execute "git status" was not granted.`,
		},
	}

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "run bash",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	secondRequest := client.requests[1]
	toolResult := secondRequest.Messages[2].Content[0]
	if !toolResult.IsError {
		t.Fatalf("tool result is_error = false, want true")
	}
	if toolResult.Text != `Permission to execute "git status" was not granted.` {
		t.Fatalf("tool result text = %q, want Bash approval denial", toolResult.Text)
	}
}

// TestRuntimeRunConcurrentBatchPreservesEventOrder verifies that started/finished events
// preserve original tool_use ordering even when tools complete out of order in a concurrent batch.
func TestRuntimeRunConcurrentBatchPreservesEventOrder(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{
					Type: model.EventTypeToolUse,
					ToolUse: &model.ToolUse{
						ID:   "toolu_read",
						Name: "Read",
					},
				},
				model.Event{
					Type: model.EventTypeToolUse,
					ToolUse: &model.ToolUse{
						ID:   "toolu_glob",
						Name: "Glob",
					},
				},
			),
			newModelStream(model.Event{
				Type: model.EventTypeTextDelta,
				Text: "done",
			}),
		},
	}

	readDone := make(chan struct{})
	executor := &fakeToolExecutor{
		safe: map[string]bool{
			"Read": true,
			"Glob": true,
		},
		run: func(ctx context.Context, call tool.Call) (tool.Result, error) {
			if call.Name == "Glob" {
				// Glob finishes fast
				time.Sleep(5 * time.Millisecond)
				return tool.Result{Output: "Glob ok"}, nil
			}
			// Read finishes slower than Glob
			time.Sleep(50 * time.Millisecond)
			close(readDone)
			return tool.Result{Output: "Read ok"}, nil
		},
	}
	runtime := New(client, "claude-sonnet-4-5", executor)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "read and glob",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var events []event.Event
	for evt := range out {
		events = append(events, evt)
	}

	// tool.call.started events should appear in original order (Read, Glob)
	var startedEvents []event.Event
	for _, evt := range events {
		if evt.Type == event.TypeToolCallStarted {
			startedEvents = append(startedEvents, evt)
		}
	}
	if len(startedEvents) != 2 {
		t.Fatalf("started event count = %d, want 2", len(startedEvents))
	}
	readStarted, _ := startedEvents[0].Payload.(event.ToolCallPayload)
	globStarted, _ := startedEvents[1].Payload.(event.ToolCallPayload)
	if readStarted.ID != "toolu_read" {
		t.Fatalf("first started = %s, want toolu_read", readStarted.ID)
	}
	if globStarted.ID != "toolu_glob" {
		t.Fatalf("second started = %s, want toolu_glob", globStarted.ID)
	}

	// tool.call.finished events should also preserve original order (Read, Glob)
	var finishedEvents []event.Event
	for _, evt := range events {
		if evt.Type == event.TypeToolCallFinished {
			finishedEvents = append(finishedEvents, evt)
		}
	}
	if len(finishedEvents) != 2 {
		t.Fatalf("finished event count = %d, want 2", len(finishedEvents))
	}
	readFinished, _ := finishedEvents[0].Payload.(event.ToolResultPayload)
	globFinished, _ := finishedEvents[1].Payload.(event.ToolResultPayload)
	if readFinished.ID != "toolu_read" {
		t.Fatalf("first finished = %s, want toolu_read", readFinished.ID)
	}
	if globFinished.ID != "toolu_glob" {
		t.Fatalf("second finished = %s, want toolu_glob", globFinished.ID)
	}
}

// TestRuntimeRun_AutoCompactDisabled verifies that auto-compact is not triggered
// when AutoCompact is false (default).
func TestRuntimeRun_AutoCompactDisabled(t *testing.T) {
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			return newModelStream(model.Event{
				Type: model.EventTypeTextDelta,
				Text: "response",
			}), nil
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)
	// AutoCompact defaults to false.

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var hasCompactDone bool
	for evt := range out {
		if evt.Type == event.TypeCompactDone {
			hasCompactDone = true
		}
	}
	if hasCompactDone {
		t.Error("expected no compact.done event when AutoCompact is disabled")
	}
}

// TestRuntimeRun_AutoCompactSmallMessages verifies that auto-compact does not
// trigger when messages are small.
func TestRuntimeRun_AutoCompactSmallMessages(t *testing.T) {
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			return newModelStream(model.Event{
				Type: model.EventTypeTextDelta,
				Text: "response",
			}), nil
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)
	runtime.AutoCompact = true

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "short message",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var hasCompactDone bool
	for evt := range out {
		if evt.Type == event.TypeCompactDone {
			hasCompactDone = true
		}
	}
	if hasCompactDone {
		t.Error("expected no compact.done for small messages")
	}
}

// TestRuntimeRun_MaxTokensContinuation verifies that when the model stops with
// max_tokens and no tool_use, the engine injects a continuation user message
// and sends another request so the model can finish generating.
func TestRuntimeRun_MaxTokensContinuation(t *testing.T) {
	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			if callCount == 1 {
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "partial"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonMaxTokens},
				), nil
			}
			return newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: " rest"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			), nil
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var textDeltas []string
	var doneCount int
	for evt := range out {
		if evt.Type == event.TypeMessageDelta {
			textDeltas = append(textDeltas, evt.Payload.(event.MessageDeltaPayload).Text)
		}
		if evt.Type == event.TypeConversationDone {
			doneCount++
		}
	}
	if doneCount != 1 {
		t.Fatalf("conversation.done count = %d, want 1", doneCount)
	}
	if callCount != 2 {
		t.Fatalf("Stream() call count = %d, want 2 (initial + continuation)", callCount)
	}
	got := ""
	for _, d := range textDeltas {
		got += d
	}
	if got != "partial rest" {
		t.Fatalf("text deltas = %q, want 'partial rest'", got)
	}
}

// TestRuntimeRun_MaxTokensContinuationDoesNotPersistSyntheticUserMessage verifies
// the continuation instruction is sent to the model but not stored in final history.
func TestRuntimeRun_MaxTokensContinuationDoesNotPersistSyntheticUserMessage(t *testing.T) {
	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			if callCount == 1 {
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "partial"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonMaxTokens},
				), nil
			}
			return newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: " rest"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			), nil
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var donePayload event.ConversationDonePayload
	for evt := range out {
		if evt.Type != event.TypeConversationDone {
			continue
		}
		payload, ok := evt.Payload.(event.ConversationDonePayload)
		if !ok {
			t.Fatalf("done payload type = %T", evt.Payload)
		}
		donePayload = payload
	}

	if len(client.requests) != 2 {
		t.Fatalf("Stream() call count = %d, want 2", len(client.requests))
	}
	secondRequest := client.requests[1]
	if len(secondRequest.Messages) != 3 {
		t.Fatalf("second request message count = %d, want 3", len(secondRequest.Messages))
	}
	if got := secondRequest.Messages[2].Content[0].Text; got != continuationUserMessage {
		t.Fatalf("second request continuation message = %q, want %q", got, continuationUserMessage)
	}

	if len(donePayload.History.Messages) != 3 {
		t.Fatalf("done history message count = %d, want 3", len(donePayload.History.Messages))
	}
	for _, msg := range donePayload.History.Messages {
		for _, part := range msg.Content {
			if part.Text == continuationUserMessage {
				t.Fatal("final history should not persist synthetic continuation user message")
			}
		}
	}
}

// TestRuntimeRun_MaxTokensContinuationLimit verifies that continuation stops
// after maxContinuationAttempts even if the model keeps returning max_tokens.
func TestRuntimeRun_MaxTokensContinuationLimit(t *testing.T) {
	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			return newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "chunk"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonMaxTokens},
			), nil
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var doneCount int
	for range out {
		doneCount++
	}
	// 1 initial + 3 continuations = 4 total calls, then stops.
	if callCount != maxContinuationAttempts+1 {
		t.Fatalf("Stream() call count = %d, want %d", callCount, maxContinuationAttempts+1)
	}
	// doneCount includes all events, not just conversation.done.
	// Just verify the run completed without error.
}

// TestRuntimeRun_MaxTokensWithToolUseNoContinuation verifies that max_tokens
// with tool_use blocks does NOT trigger continuation.
func TestRuntimeRun_MaxTokensWithToolUseNoContinuation(t *testing.T) {
	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			return newModelStream(
				model.Event{
					Type: model.EventTypeToolUse,
					ToolUse: &model.ToolUse{
						ID:    "toolu_1",
						Name:  "Read",
						Input: map[string]any{"file_path": "test.go"},
					},
				},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonMaxTokens},
			), nil
		},
	}
	executor := &fakeToolExecutor{
		results: map[string]tool.Result{"Read": {Output: "ok"}},
	}
	runtime := New(client, "claude-sonnet-4-20250514", executor)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "read file",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for range out {
	}
	if callCount < 2 {
		t.Fatalf("Stream() call count = %d, expected at least 2 (tool loop)", callCount)
	}
}

// TestRuntimeRun_PromptTooLongRecovery verifies that a prompt-too-long API error
// triggers emergency compaction and retries with the compressed history.
func TestRuntimeRun_PromptTooLongRecovery(t *testing.T) {
	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			if callCount == 1 {
				// Note: token counts chosen to avoid accidental substring matches
				// with isRetriableError patterns (e.g. "500" in "250000").
				return nil, errors.New("prompt is too long: 250000 tokens > 200000")
			}
			return newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "recovered"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			), nil
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)
	runtime.AutoCompact = true

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var hasCompactDone bool
	var hasRecoveredText bool
	for evt := range out {
		if evt.Type == event.TypeCompactDone {
			hasCompactDone = true
		}
		if evt.Type == event.TypeMessageDelta {
			if evt.Payload.(event.MessageDeltaPayload).Text == "recovered" {
				hasRecoveredText = true
			}
		}
	}
	if !hasCompactDone {
		t.Error("expected compact.done event from emergency compaction")
	}
	if !hasRecoveredText {
		t.Error("expected 'recovered' text from post-compact retry")
	}
}

// TestRuntimeRun_PromptTooLongRecoveryOnlyOnce verifies that recovery is attempted
// at most once — if the retry still fails, the error is surfaced.
func TestRuntimeRun_PromptTooLongRecoveryOnlyOnce(t *testing.T) {
	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			return nil, errors.New("prompt is too long: 250000 tokens > 200000")
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)
	runtime.AutoCompact = true

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("unexpected Run() error: %v", err)
	}

	var hasError bool
	for evt := range out {
		if evt.Type == event.TypeError {
			hasError = true
			payload, ok := evt.Payload.(event.ErrorPayload)
			if !ok {
				t.Fatalf("error payload type = %T", evt.Payload)
			}
			if !strings.Contains(payload.Message, "prompt is too long") {
				t.Errorf("error message = %q, want prompt too long", payload.Message)
			}
		}
	}
	if !hasError {
		t.Error("expected error event when recovery fails")
	}
}

// TestRuntimeRun_PromptTooLongNoAutoCompact verifies prompt-too-long errors
// are not recovered when AutoCompact is disabled.
func TestRuntimeRun_PromptTooLongNoAutoCompact(t *testing.T) {
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			return nil, errors.New("prompt is too long: 250000 tokens > 200000")
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)
	runtime.AutoCompact = false

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("unexpected Run() error: %v", err)
	}

	var hasError bool
	for evt := range out {
		if evt.Type == event.TypeError {
			hasError = true
		}
	}
	if !hasError {
		t.Error("expected error event when AutoCompact is disabled and prompt is too long")
	}
}

// TestRuntimeRun_MaxTokensContinuationSkipsAutoCompact verifies that auto-compact
// is skipped when transient continuation messages are pending, preventing the
// compactor from summarizing away the truncated assistant turn.
func TestRuntimeRun_MaxTokensContinuationSkipsAutoCompact(t *testing.T) {
	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			if callCount == 1 {
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "partial"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonMaxTokens},
				), nil
			}
			return newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: " rest"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			), nil
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)
	runtime.AutoCompact = true
	// Without the fix, AutoCompact would trigger on the continuation iteration,
	// potentially replacing history with a compacted version (compact event emitted).
	// With the fix, no compact event should appear during the continuation flow.

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var compactCount int
	for evt := range out {
		if evt.Type == event.TypeCompactDone {
			compactCount++
		}
	}

	if callCount != 2 {
		t.Fatalf("Stream() call count = %d, want 2", callCount)
	}
	if compactCount != 0 {
		t.Errorf("compact events = %d, want 0 (auto-compact should be skipped during continuation)", compactCount)
	}
	// Verify the second request still has the truncated assistant content
	// (i.e. history was not compacted away).
	secondReq := client.requests[1]
	if len(secondReq.Messages) != 3 {
		t.Fatalf("second request message count = %d, want 3 (user + assistant + continuation)", len(secondReq.Messages))
	}
}

// TestRuntimeRun_MaxTokensContinuationWithPromptTooLong verifies that when a
// continuation request triggers prompt-too-long, the emergency compact preserves
// the continuation instruction in the retry request.
func TestRuntimeRun_PromptTooLongRecoveryResetsOnSuccess(t *testing.T) {
	// Verify that hasAttemptedRecovery resets after a successful model call,
	// so a second prompt-too-long error in the same turn can also be recovered.
	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			if callCount == 1 {
				// First call: prompt too long → recovery compact
				return nil, errors.New("prompt is too long: 300000 tokens > 200000")
			}
			if callCount == 2 {
				// Compact's internal summary call
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "summary"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
				), nil
			}
			if callCount == 3 {
				// Post-compact retry succeeds
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "answer"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
				), nil
			}
			t.Fatalf("unexpected call %d", callCount)
			return nil, nil
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)
	runtime.AutoCompact = true

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range out {
	}
	if callCount != 3 {
		t.Fatalf("Stream() call count = %d, want 3", callCount)
	}
}

func TestRuntimeRun_PromptTooLongRecoveryNoCorruptOrder(t *testing.T) {
	// Verify that a plain prompt-too-long error (no continuation) does not
	// re-append an older assistant message after compact, which would
	// corrupt message ordering.
	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			if callCount == 1 {
				return nil, errors.New("prompt is too long: 300000 tokens > 200000")
			}
			if callCount == 2 {
				// compact summary call
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "summary"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
				), nil
			}
			// post-compact retry
			return newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "answer"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			), nil
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)
	runtime.AutoCompact = true

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range out {
	}

	// The post-compact retry (request 3) should NOT have an extra assistant
	// message appended after the user input.
	retryReq := client.requests[2]
	for i := 1; i < len(retryReq.Messages); i++ {
		if retryReq.Messages[i].Role == message.RoleAssistant && retryReq.Messages[i-1].Role == message.RoleUser {
			t.Errorf("post-compact retry has assistant after user at position %d, message order is corrupted", i)
		}
	}
}

func TestRuntimeRun_PromptTooLongRecoveryPreservesOriginalLastUserTurn(t *testing.T) {
	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			if callCount == 1 {
				return nil, errors.New("prompt is too long: 300000 tokens > 200000")
			}
			if callCount == 2 {
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "summary"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
				), nil
			}
			return newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "answer"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			), nil
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)
	runtime.AutoCompact = true

	originalLastUser := message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart("Inspect this screenshot and attached spec."),
			{Type: "image", Text: "image-bytes"},
			{Type: "document", Text: "document-bytes"},
		},
	}

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Messages: []message.Message{
			{
				Role:    message.RoleUser,
				Content: []message.ContentPart{message.TextPart("Earlier request")},
			},
			originalLastUser,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range out {
	}

	if callCount != 3 {
		t.Fatalf("Stream() call count = %d, want 3", callCount)
	}

	retryReq := client.requests[2]
	var foundOriginalUser bool
	var foundPlaceholderUser bool
	for _, msg := range retryReq.Messages {
		if msg.Role != message.RoleUser {
			continue
		}
		if reflect.DeepEqual(msg.Content, originalLastUser.Content) {
			foundOriginalUser = true
		}
		for _, part := range msg.Content {
			if part.Text == "[image]" || part.Text == "[document]" {
				foundPlaceholderUser = true
			}
		}
	}
	if !foundOriginalUser {
		t.Fatal("post-compact retry is missing the original last user turn with attachments")
	}
	if foundPlaceholderUser {
		t.Fatal("post-compact retry should not preserve stripped attachment placeholders")
	}
}

func TestRuntimeRun_ContinuationBudgetResetsOnToolLoop(t *testing.T) {
	// Verify that continuationAttempts resets when entering the tool loop,
	// so a truncated response in the next phase gets a fresh budget.
	callCount := 0
	toolExecuted := false
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			if callCount == 1 {
				// Phase 1: text + tool_use (enters tool loop)
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "let me check"},
					model.Event{Type: model.EventTypeToolUse, ToolUse: &model.ToolUse{ID: "t1", Name: "Read", Input: map[string]any{"path": "/tmp/x"}}},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonToolUse},
				), nil
			}
			if callCount == 2 {
				// Phase 2: max_tokens (should start fresh continuation chain)
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "long answer"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonMaxTokens},
				), nil
			}
			// Phase 2 continuation: succeeds
			return newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: " rest"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			), nil
		},
	}
	executor := &fakeToolExecutor{
		run: func(ctx context.Context, call tool.Call) (tool.Result, error) {
			toolExecuted = true
			return tool.Result{Output: "tool result"}, nil
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", executor)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range out {
	}
	if !toolExecuted {
		t.Fatal("tool was not executed")
	}
	// Expected: call 1 = text+tool, call 2 = max_tokens, call 3 = continuation
	if callCount != 3 {
		t.Fatalf("Stream() call count = %d, want 3", callCount)
	}
	// Verify continuation message was injected in request 3
	thirdReq := client.requests[2]
	lastMsg := thirdReq.Messages[len(thirdReq.Messages)-1]
	if lastMsg.Role != message.RoleUser {
		t.Fatalf("continuation request last message role = %s, want %s", lastMsg.Role, message.RoleUser)
	}
	if !strings.Contains(lastMsg.Content[0].Text, "Resume directly") {
		t.Errorf("continuation request last message = %q, want continuation instruction", lastMsg.Content[0].Text)
	}
}

func TestRuntimeRun_MaxTokensContinuationWithPromptTooLong(t *testing.T) {
	callCount := 0
	latestPrompt := "hello"
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			if callCount == 1 {
				// First call: max_tokens truncation
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "partial"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonMaxTokens},
				), nil
			}
			if callCount == 2 {
				// Second call (continuation): prompt too long
				return nil, errors.New("prompt is too long: 250000 tokens > 200000")
			}
			// callCount 3: compact's internal summary request
			// callCount 4: post-compact retry → success
			return newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: " rest"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			), nil
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)
	runtime.AutoCompact = true

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for range out {
	}

	// Expected calls: 1=initial, 2=continuation(prompt-too-long),
	// 3=compact summary, 4=post-compact retry with continuation.
	if callCount != 4 {
		t.Fatalf("Stream() call count = %d, want 4", callCount)
	}

	// The fourth request (post-compact retry) should still include the
	// latest user prompt, the truncated assistant text, and the continuation
	// user message so the model resumes mid-thought against the exact request.
	fourthReq := client.requests[3]
	lastMsg := fourthReq.Messages[len(fourthReq.Messages)-1]
	if lastMsg.Role != message.RoleUser {
		t.Fatalf("post-compact retry last message role = %s, want %s", lastMsg.Role, message.RoleUser)
	}
	text := lastMsg.Content[0].Text
	if !strings.Contains(text, "Resume directly") {
		t.Errorf("post-compact retry last message = %q, want continuation instruction", text)
	}

	var foundLatestPrompt bool
	var foundTruncatedAssistant bool
	for _, msg := range fourthReq.Messages {
		for _, part := range msg.Content {
			if msg.Role == message.RoleUser && part.Text == latestPrompt {
				foundLatestPrompt = true
			}
			if msg.Role == message.RoleAssistant && part.Text == "partial" {
				foundTruncatedAssistant = true
			}
		}
	}
	if !foundLatestPrompt {
		t.Error("post-compact retry is missing the latest user prompt text")
	}
	if !foundTruncatedAssistant {
		t.Error("post-compact retry is missing the truncated assistant message with original text")
	}
}

func TestRuntimeRun_MaxTokensContinuationWithPromptTooLongPreservesToolLoopContext(t *testing.T) {
	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			switch callCount {
			case 1:
				return newModelStream(
					model.Event{Type: model.EventTypeToolUse, ToolUse: &model.ToolUse{ID: "tool-1", Name: "Read", Input: map[string]any{"path": "/tmp/spec.md"}}},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonToolUse},
				), nil
			case 2:
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "partial answer from tool output"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonMaxTokens},
				), nil
			case 3:
				return nil, errors.New("prompt is too long: 250000 tokens > 200000")
			default:
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: " final"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
				), nil
			}
		},
	}
	executor := &fakeToolExecutor{
		run: func(ctx context.Context, call tool.Call) (tool.Result, error) {
			return tool.Result{Output: "tool output"}, nil
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", executor)
	runtime.AutoCompact = true

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Input:     "use the tool result",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range out {
	}

	if callCount != 5 {
		t.Fatalf("Stream() call count = %d, want 5", callCount)
	}

	retryReq := client.requests[4]
	var foundOriginalUser bool
	var foundToolUse bool
	var foundToolResult bool
	var foundTruncatedAssistant bool
	for _, msg := range retryReq.Messages {
		for _, part := range msg.Content {
			switch {
			case msg.Role == message.RoleUser && part.Type == "text" && part.Text == "use the tool result":
				foundOriginalUser = true
			case msg.Role == message.RoleAssistant && part.Type == "tool_use" && part.ToolUseID == "tool-1":
				foundToolUse = true
			case msg.Role == message.RoleUser && part.Type == "tool_result" && part.ToolUseID == "tool-1" && part.Text == "tool output":
				foundToolResult = true
			case msg.Role == message.RoleAssistant && part.Type == "text" && part.Text == "partial answer from tool output":
				foundTruncatedAssistant = true
			}
		}
	}
	if !foundOriginalUser {
		t.Fatal("post-compact retry is missing the original user prompt")
	}
	if !foundToolUse {
		t.Fatal("post-compact retry is missing the assistant tool_use message")
	}
	if !foundToolResult {
		t.Fatal("post-compact retry is missing the user tool_result message")
	}
	if !foundTruncatedAssistant {
		t.Fatal("post-compact retry is missing the truncated assistant message")
	}
}

// TestRuntimeRun_TokenBudgetContinuation verifies that the engine auto-continues
// the model when the turn token budget has not been reached.
func TestRuntimeRun_TokenBudgetContinuation(t *testing.T) {
	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			if callCount == 1 {
				// First call: model produces 500 output tokens, well under budget.
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "partial work"},
					model.Event{Type: model.EventTypeDone, Usage: &model.Usage{OutputTokens: 500}},
				), nil
			}
			// Second call: model produces enough to exceed 90% threshold.
			return newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: " more work"},
				model.Event{Type: model.EventTypeDone, Usage: &model.Usage{OutputTokens: 9000}},
			), nil
		},
	}
	runtime := New(client, "claude-sonnet-4-5", nil)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID:       "cli",
		Input:           "do work",
		TurnTokenBudget: 10000,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	if len(client.requests) != 2 {
		t.Fatalf("expected 2 model calls, got %d", len(client.requests))
	}

	// The second request should contain a nudge user message.
	lastMsg := client.requests[1].Messages[len(client.requests[1].Messages)-1]
	if lastMsg.Role != message.RoleUser {
		t.Fatalf("second request last message role = %s, want user", lastMsg.Role)
	}
	if !strings.Contains(lastMsg.Content[0].Text, "token target") {
		t.Fatalf("nudge message = %q, want token budget nudge", lastMsg.Content[0].Text)
	}
}

// TestRuntimeRun_TokenBudgetNoBudgetNoContinuation verifies that without a
// budget set, the engine does not trigger budget continuation.
func TestRuntimeRun_TokenBudgetNoBudgetNoContinuation(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "done"},
				model.Event{Type: model.EventTypeDone, Usage: &model.Usage{OutputTokens: 500}},
			),
		},
	}
	runtime := New(client, "claude-sonnet-4-5", nil)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "hello",
		// TurnTokenBudget is zero (default) — no budget tracking.
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	if len(client.requests) != 1 {
		t.Fatalf("expected 1 model call with no budget, got %d", len(client.requests))
	}
}

// TestRuntimeRun_TokenBudgetStopAtThreshold verifies that the engine stops
// when the turn output tokens exceed 90% of the budget.
func TestRuntimeRun_TokenBudgetStopAtThreshold(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "done"},
				model.Event{Type: model.EventTypeDone, Usage: &model.Usage{OutputTokens: 9500}},
			),
		},
	}
	runtime := New(client, "claude-sonnet-4-5", nil)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID:       "cli",
		Input:           "big task",
		TurnTokenBudget: 10000,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var done event.Event
	for evt := range out {
		if evt.Type == event.TypeConversationDone {
			done = evt
		}
	}
	if done.Type != event.TypeConversationDone {
		t.Fatal("expected ConversationDone event")
	}
	if len(client.requests) != 1 {
		t.Fatalf("expected 1 model call (no continuation at threshold), got %d", len(client.requests))
	}
}

func TestRuntimeRun_TokenBudgetIgnoresAutoCompactSummaryUsage(t *testing.T) {
	t.Setenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW", "50000")

	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			switch callCount {
			case 1:
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "summary"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn, Usage: &model.Usage{OutputTokens: 8500}},
				), nil
			case 2:
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "partial work"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn, Usage: &model.Usage{OutputTokens: 500}},
				), nil
			default:
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: " more work"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn, Usage: &model.Usage{OutputTokens: 9000}},
				), nil
			}
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)
	runtime.AutoCompact = true

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "test",
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					message.TextPart(strings.Repeat("x", 400000)),
				},
			},
		},
		TurnTokenBudget: 10000,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	if len(client.requests) != 3 {
		t.Fatalf("expected 3 model calls (compact + response + continuation), got %d", len(client.requests))
	}

	lastMsg := client.requests[2].Messages[len(client.requests[2].Messages)-1]
	if lastMsg.Role != message.RoleUser {
		t.Fatalf("continuation request last message role = %s, want user", lastMsg.Role)
	}
	if !strings.Contains(lastMsg.Content[0].Text, "token target") {
		t.Fatalf("nudge message = %q, want token budget nudge", lastMsg.Content[0].Text)
	}
}

func TestRuntimeRun_TokenBudgetDoesNotBypassMaxTokensContinuationCap(t *testing.T) {
	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			return newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "chunk"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonMaxTokens, Usage: &model.Usage{OutputTokens: 100}},
			), nil
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID:       "test",
		Input:           "keep going",
		TurnTokenBudget: 10000,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	if got, want := len(client.requests), maxContinuationAttempts+1; got != want {
		t.Fatalf("model call count = %d, want %d", got, want)
	}
	lastReq := client.requests[len(client.requests)-1]
	lastMsg := lastReq.Messages[len(lastReq.Messages)-1]
	if lastMsg.Role != message.RoleUser {
		t.Fatalf("last request last message role = %s, want user continuation prompt", lastMsg.Role)
	}
	if strings.Contains(lastMsg.Content[0].Text, "token target") {
		t.Fatalf("last request last message = %q, want max_tokens continuation prompt without budget nudge", lastMsg.Content[0].Text)
	}
	if callCount != maxContinuationAttempts+1 {
		t.Fatalf("Stream() call count = %d, want %d", callCount, maxContinuationAttempts+1)
	}
}
