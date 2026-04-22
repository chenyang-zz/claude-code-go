package console

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
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

func TestStreamRendererConversationDone(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(NewPrinter(&buf))

	err := r.RenderEvent(event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload:   event.ConversationDonePayload{},
	})
	if err != nil {
		t.Fatalf("RenderEvent() error = %v", err)
	}
	if !strings.Contains(buf.String(), "conversation complete") {
		t.Fatalf("output = %q, want conversation complete line", buf.String())
	}
}

func TestStreamRendererUsage(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(NewPrinter(&buf))

	err := r.RenderEvent(event.Event{
		Type:      event.TypeUsage,
		Timestamp: time.Now(),
		Payload: event.UsagePayload{
			TurnUsage: model.Usage{
				InputTokens:              100,
				OutputTokens:             50,
				CacheCreationInputTokens: 10,
				CacheReadInputTokens:     20,
			},
			StopReason: "end_turn",
		},
	})
	if err != nil {
		t.Fatalf("RenderEvent() error = %v", err)
	}
	want := "Usage: in=100 out=50 cache_create=10 cache_read=20 (stop: end_turn)"
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("output = %q, want %q", buf.String(), want)
	}
}

func TestStreamRendererRetryAttempted(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(NewPrinter(&buf))

	err := r.RenderEvent(event.Event{
		Type:      event.TypeRetryAttempted,
		Timestamp: time.Now(),
		Payload: event.RetryAttemptedPayload{
			Attempt:     2,
			MaxAttempts: 5,
			BackoffMs:   1500,
			Error:       "rate limit exceeded",
		},
	})
	if err != nil {
		t.Fatalf("RenderEvent() error = %v", err)
	}
	want := "Retry attempt 2/5 (backoff 1500ms): rate limit exceeded"
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("output = %q, want %q", buf.String(), want)
	}
}

func TestStreamRendererModelFallback(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(NewPrinter(&buf))

	err := r.RenderEvent(event.Event{
		Type:      event.TypeModelFallback,
		Timestamp: time.Now(),
		Payload: event.ModelFallbackPayload{
			OriginalModel: "claude-opus-4",
			FallbackModel: "claude-sonnet-4-5",
		},
	})
	if err != nil {
		t.Fatalf("RenderEvent() error = %v", err)
	}
	want := "Model fallback: claude-opus-4 -> claude-sonnet-4-5"
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("output = %q, want %q", buf.String(), want)
	}
}

func TestStreamRendererCompactDone(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(NewPrinter(&buf))

	err := r.RenderEvent(event.Event{
		Type:      event.TypeCompactDone,
		Timestamp: time.Now(),
		Payload: event.CompactDonePayload{
			PreTokenCount:  10000,
			PostTokenCount: 5000,
		},
	})
	if err != nil {
		t.Fatalf("RenderEvent() error = %v", err)
	}
	want := "Compact done: 10000 -> 5000 tokens"
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("output = %q, want %q", buf.String(), want)
	}
}

func TestStreamRendererWrongPayloadType(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamRenderer(NewPrinter(&buf))

	err := r.RenderEvent(event.Event{
		Type:      event.TypeUsage,
		Timestamp: time.Now(),
		Payload:   "not-a-usage-payload",
	})
	if err == nil {
		t.Fatalf("RenderEvent() expected error for wrong payload type, got nil")
	}
	if !strings.Contains(err.Error(), "usage payload type mismatch") {
		t.Fatalf("error = %q, want usage payload type mismatch", err.Error())
	}
}
