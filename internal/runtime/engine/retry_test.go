package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/compact"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

type wrappedNetError struct {
	timeout bool
}

func (e wrappedNetError) Error() string   { return "wrapped network error" }
func (e wrappedNetError) Timeout() bool   { return e.timeout }
func (e wrappedNetError) Temporary() bool { return true }

// TestRuntimeRunStopReasonEndTurn verifies that end_turn stop reason is captured in the done event.
func TestRuntimeRunStopReasonEndTurn(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "hello"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn, Usage: &model.Usage{InputTokens: 10, OutputTokens: 5}},
			),
		},
	}
	runtime := New(client, "claude-sonnet-4-5", nil)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "hi",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var usageEvents []event.Event
	var doneEvent event.Event
	for evt := range out {
		if evt.Type == event.TypeUsage {
			usageEvents = append(usageEvents, evt)
		}
		if evt.Type == event.TypeConversationDone {
			doneEvent = evt
		}
	}

	// Usage event should carry end_turn stop reason.
	if len(usageEvents) != 1 {
		t.Fatalf("usage event count = %d, want 1", len(usageEvents))
	}
	usagePayload, ok := usageEvents[0].Payload.(event.UsagePayload)
	if !ok {
		t.Fatalf("usage payload type = %T, want UsagePayload", usageEvents[0].Payload)
	}
	if usagePayload.StopReason != "end_turn" {
		t.Fatalf("stop reason = %q, want end_turn", usagePayload.StopReason)
	}

	// ConversationDone should exist.
	if doneEvent.Type != event.TypeConversationDone {
		t.Fatalf("done event type = %q, want conversation.done", doneEvent.Type)
	}
}

// TestRuntimeRunUsageAccumulation verifies that usage is accumulated across multiple model calls in the tool loop.
func TestRuntimeRunUsageAccumulation(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{Type: model.EventTypeToolUse, ToolUse: &model.ToolUse{ID: "t1", Name: "Read"}},
				model.Event{Type: model.EventTypeDone, Usage: &model.Usage{InputTokens: 100, OutputTokens: 50}},
			),
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "done"},
				model.Event{Type: model.EventTypeDone, Usage: &model.Usage{InputTokens: 200, OutputTokens: 30}},
			),
		},
	}
	executor := &fakeToolExecutor{
		results: map[string]coretool.Result{"Read": {Output: "file"}},
	}
	runtime := New(client, "claude-sonnet-4-5", executor)

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "read",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var usageEvents []event.UsagePayload
	var donePayload event.ConversationDonePayload
	for evt := range out {
		if evt.Type == event.TypeUsage {
			p, ok := evt.Payload.(event.UsagePayload)
			if !ok {
				t.Fatalf("usage payload type = %T", evt.Payload)
			}
			usageEvents = append(usageEvents, p)
		}
		if evt.Type == event.TypeConversationDone {
			p, ok := evt.Payload.(event.ConversationDonePayload)
			if !ok {
				t.Fatalf("done payload type = %T", evt.Payload)
			}
			donePayload = p
		}
	}

	// Two usage events: one per model call.
	if len(usageEvents) != 2 {
		t.Fatalf("usage event count = %d, want 2", len(usageEvents))
	}

	// First usage event: turn-only (100 input, 50 output).
	if usageEvents[0].TurnUsage.InputTokens != 100 || usageEvents[0].TurnUsage.OutputTokens != 50 {
		t.Fatalf("first turn usage = %+v", usageEvents[0].TurnUsage)
	}
	if usageEvents[0].CumulativeUsage.InputTokens != 100 {
		t.Fatalf("first cumulative input = %d, want 100", usageEvents[0].CumulativeUsage.InputTokens)
	}

	// Second usage event: cumulative should be 300 input (100+200), 80 output (50+30).
	if usageEvents[1].CumulativeUsage.InputTokens != 300 {
		t.Fatalf("second cumulative input = %d, want 300", usageEvents[1].CumulativeUsage.InputTokens)
	}
	if usageEvents[1].CumulativeUsage.OutputTokens != 80 {
		t.Fatalf("second cumulative output = %d, want 80", usageEvents[1].CumulativeUsage.OutputTokens)
	}

	// ConversationDone should carry cumulative usage.
	if donePayload.Usage.InputTokens != 300 || donePayload.Usage.OutputTokens != 80 {
		t.Fatalf("done usage = %+v, want 300/80", donePayload.Usage)
	}
}

