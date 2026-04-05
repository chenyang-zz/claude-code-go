package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
)

type fakeModelClient struct {
	requests []model.Request
	streams  []model.Stream
}

func (c *fakeModelClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	c.requests = append(c.requests, req)
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

type fakeApprovalService struct {
	response approval.Response
	requests []approval.Request
}

func (s *fakeApprovalService) Decide(ctx context.Context, req approval.Request) (approval.Response, error) {
	_ = ctx
	s.requests = append(s.requests, req)
	return s.response, nil
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
