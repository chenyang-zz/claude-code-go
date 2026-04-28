package repl

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/internal/platform/remote"
	"github.com/sheepzhao/claude-code-go/pkg/sdk"
)

// recordingRenderer captures rendered events for test assertions.
type recordingRenderer struct {
	events []event.Event
	lines  []string
	err    error
}

func (r *recordingRenderer) RenderEvent(evt event.Event) error {
	if r.err != nil {
		return r.err
	}
	r.events = append(r.events, evt)
	return nil
}

func (r *recordingRenderer) RenderLine(text string) error {
	r.lines = append(r.lines, text)
	return nil
}

func TestConvertRemoteEventStreamEventTextDelta(t *testing.T) {
	t.Parallel()
	data, _ := json.Marshal(map[string]any{
		"type":  "stream_event",
		"event": map[string]any{"type": "text_delta", "text": "hello"},
	})
	evt := remote.Event{Data: data}
	got, err := ConvertRemoteEvent(evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected event, got nil")
	}
	if got.Type != event.TypeMessageDelta {
		t.Fatalf("type = %q, want %q", got.Type, event.TypeMessageDelta)
	}
	payload, ok := got.Payload.(event.MessageDeltaPayload)
	if !ok {
		t.Fatalf("payload type mismatch")
	}
	if payload.Text != "hello" {
		t.Fatalf("text = %q, want %q", payload.Text, "hello")
	}
}

func TestConvertRemoteEventResultError(t *testing.T) {
	t.Parallel()
	data, _ := json.Marshal(sdk.Result{
		Base:    sdk.Base{Type: "result"},
		Subtype: "error_during_execution",
		IsError: true,
		Errors:  []string{"something went wrong"},
	})
	evt := remote.Event{Data: data}
	got, err := ConvertRemoteEvent(evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected event, got nil")
	}
	if got.Type != event.TypeError {
		t.Fatalf("type = %q, want %q", got.Type, event.TypeError)
	}
	payload := got.Payload.(event.ErrorPayload)
	if payload.Message != "something went wrong" {
		t.Fatalf("message = %q, want %q", payload.Message, "something went wrong")
	}
}

func TestConvertRemoteEventResultSuccess(t *testing.T) {
	t.Parallel()
	stopReason := "end_turn"
	data, _ := json.Marshal(sdk.Result{
		Base:       sdk.Base{Type: "result"},
		Subtype:    "success",
		IsError:    false,
		Usage:      model.Usage{InputTokens: 10, OutputTokens: 5},
		StopReason: &stopReason,
	})
	evt := remote.Event{Data: data}
	got, err := ConvertRemoteEvent(evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected event, got nil")
	}
	if got.Type != event.TypeConversationDone {
		t.Fatalf("type = %q, want %q", got.Type, event.TypeConversationDone)
	}
	payload := got.Payload.(event.ConversationDonePayload)
	if payload.StopReason != "end_turn" {
		t.Fatalf("stop_reason = %q, want %q", payload.StopReason, "end_turn")
	}
	if payload.Usage.InputTokens != 10 {
		t.Fatalf("input_tokens = %d, want 10", payload.Usage.InputTokens)
	}
}

func TestConvertRemoteEventToolProgress(t *testing.T) {
	t.Parallel()
	parentID := "parent-123"
	data, _ := json.Marshal(sdk.ToolProgress{
		Base:            sdk.Base{Type: "tool_progress"},
		ToolUseID:       "tool-123",
		ToolName:        "Bash",
		ParentToolUseID: &parentID,
		ElapsedTimeSec:  3.5,
	})
	evt := remote.Event{Data: data}
	got, err := ConvertRemoteEvent(evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected event, got nil")
	}
	if got.Type != event.TypeProgress {
		t.Fatalf("type = %q, want %q", got.Type, event.TypeProgress)
	}
	payload := got.Payload.(event.ProgressPayload)
	if payload.ToolUseID != "tool-123" {
		t.Fatalf("tool_use_id = %q, want %q", payload.ToolUseID, "tool-123")
	}
	if payload.ParentToolUseID != "parent-123" {
		t.Fatalf("parent_tool_use_id = %q, want %q", payload.ParentToolUseID, "parent-123")
	}
}

func TestConvertRemoteEventToolProgressWithTypedPayload(t *testing.T) {
	t.Parallel()
	data, _ := json.Marshal(sdk.ToolProgress{
		Base:      sdk.Base{Type: "tool_progress"},
		ToolUseID: "tool-typed",
		ToolName:  "Agent",
		Progress: map[string]any{
			"type":       "agent_tool_progress",
			"status":     "started",
			"agent_type": "explore",
		},
	})
	evt := remote.Event{Data: data}
	got, err := ConvertRemoteEvent(evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected event, got nil")
	}
	payload := got.Payload.(event.ProgressPayload)
	progress, ok := payload.Data.(map[string]any)
	if !ok {
		t.Fatalf("payload data type = %T, want map[string]any", payload.Data)
	}
	if progress["type"] != "agent_tool_progress" {
		t.Fatalf("progress type = %v, want agent_tool_progress", progress["type"])
	}
}