func TestRuntimeRunUsageIncludesAutoCompactSummaryCall(t *testing.T) {
	t.Setenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW", "40000")

	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "<summary>carry forward</summary>"},
				model.Event{Type: model.EventTypeDone, Usage: &model.Usage{InputTokens: 80, OutputTokens: 20}},
			),
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "done"},
				model.Event{Type: model.EventTypeDone, Usage: &model.Usage{InputTokens: 200, OutputTokens: 30}},
			),
		},
	}

	runtime := New(client, "claude-sonnet-4-20250514", nil)
	runtime.AutoCompact = true

	largeInput := message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart(strings.Repeat("x", compact.GetAutoCompactThreshold("claude-sonnet-4-20250514")*4)),
		},
	}

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Messages:  []message.Message{largeInput},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var usageEvents []event.UsagePayload
	var donePayload event.ConversationDonePayload
	for evt := range out {
		if evt.Type == event.TypeUsage {
			p, ok := evt.Payload.(event.UsagePayload)
			if !ok {
				t.Fatalf("usage payload type = %T", evt.Payload)
			}
			usageEvents = append(usageEvents, p)
		}
		if evt.Type == event.TypeConversationDone {
			p, ok := evt.Payload.(event.ConversationDonePayload)
			if !ok {
				t.Fatalf("done payload type = %T", evt.Payload)
			}
			donePayload = p
		}
	}

	if len(usageEvents) != 2 {
		t.Fatalf("usage event count = %d, want 2", len(usageEvents))
	}
	if usageEvents[0].TurnUsage.InputTokens != 80 || usageEvents[0].TurnUsage.OutputTokens != 20 {
		t.Fatalf("first turn usage = %+v, want 80/20", usageEvents[0].TurnUsage)
	}
	if usageEvents[1].CumulativeUsage.InputTokens != 280 || usageEvents[1].CumulativeUsage.OutputTokens != 50 {
		t.Fatalf("second cumulative usage = %+v, want 280/50", usageEvents[1].CumulativeUsage)
	}
	if donePayload.Usage.InputTokens != 280 || donePayload.Usage.OutputTokens != 50 {
		t.Fatalf("done usage = %+v, want 280/50", donePayload.Usage)
	}
}

// TestRetryPolicyBackoffDuration verifies exponential backoff grows and respects max cap.
func TestRetryPolicyBackoffDuration(t *testing.T) {
	policy := RetryPolicy{
		MaxAttempts:    5,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
	}

	// Attempt 1: ~100ms + jitter
	d1 := policy.backoffDuration(1)
	if d1 < 100*time.Millisecond || d1 > 200*time.Millisecond {
		t.Fatalf("attempt 1 backoff = %v, want ~100-125ms", d1)
	}

	// Attempt 3: ~400ms + jitter
	d3 := policy.backoffDuration(3)
	if d3 < 400*time.Millisecond || d3 > 600*time.Millisecond {
		t.Fatalf("attempt 3 backoff = %v, want ~400-500ms", d3)
	}

	// Attempt 10: should be capped at MaxBackoff + jitter
	d10 := policy.backoffDuration(10)
	if d10 < 1*time.Second || d10 > 1300*time.Millisecond {
		t.Fatalf("attempt 10 backoff = %v, want ~1000-1250ms (capped)", d10)
	}
}

// TestIsRetriableError verifies error classification for retry decisions.
func TestIsRetriableError(t *testing.T) {
	tests := []struct {
		errMsg string
		want   bool
	}{
		{"connection refused", true},
		{"connection reset by peer", true},
		{"dial tcp: ECONNREFUSED", true},
		{"http 529: overloaded", true},
		{"server error 500", true},
		{"bad gateway 502", true},
		{"service unavailable 503", true},
		{"gateway timeout 504", true},
		{"rate_limit exceeded 429", true},
		{"request timeout 408", true},
		{"unauthorized 401", false},
		{"forbidden 403", false},
		{"not found 404", false},
		{"bad request 400", false},
	}
	for _, tt := range tests {
		got := isRetriableError(errors.New(tt.errMsg))
		if got != tt.want {
			t.Errorf("isRetriableError(%q) = %v, want %v", tt.errMsg, got, tt.want)
		}
	}
}

