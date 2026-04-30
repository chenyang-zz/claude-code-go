package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

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

// TestParseRetryAfterSeconds verifies parseRetryAfter handles integer seconds.
func TestParseRetryAfterSeconds(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		wantDur  time.Duration
		wantOk   bool
	}{
		{"valid seconds", "120", 120 * time.Second, true},
		{"zero seconds", "0", 0, true},
		{"missing header", "", 0, false},
		{"invalid string", "invalid", 0, false},
		{"negative seconds", "-1", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.Header{}
			if tt.value != "" {
				h.Set("retry-after", tt.value)
			}
			dur, ok := parseRetryAfter(h)
			if ok != tt.wantOk {
				t.Fatalf("parseRetryAfter() ok = %v, want %v", ok, tt.wantOk)
			}
			if dur != tt.wantDur {
				t.Fatalf("parseRetryAfter() dur = %v, want %v", dur, tt.wantDur)
			}
		})
	}
}

// TestParseRetryAfterRFC1123 verifies parseRetryAfter handles RFC1123 dates.
func TestParseRetryAfterRFC1123(t *testing.T) {
	future := time.Now().UTC().Add(2 * time.Minute).Format(time.RFC1123)
	past := time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC1123)

	h := http.Header{}
	h.Set("retry-after", future)
	dur, ok := parseRetryAfter(h)
	if !ok {
		t.Fatal("parseRetryAfter() ok = false, want true for RFC1123 date")
	}
	// Allow a small tolerance for clock skew during the test.
	if dur < 110*time.Second || dur > 130*time.Second {
		t.Fatalf("parseRetryAfter() dur = %v, want ~2m", dur)
	}

	h2 := http.Header{}
	h2.Set("retry-after", past)
	dur2, ok2 := parseRetryAfter(h2)
	if !ok2 {
		t.Fatal("parseRetryAfter() ok = false, want true for past RFC1123 date")
	}
	if dur2 != 0 {
		t.Fatalf("parseRetryAfter() dur = %v, want 0 for past date", dur2)
	}
}

// TestParseRateLimitHeaders verifies rate limit header extraction.
func TestParseRateLimitHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("x-ratelimit-remaining", "42")
	h.Set("x-ratelimit-reset", "1760000000")
	h.Set("x-ratelimit-limit", "100")

	rl := parseRateLimitHeaders(h)
	if rl.Remaining != 42 {
		t.Fatalf("remaining = %d, want 42", rl.Remaining)
	}
	if rl.Reset != 1760000000 {
		t.Fatalf("reset = %d, want 1760000000", rl.Reset)
	}
	if rl.RequestLimit != 100 {
		t.Fatalf("requestLimit = %d, want 100", rl.RequestLimit)
	}
}

// TestParseRateLimitHeadersMissing verifies missing headers return zero values.
func TestParseRateLimitHeadersMissing(t *testing.T) {
	rl := parseRateLimitHeaders(http.Header{})
	if rl.Remaining != 0 || rl.Reset != 0 || rl.RequestLimit != 0 {
		t.Fatalf("expected zero values, got %+v", rl)
	}
}

// TestComputeBackoffRetryAfter verifies computeBackoff prefers retry-after.
func TestComputeBackoffRetryAfter(t *testing.T) {
	h := http.Header{}
	h.Set("retry-after", "60")
	h.Set("x-ratelimit-reset", "1760000000")

	dur, ok := computeBackoff(h)
	if !ok {
		t.Fatal("computeBackoff() ok = false, want true")
	}
	if dur != 60*time.Second {
		t.Fatalf("computeBackoff() dur = %v, want 60s", dur)
	}
}

// TestComputeBackoffRateLimitReset verifies computeBackoff falls back to
// x-ratelimit-reset when retry-after is absent.
func TestComputeBackoffRateLimitReset(t *testing.T) {
	reset := time.Now().UTC().Add(90 * time.Second)
	h := http.Header{}
	h.Set("x-ratelimit-reset", strconv.FormatInt(reset.Unix(), 10))

	dur, ok := computeBackoff(h)
	if !ok {
		t.Fatal("computeBackoff() ok = false, want true")
	}
	// Allow small tolerance for clock skew.
	if dur < 80*time.Second || dur > 100*time.Second {
		t.Fatalf("computeBackoff() dur = %v, want ~90s", dur)
	}
}

