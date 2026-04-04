package engine

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

type fakeModelClient struct {
	lastRequest model.Request
	stream      model.Stream
}

func (c *fakeModelClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	c.lastRequest = req
	return c.stream, nil
}

// TestRuntimeRunBuildsUserMessage verifies plain text input is converted into one user text message.
func TestRuntimeRunBuildsUserMessage(t *testing.T) {
	stream := make(chan model.Event, 2)
	stream <- model.Event{
		Type: model.EventTypeTextDelta,
		Text: "hello",
	}
	close(stream)

	client := &fakeModelClient{stream: stream}
	runtime := New(client, "claude-sonnet-4-5")

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "hello world",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(client.lastRequest.Messages) != 1 {
		t.Fatalf("Stream() got %d messages, want 1", len(client.lastRequest.Messages))
	}

	msg := client.lastRequest.Messages[0]
	if msg.Role != message.RoleUser || len(msg.Content) != 1 || msg.Content[0].Text != "hello world" {
		t.Fatalf("Stream() request message = %#v, want one user text message", msg)
	}

	evt := <-out
	if evt.Type != event.TypeMessageDelta {
		t.Fatalf("Run() event type = %q, want message.delta", evt.Type)
	}
}

// TestRuntimeRunConvertsToolUse verifies provider tool-use events become runtime tool call events.
func TestRuntimeRunConvertsToolUse(t *testing.T) {
	stream := make(chan model.Event, 2)
	stream <- model.Event{
		Type: model.EventTypeToolUse,
		ToolUse: &model.ToolUse{
			ID:   "toolu_1",
			Name: "Read",
			Input: map[string]any{
				"file_path": "main.go",
			},
		},
	}
	close(stream)

	client := &fakeModelClient{stream: stream}
	runtime := New(client, "claude-sonnet-4-5")

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
}

// TestRuntimeRunConvertsProviderErrors verifies provider error stream items become runtime error events.
func TestRuntimeRunConvertsProviderErrors(t *testing.T) {
	stream := make(chan model.Event, 2)
	stream <- model.Event{
		Type:  model.EventTypeError,
		Error: "provider failed",
	}
	close(stream)

	client := &fakeModelClient{stream: stream}
	runtime := New(client, "claude-sonnet-4-5")

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
