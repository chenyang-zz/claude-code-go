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

func TestClientStreamUsesTopLevelSystemAndCustomMaxTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		if got := body["system"]; got != "summarize the conversation" {
			t.Fatalf("request system = %#v, want summarize the conversation", got)
		}
		if got := int(body["max_tokens"].(float64)); got != 20000 {
			t.Fatalf("request max_tokens = %d, want 20000", got)
		}

		messages, ok := body["messages"].([]any)
		if !ok || len(messages) != 1 {
			t.Fatalf("request messages = %#v, want 1 user message", body["messages"])
		}
		first, ok := messages[0].(map[string]any)
		if !ok {
			t.Fatalf("first message = %#v, want object", messages[0])
		}
		if got := first["role"]; got != "user" {
			t.Fatalf("first message role = %#v, want user", got)
		}

		w.Header().Set("content-type", "text/event-stream")
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model:           "claude-sonnet-4-5",
		System:          "summarize the conversation",
		MaxOutputTokens: 20000,
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					message.TextPart("[compact_boundary]"),
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

// TestClientStreamUsesAuthToken verifies Anthropic account auth uses a Bearer header when no API key is configured.
func TestClientStreamUsesAuthToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("authorization"); got != "Bearer auth-token" {
			t.Fatalf("authorization = %q, want Bearer auth-token", got)
		}
		if got := r.Header.Get("x-api-key"); got != "" {
			t.Fatalf("x-api-key = %q, want empty when using auth token", got)
		}
		w.Header().Set("content-type", "text/event-stream")
	}))
	defer server.Close()

	client := NewClient(Config{
		AuthToken:  "auth-token",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-5",
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	for range stream {
	}
}

// TestClientStreamMapsToolLoopMessages verifies assistant tool_use and user tool_result history are preserved in the Anthropic request body.
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
		assistantContent, ok := assistant["content"].([]any)
		if !ok || len(assistantContent) != 1 {
			t.Fatalf("assistant content = %#v, want 1 block", assistant["content"])
		}
		toolUse, ok := assistantContent[0].(map[string]any)
		if !ok {
			t.Fatalf("assistant tool_use = %#v, want object", assistantContent[0])
		}
		if toolUse["type"] != "tool_use" || toolUse["id"] != "toolu_1" || toolUse["name"] != "Read" {
			t.Fatalf("assistant tool_use block = %#v", toolUse)
		}

		user, ok := messages[2].(map[string]any)
		if !ok {
			t.Fatalf("user message = %#v, want object", messages[2])
		}
		userContent, ok := user["content"].([]any)
		if !ok || len(userContent) != 1 {
			t.Fatalf("user content = %#v, want 1 block", user["content"])
		}
		toolResult, ok := userContent[0].(map[string]any)
		if !ok {
			t.Fatalf("user tool_result = %#v, want object", userContent[0])
		}
		if toolResult["type"] != "tool_result" || toolResult["tool_use_id"] != "toolu_1" || toolResult["content"] != "file contents" {
			t.Fatalf("user tool_result block = %#v", toolResult)
		}
		if toolResult["is_error"] != true {
			t.Fatalf("user tool_result is_error = %#v, want true", toolResult["is_error"])
		}

		w.Header().Set("content-type", "text/event-stream")
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-5",
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
					message.ToolUsePart("toolu_1", "Read", map[string]any{"file_path": "main.go"}),
				},
			},
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					message.ToolResultPart("toolu_1", "file contents", true),
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

// TestClientStreamTaskBudget verifies that task_budget is included in the
// request body output_config and the task-budgets beta header is sent when
// the client is first-party and TaskBudget is provided.
func TestClientStreamTaskBudget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify beta header.
		if got := r.Header.Get("anthropic-beta"); got != "task-budgets-2026-03-13" {
			t.Fatalf("anthropic-beta = %q, want task-budgets-2026-03-13", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		// Verify output_config.task_budget.
		outputConfig, ok := body["output_config"].(map[string]any)
		if !ok {
			t.Fatalf("output_config = %#v, want object", body["output_config"])
		}
		taskBudget, ok := outputConfig["task_budget"].(map[string]any)
		if !ok {
			t.Fatalf("task_budget = %#v, want object", outputConfig["task_budget"])
		}
		if taskBudget["type"] != "tokens" {
			t.Fatalf("task_budget.type = %q, want tokens", taskBudget["type"])
		}
		if taskBudget["total"] != float64(500000) {
			t.Fatalf("task_budget.total = %v, want 500000", taskBudget["total"])
		}
		remaining, hasRemaining := taskBudget["remaining"]
		if !hasRemaining {
			t.Fatal("task_budget.remaining missing, expected to be set")
		}
		if remaining != float64(200000) {
			t.Fatalf("task_budget.remaining = %v, want 200000", remaining)
		}

		w.Header().Set("content-type", "text/event-stream")
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:       "test-key",
		BaseURL:      server.URL,
		HTTPClient:   server.Client(),
		IsFirstParty: true,
	})

	remaining := 200000
	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-5",
		TaskBudget: &model.TaskBudgetParam{
			Type:      "tokens",
			Total:     500_000,
			Remaining: &remaining,
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	for range stream {
	}
}

// TestClientStreamTaskBudgetNotFirstParty verifies that task_budget is NOT
// included when the client is not first-party (e.g. Vertex/Bedrock).
func TestClientStreamTaskBudgetNotFirstParty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should NOT have beta header.
		if got := r.Header.Get("anthropic-beta"); got != "" {
			t.Fatalf("anthropic-beta = %q, want empty for non-first-party", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		// Should NOT have output_config.
		if _, ok := body["output_config"]; ok {
			t.Fatal("output_config should not be present for non-first-party")
		}

		w.Header().Set("content-type", "text/event-stream")
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:       "test-key",
		BaseURL:      server.URL,
		HTTPClient:   server.Client(),
		IsFirstParty: false,
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-5",
		TaskBudget: &model.TaskBudgetParam{
			Type:  "tokens",
			Total: 500_000,
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	for range stream {
	}
}

// TestClientStreamTaskBudgetNoRemaining verifies that remaining is omitted
// from the wire format when not set.
func TestClientStreamTaskBudgetNoRemaining(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		outputConfig, ok := body["output_config"].(map[string]any)
		if !ok {
			t.Fatalf("output_config = %#v, want object", body["output_config"])
		}
		taskBudget, ok := outputConfig["task_budget"].(map[string]any)
		if !ok {
			t.Fatalf("task_budget = %#v, want object", outputConfig["task_budget"])
		}
		if _, hasRemaining := taskBudget["remaining"]; hasRemaining {
			t.Fatal("task_budget.remaining should be omitted when nil")
		}

		w.Header().Set("content-type", "text/event-stream")
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:       "test-key",
		BaseURL:      server.URL,
		HTTPClient:   server.Client(),
		IsFirstParty: true,
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-5",
		TaskBudget: &model.TaskBudgetParam{
			Type:  "tokens",
			Total: 500_000,
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	for range stream {
	}
}