// TestComputeBackoffMissing verifies computeBackoff returns false when no
// relevant headers are present.
func TestComputeBackoffMissing(t *testing.T) {
	dur, ok := computeBackoff(http.Header{})
	if ok {
		t.Fatal("computeBackoff() ok = true, want false")
	}
	if dur != 0 {
		t.Fatalf("computeBackoff() dur = %v, want 0", dur)
	}
}

// TestComputeRetryDelayRateLimitRetryAfter verifies that rate-limit errors
// with a retry-after header use that value.
func TestComputeRetryDelayRateLimitRetryAfter(t *testing.T) {
	headers := http.Header{}
	headers.Set("retry-after", "60")
	err := &APIError{Status: 429, Type: ErrorTypeRateLimit, Headers: headers}

	dur, retryErr := computeRetryDelay(err, 0)
	if retryErr != nil {
		t.Fatalf("computeRetryDelay() error = %v, want nil", retryErr)
	}
	if dur != 60*time.Second {
		t.Fatalf("computeRetryDelay() dur = %v, want 60s", dur)
	}
}

// TestComputeRetryDelayOverloadedRetryAfter verifies that overloaded errors
// with a retry-after header use that value.
func TestComputeRetryDelayOverloadedRetryAfter(t *testing.T) {
	headers := http.Header{}
	headers.Set("retry-after", "30")
	err := &APIError{Status: 529, Type: ErrorTypeOverloaded, Headers: headers}

	dur, retryErr := computeRetryDelay(err, 0)
	if retryErr != nil {
		t.Fatalf("computeRetryDelay() error = %v, want nil", retryErr)
	}
	if dur != 30*time.Second {
		t.Fatalf("computeRetryDelay() dur = %v, want 30s", dur)
	}
}

// TestComputeRetryDelayRateLimitResetFallback verifies that rate-limit errors
// fall back to rate-limit-reset when retry-after is absent.
func TestComputeRetryDelayRateLimitResetFallback(t *testing.T) {
	reset := time.Now().UTC().Add(2 * time.Minute)
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-reset", strconv.FormatInt(reset.Unix(), 10))
	err := &APIError{Status: 429, Type: ErrorTypeRateLimit, Headers: headers}

	dur, retryErr := computeRetryDelay(err, 0)
	if retryErr != nil {
		t.Fatalf("computeRetryDelay() error = %v, want nil", retryErr)
	}
	// Allow tolerance for clock skew.
	if dur < 110*time.Second || dur > 130*time.Second {
		t.Fatalf("computeRetryDelay() dur = %v, want ~2m", dur)
	}
}

// TestComputeRetryDelayRateLimitExponentialFallback verifies that rate-limit
// errors without any headers fall back to exponential backoff.
func TestComputeRetryDelayRateLimitExponentialFallback(t *testing.T) {
	err := &APIError{Status: 429, Type: ErrorTypeRateLimit}

	dur, retryErr := computeRetryDelay(err, 0)
	if retryErr != nil {
		t.Fatalf("computeRetryDelay() error = %v, want nil", retryErr)
	}
	// attempt=0: baseDelay=500ms * 2^0 = 500ms, plus up to 25% jitter = max 625ms.
	if dur < 500*time.Millisecond || dur > 625*time.Millisecond {
		t.Fatalf("computeRetryDelay() dur = %v, want ~500-625ms", dur)
	}
}

// TestComputeRetryDelayExponentialBackoffAttempts verifies exponential backoff
// scales with attempt number.
func TestComputeRetryDelayExponentialBackoffAttempts(t *testing.T) {
	err := &APIError{Status: 502, Message: "bad gateway"}

	// attempt=0: ~500ms
	dur0, retryErr := computeRetryDelay(err, 0)
	if retryErr != nil {
		t.Fatalf("computeRetryDelay() error = %v, want nil", retryErr)
	}
	if dur0 < 500*time.Millisecond || dur0 > 625*time.Millisecond {
		t.Fatalf("attempt 0 dur = %v, want ~500-625ms", dur0)
	}

	// attempt=1: ~1000ms
	dur1, retryErr := computeRetryDelay(err, 1)
	if retryErr != nil {
		t.Fatalf("computeRetryDelay() error = %v, want nil", retryErr)
	}
	if dur1 < 1000*time.Millisecond || dur1 > 1250*time.Millisecond {
		t.Fatalf("attempt 1 dur = %v, want ~1-1.25s", dur1)
	}

	// attempt=2: ~2000ms
	dur2, retryErr := computeRetryDelay(err, 2)
	if retryErr != nil {
		t.Fatalf("computeRetryDelay() error = %v, want nil", retryErr)
	}
	if dur2 < 2000*time.Millisecond || dur2 > 2500*time.Millisecond {
		t.Fatalf("attempt 2 dur = %v, want ~2-2.5s", dur2)
	}
}