func TestIsNetworkErrorMatchesWrappedNetError(t *testing.T) {
	var err error = fmt.Errorf("stream failed: %w", wrappedNetError{timeout: true})
	if !isNetworkError(err) {
		t.Fatal("isNetworkError() = false, want true for wrapped net.Error")
	}
}

// TestRuntimeRunRetryOnTransientError verifies the engine retries transient errors and succeeds.
func TestRuntimeRunRetryOnTransientError(t *testing.T) {
	var attempts int32
	client := &fakeModelClient{
		streams: []model.Stream{},
	}
	// Override Stream to fail twice then succeed.
	origStream := client.Stream
	_ = origStream
	client.streamFn = func(ctx context.Context, req model.Request) (model.Stream, error) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 2 {
			return nil, errors.New("connection reset by peer")
		}
		return newModelStream(
			model.Event{Type: model.EventTypeTextDelta, Text: "recovered"},
			model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
		), nil
	}

	runtime := New(client, "claude-sonnet-4-5", nil)
	runtime.RetryPolicy = RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
	}

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "test",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var last event.Event
	for evt := range out {
		last = evt
	}
	if last.Type != event.TypeConversationDone {
		t.Fatalf("last event type = %q, want conversation.done", last.Type)
	}

	finalAttempts := atomic.LoadInt32(&attempts)
	if finalAttempts != 3 {
		t.Fatalf("total attempts = %d, want 3 (2 failures + 1 success)", finalAttempts)
	}
}

// TestRuntimeRunRetryExhausted verifies the engine returns an error after exhausting retries.
func TestRuntimeRunRetryExhausted(t *testing.T) {
	client := &fakeModelClient{}
	client.streamFn = func(ctx context.Context, req model.Request) (model.Stream, error) {
		return nil, errors.New("503 service unavailable")
	}

	runtime := New(client, "claude-sonnet-4-5", nil)
	runtime.RetryPolicy = RetryPolicy{
		MaxAttempts:    2,
		InitialBackoff: 5 * time.Millisecond,
		MaxBackoff:     20 * time.Millisecond,
	}

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "test",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// The error should be surfaced as an error event.
	var evt event.Event
	for e := range out {
		evt = e
	}
	if evt.Type != event.TypeError {
		t.Fatalf("event type = %q, want error", evt.Type)
	}
	payload, ok := evt.Payload.(event.ErrorPayload)
	if !ok {
		t.Fatalf("payload type = %T, want ErrorPayload", evt.Payload)
	}
	if payload.Message == "" {
		t.Fatal("error message is empty")
	}
}

// TestRuntimeRunFallbackModel verifies the engine switches to the fallback model on primary failure.
func TestRuntimeRunFallbackModel(t *testing.T) {
	client := &fakeModelClient{}
	var modelsUsed []string
	client.streamFn = func(ctx context.Context, req model.Request) (model.Stream, error) {
		modelsUsed = append(modelsUsed, req.Model)
		if req.Model == "primary-model" {
			// Primary fails with retriable error (no retry policy to keep test fast).
			return nil, errors.New("529 overloaded")
		}
		// Fallback succeeds.
		return newModelStream(
			model.Event{Type: model.EventTypeTextDelta, Text: "fallback response"},
			model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
		), nil
	}

	runtime := New(client, "primary-model", nil)
	runtime.FallbackModel = "fallback-model"
	runtime.RetryPolicy = RetryPolicy{MaxAttempts: 0} // Disable retry to test pure fallback.

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "test",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var events []event.Event
	for evt := range out {
		events = append(events, evt)
	}

	// Should have used primary first, then fallback.
	if len(modelsUsed) != 2 {
		t.Fatalf("models used = %v, want 2 attempts", modelsUsed)
	}
	if modelsUsed[0] != "primary-model" {
		t.Fatalf("first model = %q, want primary-model", modelsUsed[0])
	}
	if modelsUsed[1] != "fallback-model" {
		t.Fatalf("second model = %q, want fallback-model", modelsUsed[1])
	}

	// Last event should be conversation.done.
	last := events[len(events)-1]
	if last.Type != event.TypeConversationDone {
		t.Fatalf("last event type = %q, want conversation.done", last.Type)
	}
}

