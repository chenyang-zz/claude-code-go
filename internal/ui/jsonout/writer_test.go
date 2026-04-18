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
	if payload["Name"] != "Glob" {
		t.Fatalf("payload.Name = %v, want Glob", payload["Name"])
	}
	if payload["ID"] != "toolu_1" {
		t.Fatalf("payload.ID = %v, want toolu_1", payload["ID"])
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
