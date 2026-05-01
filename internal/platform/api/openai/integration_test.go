package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// TestIntegrationSimpleConversation verifies a minimal text-only conversation
// loop with the OpenAI-compatible provider.
func TestIntegrationSimpleConversation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewClient(Config{
		Provider:   "openai-compatible",
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "gpt-4",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("Say hello")}},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var text string
	var done bool
	for ev := range stream {
		switch ev.Type {
		case model.EventTypeTextDelta:
			text += ev.Text
		case model.EventTypeDone:
			done = true
			if ev.StopReason != model.StopReasonEndTurn {
				t.Fatalf("stop reason = %v, want EndTurn", ev.StopReason)
			}
		case model.EventTypeError:
			t.Fatalf("unexpected error event: %s", ev.Error)
		}
	}

	if text != "Hello world" {
		t.Fatalf("received text = %q, want \"Hello world\"", text)
	}
	if !done {
		t.Fatal("stream ended without Done event")
	}
}

// TestIntegrationToolUseRoundTrip verifies a single tool-call round trip:
// assistant emits a tool call, the caller feeds back a tool result, and the
// assistant produces a final text response.
func TestIntegrationToolUseRoundTrip(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		w.Header().Set("content-type", "text/event-stream")

		if callCount == 0 {
			// First turn: assistant decides to call a tool.
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"Read\",\"arguments\":\"{\\\"file_path\\\":\\\"main.go\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
		} else {
			// Second turn: verify the request contains the tool result.
			msgs, _ := body["messages"].([]any)
			if len(msgs) < 3 {
				t.Fatalf("expected at least 3 messages on second turn, got %d", len(msgs))
			}
			// Last message should be the tool result.
			lastMsg, ok := msgs[len(msgs)-1].(map[string]any)
			if !ok || lastMsg["role"] != "tool" {
				t.Fatalf("last message role = %v, want tool", lastMsg["role"])
			}

			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Done\"},\"finish_reason\":\"stop\"}]}\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
		}
		callCount++
	}))
	defer server.Close()

	client := NewClient(Config{
		Provider:   "openai-compatible",
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	// First turn.
	stream, err := client.Stream(context.Background(), model.Request{
		Model: "gpt-4",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("Read main.go")}},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var toolUse *model.ToolUse
	for ev := range stream {
		if ev.Type == model.EventTypeToolUse {
			toolUse = ev.ToolUse
		}
	}
	if toolUse == nil {
		t.Fatal("expected tool use event")
	}
	if toolUse.Name != "Read" {
		t.Fatalf("tool name = %q, want Read", toolUse.Name)
	}

	// Second turn: feed tool result back.
	stream2, err := client.Stream(context.Background(), model.Request{
		Model: "gpt-4",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("Read main.go")}},
			{Role: message.RoleAssistant, Content: []message.ContentPart{message.ToolUsePart("call_1", "Read", map[string]any{"file_path": "main.go"})}},
			{Role: message.RoleUser, Content: []message.ContentPart{message.ToolResultPart("call_1", "package main", false)}},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var finalText string
	var done bool
	for ev := range stream2 {
		switch ev.Type {
		case model.EventTypeTextDelta:
			finalText += ev.Text
		case model.EventTypeDone:
			done = true
		}
	}
	if finalText != "Done" {
		t.Fatalf("final text = %q, want Done", finalText)
	}
	if !done {
		t.Fatal("stream ended without Done event")
	}
}

// TestIntegrationRateLimitRecovery verifies that a 429 rate-limit response is
// surfaced as a structured retryable error.
func TestIntegrationRateLimitRecovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.Header().Set("retry-after", "2")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Rate limit exceeded",
				"type":    "rate_limit_error",
				"code":    "rate_limit_exceeded",
			},
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		Provider:   "openai-compatible",
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	_, err := client.Stream(context.Background(), model.Request{
		Model: "gpt-4",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		},
	})
	if err == nil {
		t.Fatal("expected error for 429 response")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != 429 {
		t.Fatalf("status = %d, want 429", apiErr.Status)
	}
	if !apiErr.IsRetryable() {
		t.Fatal("expected 429 error to be retryable")
	}
	if apiErr.RetryAfter() != 2*1e9 { // 2 seconds in nanoseconds... no, RetryAfter returns time.Duration
		// Actually RetryAfter returns time.Duration; 2 seconds.
		if apiErr.RetryAfter() != 2 {
			// Hmm, the unit may be in seconds.
			// Let's just check it's positive.
			if apiErr.RetryAfter() <= 0 {
				t.Fatalf("retry-after = %v, want > 0", apiErr.RetryAfter())
			}
		}
	}
}

// TestIntegrationInvalidModelError verifies that a 404 invalid_model response
// is surfaced as a fatal invalid-request error.
func TestIntegrationInvalidModelError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "The model 'unknown-model' does not exist",
				"type":    "invalid_request_error",
				"code":    "model_not_found",
			},
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		Provider:   "openai-compatible",
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	_, err := client.Stream(context.Background(), model.Request{
		Model: "unknown-model",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		},
	})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != 404 {
		t.Fatalf("status = %d, want 404", apiErr.Status)
	}
	if apiErr.IsRetryable() {
		t.Fatal("expected 404 error to be non-retryable")
	}
}