// TestRuntimeRunNonRetriableErrorBypassesFallback verifies non-retriable errors do not trigger fallback.
func TestRuntimeRunNonRetriableErrorBypassesFallback(t *testing.T) {
	client := &fakeModelClient{}
	client.streamFn = func(ctx context.Context, req model.Request) (model.Stream, error) {
		return nil, errors.New("401 unauthorized")
	}

	runtime := New(client, "primary-model", nil)
	runtime.FallbackModel = "fallback-model"
	runtime.RetryPolicy = RetryPolicy{MaxAttempts: 0} // Disable retry for non-retriable error test.

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "test",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var evt event.Event
	for e := range out {
		evt = e
	}
	if evt.Type != event.TypeError {
		t.Fatalf("event type = %q, want error", evt.Type)
	}
}

// TestRuntimeRunRetryEmitsEvents verifies retry attempts emit TypeRetryAttempted events.
func TestRuntimeRunRetryEmitsEvents(t *testing.T) {
	var attempts int32
	client := &fakeModelClient{}
	client.streamFn = func(ctx context.Context, req model.Request) (model.Stream, error) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 2 {
			return nil, errors.New("503 service unavailable")
		}
		return newModelStream(
			model.Event{Type: model.EventTypeTextDelta, Text: "ok"},
			model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
		), nil
	}

	runtime := New(client, "claude-sonnet-4-5", nil)
	runtime.RetryPolicy = RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 5 * time.Millisecond,
		MaxBackoff:     20 * time.Millisecond,
	}

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "test",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var retryEvents []event.RetryAttemptedPayload
	for evt := range out {
		if evt.Type == event.TypeRetryAttempted {
			p, ok := evt.Payload.(event.RetryAttemptedPayload)
			if !ok {
				t.Fatalf("retry payload type = %T", evt.Payload)
			}
			retryEvents = append(retryEvents, p)
		}
	}

	if len(retryEvents) != 2 {
		t.Fatalf("retry event count = %d, want 2", len(retryEvents))
	}
	if retryEvents[0].Attempt != 1 || retryEvents[1].Attempt != 2 {
		t.Fatalf("retry attempts = %d, %d; want 1, 2", retryEvents[0].Attempt, retryEvents[1].Attempt)
	}
	if retryEvents[0].Error != "503 service unavailable" {
		t.Fatalf("retry error = %q, want 503 service unavailable", retryEvents[0].Error)
	}
}

// TestRuntimeRunFallbackEmitsEvent verifies fallback model switch emits TypeModelFallback event.
func TestRuntimeRunFallbackEmitsEvent(t *testing.T) {
	client := &fakeModelClient{}
	client.streamFn = func(ctx context.Context, req model.Request) (model.Stream, error) {
		if req.Model == "primary-model" {
			return nil, errors.New("529 overloaded")
		}
		return newModelStream(
			model.Event{Type: model.EventTypeTextDelta, Text: "fallback"},
			model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
		), nil
	}

	runtime := New(client, "primary-model", nil)
	runtime.FallbackModel = "fallback-model"
	runtime.RetryPolicy = RetryPolicy{MaxAttempts: 0}

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "test",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var fallbackEvents []event.ModelFallbackPayload
	for evt := range out {
		if evt.Type == event.TypeModelFallback {
			p, ok := evt.Payload.(event.ModelFallbackPayload)
			if !ok {
				t.Fatalf("fallback payload type = %T", evt.Payload)
			}
			fallbackEvents = append(fallbackEvents, p)
		}
	}

	if len(fallbackEvents) != 1 {
		t.Fatalf("fallback event count = %d, want 1", len(fallbackEvents))
	}
	if fallbackEvents[0].OriginalModel != "primary-model" {
		t.Fatalf("original model = %q, want primary-model", fallbackEvents[0].OriginalModel)
	}
	if fallbackEvents[0].FallbackModel != "fallback-model" {
		t.Fatalf("fallback model = %q, want fallback-model", fallbackEvents[0].FallbackModel)
	}
}

