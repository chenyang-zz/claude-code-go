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

// TestClientStreamReadsThinking verifies Anthropic thinking SSE payloads are mapped into shared thinking events.
func TestClientStreamReadsThinking(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_start\n"))
		_, _ = w.Write([]byte("data: {\"index\":0,\"content_block\":{\"type\":\"thinking\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"Let me analyze\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"index\":0,\"delta\":{\"type\":\"signature_delta\",\"signature\":\"sig123\"}}\n\n"))
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
	if evt.Type != model.EventTypeThinking {
		t.Fatalf("Stream() first event type = %q, want thinking", evt.Type)
	}
	if evt.Thinking != "Let me analyze" {
		t.Fatalf("Stream() thinking = %q, want Let me analyze", evt.Thinking)
	}
	if evt.Signature != "sig123" {
		t.Fatalf("Stream() signature = %q, want sig123", evt.Signature)
	}
}

// TestClientStreamReadsRedactedThinking verifies Anthropic redacted_thinking SSE payloads are mapped into shared thinking events.
func TestClientStreamReadsRedactedThinking(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_start\n"))
		_, _ = w.Write([]byte("data: {\"index\":0,\"content_block\":{\"type\":\"redacted_thinking\",\"data\":\"redacted_data_abc\"}}\n\n"))
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
	if evt.Type != model.EventTypeThinking {
		t.Fatalf("Stream() first event type = %q, want thinking", evt.Type)
	}
	if evt.Thinking != "redacted_data_abc" {
		t.Fatalf("Stream() thinking = %q, want redacted_data_abc", evt.Thinking)
	}
}