// TestComputeRetryDelayExponentialBackoffMaxCap verifies backoff is capped at
// maxDelay (30s).
func TestComputeRetryDelayExponentialBackoffMaxCap(t *testing.T) {
	err := &APIError{Status: 503, Message: "service unavailable"}

	// attempt=10 would be 500ms * 2^10 = ~512s, but capped at 30s.
	dur, retryErr := computeRetryDelay(err, 10)
	if retryErr != nil {
		t.Fatalf("computeRetryDelay() error = %v, want nil", retryErr)
	}
	if dur < 30*time.Second || dur > 37500*time.Millisecond {
		t.Fatalf("computeRetryDelay() dur = %v, want ~30-37.5s", dur)
	}
}

// TestComputeRetryDelayNotRetryable verifies non-retryable errors return an error.
func TestComputeRetryDelayNotRetryable(t *testing.T) {
	err := &APIError{Status: 400, Type: ErrorTypeInvalidRequest, Message: "bad request"}

	dur, retryErr := computeRetryDelay(err, 0)
	if retryErr == nil {
		t.Fatal("computeRetryDelay() error = nil, want non-nil")
	}
	if dur != 0 {
		t.Fatalf("computeRetryDelay() dur = %v, want 0", dur)
	}
}

// TestComputeRetryDelayNilError verifies nil error returns an error.
func TestComputeRetryDelayNilError(t *testing.T) {
	dur, retryErr := computeRetryDelay(nil, 0)
	if retryErr == nil {
		t.Fatal("computeRetryDelay() error = nil, want non-nil")
	}
	if dur != 0 {
		t.Fatalf("computeRetryDelay() dur = %v, want 0", dur)
	}
}

// TestComputeRetryDelayOverloadedNoHeaders verifies overloaded errors without
// headers fall back to exponential backoff.
func TestComputeRetryDelayOverloadedNoHeaders(t *testing.T) {
	err := &APIError{Status: 529, Type: ErrorTypeOverloaded}

	dur, retryErr := computeRetryDelay(err, 0)
	if retryErr != nil {
		t.Fatalf("computeRetryDelay() error = %v, want nil", retryErr)
	}
	if dur < 500*time.Millisecond || dur > 625*time.Millisecond {
		t.Fatalf("computeRetryDelay() dur = %v, want ~500-625ms", dur)
	}
}