func TestRuntimeRunFallbackDoesNotEmitEventWhenFallbackStreamFails(t *testing.T) {
	client := &fakeModelClient{}
	client.streamFn = func(ctx context.Context, req model.Request) (model.Stream, error) {
		if req.Model == "primary-model" {
			return nil, errors.New("529 overloaded")
		}
		return newModelStream(
			model.Event{Type: model.EventTypeTextDelta, Text: "partial fallback"},
			model.Event{Type: model.EventTypeError, Error: "503 service unavailable"},
		), nil
	}

	runtime := New(client, "primary-model", nil)
	runtime.FallbackModel = "fallback-model"
	runtime.RetryPolicy = RetryPolicy{MaxAttempts: 0}

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "test",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for evt := range out {
		if evt.Type == event.TypeModelFallback {
			t.Fatal("received model.fallback event for failed fallback stream")
		}
	}
}

// TestRuntimeRunRetriesMidStreamError verifies that an error arriving during stream consumption
// (not just during connection) triggers the retry path.
func TestRuntimeRunRetriesMidStreamError(t *testing.T) {
	var attempts int32
	client := &fakeModelClient{}
	client.streamFn = func(ctx context.Context, req model.Request) (model.Stream, error) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			// First attempt: stream opens but sends a retriable error mid-stream.
			return newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "partial..."},
				model.Event{Type: model.EventTypeError, Error: "503 service unavailable"},
			), nil
		}
		// Second attempt: succeeds.
		return newModelStream(
			model.Event{Type: model.EventTypeTextDelta, Text: "recovered"},
			model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
		), nil
	}

	runtime := New(client, "claude-sonnet-4-5", nil)
	runtime.RetryPolicy = RetryPolicy{
		MaxAttempts:    2,
		InitialBackoff: 5 * time.Millisecond,
		MaxBackoff:     20 * time.Millisecond,
	}

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "test",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var events []event.Event
	for evt := range out {
		events = append(events, evt)
	}

	// Should have retry event and then conversation.done.
	var retryCount int
	var doneFound bool
	for _, evt := range events {
		if evt.Type == event.TypeRetryAttempted {
			retryCount++
		}
		if evt.Type == event.TypeConversationDone {
			doneFound = true
		}
	}
	if retryCount != 1 {
		t.Fatalf("retry event count = %d, want 1", retryCount)
	}
	if !doneFound {
		t.Fatal("missing conversation.done event after mid-stream retry")
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Fatalf("total attempts = %d, want 2", atomic.LoadInt32(&attempts))
	}
}

// TestRuntimeRunFallbackOnMidStreamError verifies fallback triggers when a mid-stream
// retriable error exhausts retries.
func TestRuntimeRunFallbackOnMidStreamError(t *testing.T) {
	var modelsUsed []string
	client := &fakeModelClient{}
	client.streamFn = func(ctx context.Context, req model.Request) (model.Stream, error) {
		modelsUsed = append(modelsUsed, req.Model)
		if req.Model == "primary-model" {
			return newModelStream(
				model.Event{Type: model.EventTypeError, Error: "529 overloaded"},
			), nil
		}
		return newModelStream(
			model.Event{Type: model.EventTypeTextDelta, Text: "fallback ok"},
			model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
		), nil
	}

	runtime := New(client, "primary-model", nil)
	runtime.FallbackModel = "fallback-model"
	runtime.RetryPolicy = RetryPolicy{MaxAttempts: 0} // No retry — go straight to fallback.

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "cli",
		Input:     "test",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var fallbackFound bool
	for evt := range out {
		if evt.Type == event.TypeModelFallback {
			fallbackFound = true
		}
	}
	if !fallbackFound {
		t.Fatal("missing model.fallback event for mid-stream error")
	}
	// Primary tried once, then fallback.
	if len(modelsUsed) != 2 || modelsUsed[0] != "primary-model" || modelsUsed[1] != "fallback-model" {
		t.Fatalf("models used = %v, want [primary-model, fallback-model]", modelsUsed)
	}
}