// TestClientStreamMapsThinkingMessages verifies assistant thinking and redacted_thinking history are preserved in the Anthropic request body.
func TestClientStreamMapsThinkingMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		messages, ok := body["messages"].([]any)
		if !ok || len(messages) != 2 {
			t.Fatalf("request messages = %#v, want 2 messages", body["messages"])
		}

		assistant, ok := messages[1].(map[string]any)
		if !ok {
			t.Fatalf("assistant message = %#v, want object", messages[1])
		}
		assistantContent, ok := assistant["content"].([]any)
		if !ok || len(assistantContent) != 2 {
			t.Fatalf("assistant content = %#v, want 2 blocks", assistant["content"])
		}

		thinking, ok := assistantContent[0].(map[string]any)
		if !ok {
			t.Fatalf("thinking block = %#v, want object", assistantContent[0])
		}
		if thinking["type"] != "thinking" || thinking["thinking"] != "analysis" || thinking["signature"] != "sig" {
			t.Fatalf("thinking block = %#v", thinking)
		}

		redacted, ok := assistantContent[1].(map[string]any)
		if !ok {
			t.Fatalf("redacted block = %#v, want object", assistantContent[1])
		}
		if redacted["type"] != "redacted_thinking" || redacted["data"] != "opaque" {
			t.Fatalf("redacted block = %#v", redacted)
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
					message.TextPart("hello"),
				},
			},
			{
				Role: message.RoleAssistant,
				Content: []message.ContentPart{
					message.ThinkingPart("analysis", "sig"),
					message.RedactedThinkingPart("opaque"),
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

// TestClientStreamAddsCacheControlWhenEnabled verifies that a cache_control
// marker is placed on the last content block of the last message when
// EnablePromptCaching is true.
func TestClientStreamAddsCacheControlWhenEnabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		messages, ok := body["messages"].([]any)
		if !ok || len(messages) != 2 {
			t.Fatalf("request messages = %#v, want 2 messages", body["messages"])
		}

		lastMsg, ok := messages[1].(map[string]any)
		if !ok {
			t.Fatalf("last message = %#v, want object", messages[1])
		}
		content, ok := lastMsg["content"].([]any)
		if !ok || len(content) == 0 {
			t.Fatalf("last message content = %#v, want non-empty array", lastMsg["content"])
		}
		lastBlock, ok := content[len(content)-1].(map[string]any)
		if !ok {
			t.Fatalf("last content block = %#v, want object", content[len(content)-1])
		}
		cacheControl, ok := lastBlock["cache_control"].(map[string]any)
		if !ok {
			t.Fatalf("cache_control = %#v, want object", lastBlock["cache_control"])
		}
		if cacheControl["type"] != "ephemeral" {
			t.Fatalf("cache_control.type = %q, want ephemeral", cacheControl["type"])
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
		Model:               "claude-sonnet-4-5",
		EnablePromptCaching: true,
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					{Type: "text", Text: "first"},
				},
			},
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					{Type: "text", Text: "second"},
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

// TestClientStreamOmitsCacheControlWhenDisabled verifies that no cache_control
// marker appears when EnablePromptCaching is false.
func TestClientStreamOmitsCacheControlWhenDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		messages, ok := body["messages"].([]any)
		if !ok || len(messages) != 1 {
			t.Fatalf("request messages = %#v, want 1 message", body["messages"])
		}

		msg, ok := messages[0].(map[string]any)
		if !ok {
			t.Fatalf("message = %#v, want object", messages[0])
		}
		content, ok := msg["content"].([]any)
		if !ok || len(content) == 0 {
			t.Fatalf("message content = %#v, want non-empty array", msg["content"])
		}
		lastBlock, ok := content[len(content)-1].(map[string]any)
		if !ok {
			t.Fatalf("last content block = %#v, want object", content[len(content)-1])
		}
		if _, hasCacheControl := lastBlock["cache_control"]; hasCacheControl {
			t.Fatalf("last block should not have cache_control, got %#v", lastBlock["cache_control"])
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
		Model:               "claude-sonnet-4-5",
		EnablePromptCaching: false,
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					{Type: "text", Text: "hello"},
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

// TestClientStreamSkipsThinkingBlocksForCacheControl verifies that when the
// last message is from the assistant and ends with thinking blocks, the
// cache_control marker is placed on the preceding eligible block instead.
func TestClientStreamSkipsThinkingBlocksForCacheControl(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		messages, ok := body["messages"].([]any)
		if !ok || len(messages) != 1 {
			t.Fatalf("request messages = %#v, want 1 message", body["messages"])
		}

		msg, ok := messages[0].(map[string]any)
		if !ok {
			t.Fatalf("message = %#v, want object", messages[0])
		}
		content, ok := msg["content"].([]any)
		if !ok || len(content) != 2 {
			t.Fatalf("message content length = %d, want 2", len(content))
		}

		textBlock, ok := content[0].(map[string]any)
		if !ok {
			t.Fatalf("first block = %#v, want object", content[0])
		}
		if textBlock["type"] != "text" {
			t.Fatalf("first block type = %q, want text", textBlock["type"])
		}
		cacheControl, ok := textBlock["cache_control"].(map[string]any)
		if !ok {
			t.Fatalf("text block cache_control = %#v, want object", textBlock["cache_control"])
		}
		if cacheControl["type"] != "ephemeral" {
			t.Fatalf("cache_control.type = %q, want ephemeral", cacheControl["type"])
		}

		thinkingBlock, ok := content[1].(map[string]any)
		if !ok {
			t.Fatalf("second block = %#v, want object", content[1])
		}
		if thinkingBlock["type"] != "thinking" {
			t.Fatalf("second block type = %q, want thinking", thinkingBlock["type"])
		}
		if _, hasCacheControl := thinkingBlock["cache_control"]; hasCacheControl {
			t.Fatal("thinking block should not have cache_control")
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
		Model:               "claude-sonnet-4-5",
		EnablePromptCaching: true,
		Messages: []message.Message{
			{
				Role: message.RoleAssistant,
				Content: []message.ContentPart{
					{Type: "text", Text: "hello"},
					{Type: "thinking", Thinking: "thinking..."},
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