func TestConvertRemoteEventSystemCompactBoundary(t *testing.T) {
	t.Parallel()
	data, _ := json.Marshal(map[string]any{
		"type":    "system",
		"subtype": "compact_boundary",
	})
	evt := remote.Event{Data: data}
	got, err := ConvertRemoteEvent(evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected event, got nil")
	}
	if got.Type != event.TypeCompactDone {
		t.Fatalf("type = %q, want %q", got.Type, event.TypeCompactDone)
	}
}

func TestConvertRemoteEventIgnoredTypes(t *testing.T) {
	t.Parallel()
	cases := []string{"assistant", "user", "auth_status", "tool_use_summary", "rate_limit_event"}
	for _, typ := range cases {
		data, _ := json.Marshal(map[string]any{"type": typ})
		evt := remote.Event{Data: data}
		got, err := ConvertRemoteEvent(evt)
		if err != nil {
			t.Fatalf("type=%s: unexpected error: %v", typ, err)
		}
		if got != nil {
			t.Fatalf("type=%s: expected nil, got %+v", typ, got)
		}
	}
}

func TestConvertRemoteEventUnknownType(t *testing.T) {
	t.Parallel()
	data, _ := json.Marshal(map[string]any{"type": "unknown_fancy_type"})
	evt := remote.Event{Data: data}
	got, err := ConvertRemoteEvent(evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for unknown type, got %+v", got)
	}
}

func TestConvertRemoteEventInvalidJSON(t *testing.T) {
	t.Parallel()
	evt := remote.Event{Data: []byte("not json")}
	_, err := ConvertRemoteEvent(evt)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestRemoteEventBridgeOnEventRenders(t *testing.T) {
	t.Parallel()
	renderer := &recordingRenderer{}
	bridge := &RemoteEventBridge{Renderer: renderer}

	data, _ := json.Marshal(map[string]any{
		"type":  "stream_event",
		"event": map[string]any{"type": "text_delta", "text": "world"},
	})
	bridge.OnEvent()(remote.Event{Data: data})

	if len(renderer.events) != 1 {
		t.Fatalf("rendered %d events, want 1", len(renderer.events))
	}
	if renderer.events[0].Type != event.TypeMessageDelta {
		t.Fatalf("type = %q, want %q", renderer.events[0].Type, event.TypeMessageDelta)
	}
}

func TestRemoteEventBridgeOnEventRenderError(t *testing.T) {
	t.Parallel()
	renderer := &recordingRenderer{err: errors.New("render boom")}
	bridge := &RemoteEventBridge{Renderer: renderer}

	data, _ := json.Marshal(map[string]any{
		"type":  "stream_event",
		"event": map[string]any{"type": "text_delta", "text": "world"},
	})
	// Should not panic; event is swallowed silently after logging.
	bridge.OnEvent()(remote.Event{Data: data})
}

func TestRemoteEventBridgeOnEventInvalidJSON(t *testing.T) {
	t.Parallel()
	renderer := &recordingRenderer{}
	bridge := &RemoteEventBridge{Renderer: renderer}

	bridge.OnEvent()(remote.Event{Data: []byte("bad json")})

	if len(renderer.events) != 1 {
		t.Fatalf("rendered %d events, want 1", len(renderer.events))
	}
	if renderer.events[0].Type != event.TypeError {
		t.Fatalf("type = %q, want %q", renderer.events[0].Type, event.TypeError)
	}
}

func TestConvertRemoteEventStreamEventUnknownSubtype(t *testing.T) {
	t.Parallel()
	data, _ := json.Marshal(map[string]any{
		"type":  "stream_event",
		"event": map[string]any{"type": "input_json_delta"},
	})
	evt := remote.Event{Data: data}
	got, err := ConvertRemoteEvent(evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for unhandled stream_event subtype, got %+v", got)
	}
}

func TestConvertRemoteEventTimestampIsSet(t *testing.T) {
	t.Parallel()
	data, _ := json.Marshal(map[string]any{
		"type":  "stream_event",
		"event": map[string]any{"type": "text_delta", "text": "x"},
	})
	got, _ := ConvertRemoteEvent(remote.Event{Data: data})
	if got == nil {
		t.Fatal("expected event")
	}
	if got.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
	if time.Since(got.Timestamp) > time.Second {
		t.Fatal("timestamp too old")
	}
}

func TestExtractUsage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		usage any
		want  model.Usage
	}{
		{
			name:  "nil",
			usage: nil,
			want:  model.Usage{},
		},
		{
			name:  "compatible map",
			usage: map[string]any{"input_tokens": 1, "output_tokens": 2},
			want:  model.Usage{InputTokens: 1, OutputTokens: 2},
		},
		{
			name:  "incompatible type",
			usage: "not a struct",
			want:  model.Usage{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractUsage(tc.usage)
			if got != tc.want {
				t.Fatalf("extractUsage() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
