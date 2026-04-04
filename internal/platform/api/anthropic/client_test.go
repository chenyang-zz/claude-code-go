package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// TestClientStreamReadsTextDelta verifies Anthropic SSE text delta events are mapped into shared model events.
func TestClientStreamReadsTextDelta(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("x-api-key = %q, want test-key", r.Header.Get("x-api-key"))
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		if body["model"] != "claude-sonnet-4-5" {
			t.Fatalf("request model = %#v, want claude-sonnet-4-5", body["model"])
		}

		tools, ok := body["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Fatalf("request tools = %#v, want one tool", body["tools"])
		}

		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n"))
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-5",
		Tools: []model.ToolDefinition{
			{
				Name:        "Read",
				Description: "Read a file",
				InputSchema: map[string]any{
					"type": "object",
				},
			},
		},
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					{
						Type: "text",
						Text: "hello",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	evt := <-stream
	if evt.Type != model.EventTypeTextDelta || evt.Text != "hello" {
		t.Fatalf("Stream() first event = %#v, want text delta hello", evt)
	}
}

// TestClientStreamReadsToolUse verifies Anthropic tool_use SSE payloads are mapped into shared tool-use events.
func TestClientStreamReadsToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_start\n"))
		_, _ = w.Write([]byte("data: {\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"Read\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"file_path\\\":\\\"main.go\\\"}\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_stop\n"))
		_, _ = w.Write([]byte("data: {\"index\":0}\n\n"))
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-5",
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	evt := <-stream
	if evt.Type != model.EventTypeToolUse {
		t.Fatalf("Stream() first event type = %q, want tool_use", evt.Type)
	}
	if evt.ToolUse == nil {
		t.Fatal("Stream() tool_use payload = nil")
	}
	if evt.ToolUse.ID != "toolu_1" || evt.ToolUse.Name != "Read" {
		t.Fatalf("Stream() tool_use = %#v", evt.ToolUse)
	}
	if got := evt.ToolUse.Input["file_path"]; got != "main.go" {
		t.Fatalf("Stream() tool_use input file_path = %#v, want main.go", got)
	}
}

// TestClientStreamReadsErrorEvent verifies Anthropic error SSE payloads become shared error events.
func TestClientStreamReadsErrorEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("event: error\n"))
		_, _ = w.Write([]byte("data: {\"error\":{\"message\":\"bad request\"}}\n\n"))
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-5",
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	evt := <-stream
	if evt.Type != model.EventTypeError || evt.Error != "bad request" {
		t.Fatalf("Stream() first event = %#v, want error bad request", evt)
	}
}
