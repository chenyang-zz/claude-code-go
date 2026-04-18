package console

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
)

func TestStreamRendererToolCallStarted(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(NewPrinter(&buf))

	err := r.RenderEvent(event.Event{
		Type:      event.TypeToolCallStarted,
		Timestamp: time.Now(),
		Payload: event.ToolCallPayload{
			ID:   "toolu_1",
			Name: "Glob",
			Input: map[string]any{"pattern": "*.go"},
		},
	})
	if err != nil {
		t.Fatalf("RenderEvent() error = %v", err)
	}
	if !strings.Contains(buf.String(), "Tool started: Glob") {
		t.Fatalf("output = %q, want tool started line", buf.String())
	}
}

func TestStreamRendererToolCallFinished(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(NewPrinter(&buf))

	err := r.RenderEvent(event.Event{
		Type:      event.TypeToolCallFinished,
		Timestamp: time.Now(),
		Payload: event.ToolResultPayload{
			ID:      "toolu_1",
			Name:    "Glob",
			Output:  "main.go",
			IsError: false,
		},
	})
	if err != nil {
		t.Fatalf("RenderEvent() error = %v", err)
	}
	if !strings.Contains(buf.String(), "Tool finished: Glob") {
		t.Fatalf("output = %q, want tool finished line", buf.String())
	}
	if strings.Contains(buf.String(), "error") {
		t.Fatalf("output should not contain error for success, got %q", buf.String())
	}
}

func TestStreamRendererToolCallFinishedError(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(NewPrinter(&buf))

	err := r.RenderEvent(event.Event{
		Type:      event.TypeToolCallFinished,
		Timestamp: time.Now(),
		Payload: event.ToolResultPayload{
			ID:      "toolu_1",
			Name:    "Bash",
			Output:  "exit 1",
			IsError: true,
		},
	})
	if err != nil {
		t.Fatalf("RenderEvent() error = %v", err)
	}
	if !strings.Contains(buf.String(), "Tool finished: Bash (error)") {
		t.Fatalf("output = %q, want tool finished with error", buf.String())
	}
}

func TestStreamRendererApprovalRequired(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(NewPrinter(&buf))

	err := r.RenderEvent(event.Event{
		Type:      event.TypeApprovalRequired,
		Timestamp: time.Now(),
		Payload: event.ApprovalPayload{
			CallID:   "toolu_2",
			ToolName: "Bash",
			Path:     "rm -rf build",
			Action:   "execute",
			Message:  "Permission required",
		},
	})
	if err != nil {
		t.Fatalf("RenderEvent() error = %v", err)
	}
	if !strings.Contains(buf.String(), "Approval required: Bash wants to execute rm -rf build") {
		t.Fatalf("output = %q, want approval line", buf.String())
	}
}

func TestStreamRendererApprovalRequiredMinimal(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(NewPrinter(&buf))

	err := r.RenderEvent(event.Event{
		Type:      event.TypeApprovalRequired,
		Timestamp: time.Now(),
		Payload: event.ApprovalPayload{
			CallID:   "toolu_3",
			ToolName: "Write",
		},
	})
	if err != nil {
		t.Fatalf("RenderEvent() error = %v", err)
	}
	if !strings.Contains(buf.String(), "Approval required: Write") {
		t.Fatalf("output = %q, want minimal approval line", buf.String())
	}
}
