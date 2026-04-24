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

// TestClientStreamReadsTextDeltaAndToolUse verifies OpenAI-compatible SSE text and tool-call deltas are mapped into shared model events.
func TestClientStreamReadsTextDeltaAndToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != defaultChatCompletionsPath {
			t.Fatalf("request path = %q, want %q", r.URL.Path, defaultChatCompletionsPath)
		}
		if got := r.Header.Get("authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q, want Bearer test-key", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if body["model"] != "gpt-5" {
			t.Fatalf("request model = %#v, want gpt-5", body["model"])
		}

		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"Read\",\"arguments\":\"{\\\"file_path\\\":\\\"main.go\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n"))
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
		Model: "gpt-5",
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					message.TextPart("hello"),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	first := <-stream
	if first.Type != model.EventTypeTextDelta || first.Text != "hello" {
		t.Fatalf("first stream event = %#v, want text delta hello", first)
	}

	second := <-stream
	if second.Type != model.EventTypeToolUse || second.ToolUse == nil {
		t.Fatalf("second stream event = %#v, want tool use", second)
	}
	if second.ToolUse.ID != "call_1" || second.ToolUse.Name != "Read" {
		t.Fatalf("tool use payload = %#v, want call_1/Read", second.ToolUse)
	}
	if got := second.ToolUse.Input["file_path"]; got != "main.go" {
		t.Fatalf("tool use input file_path = %#v, want main.go", got)
	}
}

// TestClientStreamMapsToolLoopMessages verifies assistant tool_calls and user tool results are preserved in the OpenAI-compatible request body.
func TestClientStreamMapsToolLoopMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		messages, ok := body["messages"].([]any)
		if !ok || len(messages) != 3 {
			t.Fatalf("request messages = %#v, want 3 messages", body["messages"])
		}

		assistant, ok := messages[1].(map[string]any)
		if !ok {
			t.Fatalf("assistant message = %#v, want object", messages[1])
		}
		toolCalls, ok := assistant["tool_calls"].([]any)
		if !ok || len(toolCalls) != 1 {
			t.Fatalf("assistant tool_calls = %#v, want 1 tool call", assistant["tool_calls"])
		}
		toolCall, ok := toolCalls[0].(map[string]any)
		if !ok {
			t.Fatalf("assistant tool_call = %#v, want object", toolCalls[0])
		}
		if toolCall["id"] != "call_1" {
			t.Fatalf("assistant tool_call id = %#v, want call_1", toolCall["id"])
		}

		toolMessage, ok := messages[2].(map[string]any)
		if !ok {
			t.Fatalf("tool message = %#v, want object", messages[2])
		}
		if toolMessage["role"] != "tool" || toolMessage["tool_call_id"] != "call_1" || toolMessage["content"] != "file contents" {
			t.Fatalf("tool message = %#v, want role tool with tool_call_id/content", toolMessage)
		}

		w.Header().Set("content-type", "text/event-stream")
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
		Model: "gpt-5",
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					message.TextPart("read the file"),
				},
			},
			{
				Role: message.RoleAssistant,
				Content: []message.ContentPart{
					message.ToolUsePart("call_1", "Read", map[string]any{"file_path": "main.go"}),
				},
			},
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					message.ToolResultPart("call_1", "file contents", false),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	for range stream {
	}
}

func TestClientStreamSendsOpenAICompatibleTokenLimits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if got := int(body["max_completion_tokens"].(float64)); got != 20000 {
			t.Fatalf("request max_completion_tokens = %d, want 20000", got)
		}
		if got := int(body["max_tokens"].(float64)); got != 20000 {
			t.Fatalf("request max_tokens = %d, want 20000", got)
		}

		w.Header().Set("content-type", "text/event-stream")
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
		Model:           "gpt-5",
		MaxOutputTokens: 20000,
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					message.TextPart("hello"),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	for range stream {
	}
}

// TestClientStreamReturnsStructuredError verifies HTTP errors are returned as structured APIError instead of fmt.Errorf.
func TestClientStreamReturnsStructuredError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		t.Fatal("Stream() error = nil, want non-nil")
	}

	// Verify the error is a structured *APIError.
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != 429 {
		t.Fatalf("status = %d, want 429", apiErr.Status)
	}
	if apiErr.Type != ErrorTypeRateLimit {
		t.Fatalf("type = %q, want rate_limit_error", apiErr.Type)
	}
	if apiErr.Message != "Rate limit exceeded" {
		t.Fatalf("message = %q, want Rate limit exceeded", apiErr.Message)
	}
}

// TestClientStreamReturnsStructuredErrorForEmptyBody verifies fallback when error body is empty.
func TestClientStreamReturnsStructuredErrorForEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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
		t.Fatal("Stream() error = nil, want non-nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != 500 {
		t.Fatalf("status = %d, want 500", apiErr.Status)
	}
}

func TestClientStreamSendsGLMMaxTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != glmChatCompletionsPath {
			t.Fatalf("request path = %q, want %q", r.URL.Path, glmChatCompletionsPath)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if got := int(body["max_tokens"].(float64)); got != 20000 {
			t.Fatalf("request max_tokens = %d, want 20000", got)
		}
		if _, ok := body["max_completion_tokens"]; ok {
			t.Fatalf("unexpected max_completion_tokens field in GLM request: %#v", body["max_completion_tokens"])
		}

		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewClient(Config{
		Provider:   "glm",
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model:           "glm-4.6",
		MaxOutputTokens: 20000,
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					message.TextPart("hello"),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	for range stream {
	}
}
