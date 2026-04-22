package jsonout

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
)

func TestWriterWriteEventToolCallStarted(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	err := w.WriteEvent(event.Event{
		Type:      event.TypeToolCallStarted,
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Payload: event.ToolCallPayload{
			ID:   "toolu_1",
			Name: "Glob",
			Input: map[string]any{
				"pattern": "**/*.go",
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	line := strings.TrimSpace(buf.String())
	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, line)
	}
	if got := parsed["type"]; got != "tool.call.started" {
		t.Fatalf("type = %v, want tool.call.started", got)
	}
	payload, _ := parsed["payload"].(map[string]any)
	if payload["name"] != "Glob" {
		t.Fatalf("payload.name = %v, want Glob", payload["name"])
	}
	if payload["id"] != "toolu_1" {
		t.Fatalf("payload.id = %v, want toolu_1", payload["id"])
	}
}

func TestWriterWriteEventToolCallFinished(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	err := w.WriteEvent(event.Event{
		Type:      event.TypeToolCallFinished,
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Payload: event.ToolResultPayload{
			ID:      "toolu_1",
			Name:    "Glob",
			Output:  "main.go",
			IsError: false,
		},
	})
	if err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	line := strings.TrimSpace(buf.String())
	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got := parsed["type"]; got != "tool.call.finished" {
		t.Fatalf("type = %v, want tool.call.finished", got)
	}
	payload, _ := parsed["payload"].(map[string]any)
	if payload["is_error"] != false {
		t.Fatalf("payload.is_error = %v, want false", payload["is_error"])
	}
}

func TestWriterWriteEventApprovalRequired(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	err := w.WriteEvent(event.Event{
		Type:      event.TypeApprovalRequired,
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Payload: event.ApprovalPayload{
			CallID:   "toolu_2",
			ToolName: "Bash",
			Path:     "rm -rf build",
			Action:   "execute",
			Message:  "Permission required",
		},
	})
	if err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	line := strings.TrimSpace(buf.String())
	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got := parsed["type"]; got != "approval.required" {
		t.Fatalf("type = %v, want approval.required", got)
	}
	payload, _ := parsed["payload"].(map[string]any)
	if payload["tool_name"] != "Bash" {
		t.Fatalf("payload.tool_name = %v, want Bash", payload["tool_name"])
	}
}

func TestWriterConsumeStream(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	ch := make(chan event.Event, 3)
	ch <- event.Event{Type: event.TypeToolCallStarted, Timestamp: time.Now(), Payload: event.ToolCallPayload{ID: "t1", Name: "Read"}}
	ch <- event.Event{Type: event.TypeToolCallFinished, Timestamp: time.Now(), Payload: event.ToolResultPayload{ID: "t1", Name: "Read"}}
	ch <- event.Event{Type: event.TypeConversationDone, Timestamp: time.Now(), Payload: nil}
	close(ch)

	if err := w.Consume(ch); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("line count = %d, want 3", len(lines))
	}
}

func TestWriterWriteEventAllTypes(t *testing.T) {
	events := []event.Event{
		{Type: event.TypeMessageDelta, Timestamp: time.Now(), Payload: event.MessageDeltaPayload{Text: "hello"}},
		{Type: event.TypeToolCallStarted, Timestamp: time.Now(), Payload: event.ToolCallPayload{ID: "t1", Name: "Bash"}},
		{Type: event.TypeToolCallFinished, Timestamp: time.Now(), Payload: event.ToolResultPayload{ID: "t1", Name: "Bash", Output: "ok"}},
		{Type: event.TypeApprovalRequired, Timestamp: time.Now(), Payload: event.ApprovalPayload{ToolName: "Write"}},
		{Type: event.TypeConversationDone, Timestamp: time.Now(), Payload: nil},
		{Type: event.TypeError, Timestamp: time.Now(), Payload: event.ErrorPayload{Message: "fail"}},
		{Type: event.TypeUsage, Timestamp: time.Now(), Payload: event.UsagePayload{StopReason: "end_turn"}},
		{Type: event.TypeRetryAttempted, Timestamp: time.Now(), Payload: event.RetryAttemptedPayload{Attempt: 1, MaxAttempts: 3}},
		{Type: event.TypeModelFallback, Timestamp: time.Now(), Payload: event.ModelFallbackPayload{OriginalModel: "a", FallbackModel: "b"}},
		{Type: event.TypeCompactDone, Timestamp: time.Now(), Payload: event.CompactDonePayload{PreTokenCount: 1000, PostTokenCount: 500}},
		{Type: event.TypeProgress, Timestamp: time.Now(), Payload: event.ProgressPayload{ToolUseID: "t2"}},
	}

	for _, evt := range events {
		var buf bytes.Buffer
		w := NewWriter(&buf)

		err := w.WriteEvent(evt)
		if err != nil {
			t.Fatalf("WriteEvent(%s) error = %v", evt.Type, err)
		}

		line := strings.TrimSpace(buf.String())
		var parsed map[string]any
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Fatalf("output for %s is not valid JSON: %v\noutput: %q", evt.Type, err, line)
		}
		if got := parsed["type"]; got != string(evt.Type) {
			t.Fatalf("type = %v, want %v", got, evt.Type)
		}
		if _, hasTimestamp := parsed["timestamp"]; !hasTimestamp {
			t.Fatalf("missing timestamp field for %s", evt.Type)
		}
	}
}

func TestWriterWriteEventOmitsEmptyPayload(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	err := w.WriteEvent(event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Payload:   nil,
	})
	if err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	line := strings.TrimSpace(buf.String())
	if strings.Contains(line, `"payload"`) {
		t.Fatalf("nil payload should be omitted, got: %s", line)
	}
}

func TestWriterRenderEvent(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	evt := event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Payload:   event.MessageDeltaPayload{Text: "hello"},
	}
	if err := w.RenderEvent(evt); err != nil {
		t.Fatalf("RenderEvent() error = %v", err)
	}

	line := strings.TrimSpace(buf.String())
	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["type"] != "message.delta" {
		t.Fatalf("type = %v, want message.delta", parsed["type"])
	}
}

func TestWriterRenderLine(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.RenderLine("system notification"); err != nil {
		t.Fatalf("RenderLine() error = %v", err)
	}

	line := strings.TrimSpace(buf.String())
	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["type"] != "message.delta" {
		t.Fatalf("type = %v, want message.delta", parsed["type"])
	}
	payload, _ := parsed["payload"].(map[string]any)
	if payload["text"] != "system notification" {
		t.Fatalf("payload.text = %v, want system notification", payload["text"])
	}
}