// TestExponentialBackoffConfig verifies custom retryConfig is respected.
func TestExponentialBackoffConfig(t *testing.T) {
	cfg := retryConfig{baseDelay: 100 * time.Millisecond, maxDelay: 1 * time.Second, jitterFrac: 0.1}

	dur := exponentialBackoff(cfg, 0)
	if dur < 100*time.Millisecond || dur > 110*time.Millisecond {
		t.Fatalf("attempt 0 dur = %v, want ~100-110ms", dur)
	}

	dur = exponentialBackoff(cfg, 3)
	// 100ms * 2^3 = 800ms, + 10% jitter = max 880ms.
	if dur < 800*time.Millisecond || dur > 880*time.Millisecond {
		t.Fatalf("attempt 3 dur = %v, want ~800-880ms", dur)
	}

	dur = exponentialBackoff(cfg, 10)
	// Should be capped at maxDelay=1s + 10% jitter.
	if dur < 1000*time.Millisecond || dur > 1100*time.Millisecond {
		t.Fatalf("attempt 10 dur = %v, want ~1-1.1s", dur)
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


// TestClientStreamExtraToolSchemas verifies that ExtraToolSchemas are serialized
// as extra_tools in the request body.
func TestClientStreamExtraToolSchemas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		extraTools, ok := body["extra_tools"].([]any)
		if !ok || len(extraTools) != 1 {
			t.Fatalf("extra_tools = %#v, want 1 tool", body["extra_tools"])
		}

		tool, ok := extraTools[0].(map[string]any)
		if !ok {
			t.Fatalf("extra_tool = %#v, want object", extraTools[0])
		}
		if tool["type"] != "web_search_20250305" {
			t.Fatalf("extra_tool type = %q, want web_search_20250305", tool["type"])
		}
		if tool["name"] != "web_search" {
			t.Fatalf("extra_tool name = %q, want web_search", tool["name"])
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
		ExtraToolSchemas: []map[string]any{
			{
				"type":     "web_search_20250305",
				"name":     "web_search",
				"max_uses": float64(8),
			},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	for range stream {
	}
}

// TestClientStreamServerToolUse verifies Anthropic server_tool_use SSE payloads
// are mapped into shared server_tool_use events.
func TestClientStreamServerToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_start\n"))
		_, _ = w.Write([]byte("data: {\"index\":0,\"content_block\":{\"type\":\"server_tool_use\",\"id\":\"stu_1\",\"name\":\"web_search\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"query\\\":\\\"test search\\\"}\"}}\n\n"))
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
	if evt.Type != model.EventTypeServerToolUse {
		t.Fatalf("Stream() first event type = %q, want server_tool_use", evt.Type)
	}
	if evt.ServerToolUse == nil {
		t.Fatal("Stream() server_tool_use payload = nil")
	}
	if evt.ServerToolUse.ID != "stu_1" || evt.ServerToolUse.Name != "web_search" {
		t.Fatalf("Stream() server_tool_use = %#v", evt.ServerToolUse)
	}
	if got := evt.ServerToolUse.Input["query"]; got != "test search" {
		t.Fatalf("Stream() server_tool_use input query = %#v, want test search", got)
	}
}

// TestClientStreamWebSearchResult verifies Anthropic web_search_tool_result SSE
// payloads are mapped into shared web_search_result events.
func TestClientStreamWebSearchResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_start\n"))
		_, _ = w.Write([]byte("data: {\"index\":1,\"content_block\":{\"type\":\"web_search_tool_result\",\"tool_use_id\":\"stu_1\",\"content\":[{\"title\":\"Example\",\"url\":\"https://example.com\"}]}}\n\n"))
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
	if evt.Type != model.EventTypeWebSearchResult {
		t.Fatalf("Stream() first event type = %q, want web_search_tool_result", evt.Type)
	}
	if evt.WebSearchResult == nil {
		t.Fatal("Stream() web_search_result payload = nil")
	}
	if evt.WebSearchResult.ToolUseID != "stu_1" {
		t.Fatalf("Stream() web_search_result tool_use_id = %q, want stu_1", evt.WebSearchResult.ToolUseID)
	}
	if len(evt.WebSearchResult.Content) != 1 {
		t.Fatalf("Stream() web_search_result content length = %d, want 1", len(evt.WebSearchResult.Content))
	}
	if evt.WebSearchResult.Content[0].Title != "Example" {
		t.Fatalf("Stream() web_search_result hit title = %q, want Example", evt.WebSearchResult.Content[0].Title)
	}
	if evt.WebSearchResult.Content[0].URL != "https://example.com" {
		t.Fatalf("Stream() web_search_result hit url = %q", evt.WebSearchResult.Content[0].URL)
	}
}

// TestClientStreamWebSearchResultError verifies error web_search_tool_result
// with error_code is properly handled.
func TestClientStreamWebSearchResultError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_start\n"))
		_, _ = w.Write([]byte("data: {\"index\":1,\"content_block\":{\"type\":\"web_search_tool_result\",\"tool_use_id\":\"stu_1\",\"error_code\":\"search_error\"}}\n\n"))
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
	if evt.Type != model.EventTypeWebSearchResult {
		t.Fatalf("Stream() first event type = %q, want web_search_tool_result", evt.Type)
	}
	if evt.WebSearchResult == nil {
		t.Fatal("Stream() web_search_result payload = nil")
	}
	if evt.WebSearchResult.ErrorCode != "search_error" {
		t.Fatalf("Stream() web_search_result error_code = %q, want search_error", evt.WebSearchResult.ErrorCode)
	}
	if len(evt.WebSearchResult.Content) != 0 {
		t.Fatalf("Stream() web_search_result content length = %d, want 0", len(evt.WebSearchResult.Content))
	}
}
