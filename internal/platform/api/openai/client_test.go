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

// TestClientStreamSendsMaxTokensForCompatibleProvider verifies that a non-official
// OpenAI-compatible provider receives only the legacy max_tokens field.
func TestClientStreamSendsMaxTokensForCompatibleProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if _, ok := body["max_completion_tokens"]; ok {
			t.Fatalf("request should not contain max_completion_tokens for compatible provider")
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

// TestClientStreamSendsMaxCompletionTokensForOfficialOpenAI verifies that the
// official OpenAI API receives only max_completion_tokens.
func TestClientStreamSendsMaxCompletionTokensForOfficialOpenAI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if _, ok := body["max_tokens"]; ok {
			t.Fatalf("request should not contain max_tokens for official OpenAI API")
		}
		if got := int(body["max_completion_tokens"].(float64)); got != 20000 {
			t.Fatalf("request max_completion_tokens = %d, want 20000", got)
		}

		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewClient(Config{
		Provider:   "openai-compatible",
		APIKey:     "test-key",
		BaseURL:    defaultBaseURL, // official OpenAI endpoint
		HTTPClient: &http.Client{Transport: &rewriteTransport{host: server.Listener.Addr().String()}},
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

// TestMapMessagesPureTextStillString confirms mapMessages preserves the legacy
// string-form Content for plain-text user messages so existing callers and
// JSON consumers see no behavior change.
func TestMapMessagesPureTextStillString(t *testing.T) {
	out := mapMessages("system", []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart("hello world"),
			},
		},
	})

	if len(out) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(out))
	}
	user := out[1]
	if user.Role != string(message.RoleUser) {
		t.Fatalf("expected user role, got %q", user.Role)
	}
	if s, ok := user.Content.(string); !ok || s != "hello world" {
		t.Fatalf("expected plain string Content, got %T %v", user.Content, user.Content)
	}
}

// TestMapMessagesImageUrlSingle verifies a single ImagePart is serialized into
// the OpenAI Chat Completions image_url object form with a base64 data URL.
func TestMapMessagesImageUrlSingle(t *testing.T) {
	out := mapMessages("", []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.ImagePart("image/png", "iVBORw0K"),
			},
		},
	})

	if len(out) != 1 {
		t.Fatalf("expected 1 user message, got %d", len(out))
	}
	parts, ok := out[0].Content.([]chatContentPart)
	if !ok {
		t.Fatalf("expected []chatContentPart, got %T", out[0].Content)
	}
	if len(parts) != 1 {
		t.Fatalf("expected 1 content part, got %d", len(parts))
	}
	if parts[0].Type != "image_url" {
		t.Fatalf("expected image_url part, got %q", parts[0].Type)
	}
	if parts[0].ImageURL == nil {
		t.Fatalf("expected ImageURL body to be non-nil")
	}
	if got, want := parts[0].ImageURL.URL, "data:image/png;base64,iVBORw0K"; got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}

	body, err := json.Marshal(out[0])
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}
	if !contains(body, []byte(`"type":"image_url"`)) || !contains(body, []byte(`"image_url":{"url":"data:image/png;base64,iVBORw0K"}`)) {
		t.Fatalf("unexpected JSON shape: %s", string(body))
	}
}

// TestMapMessagesMixedTextAndImage verifies text and image parts coexist as a
// structured Content array following the original part order.
func TestMapMessagesMixedTextAndImage(t *testing.T) {
	out := mapMessages("", []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart("describe this:"),
				message.ImagePart("image/jpeg", "/9j/4AAQ"),
			},
		},
	})

	if len(out) != 1 {
		t.Fatalf("expected 1 user message, got %d", len(out))
	}
	parts, ok := out[0].Content.([]chatContentPart)
	if !ok {
		t.Fatalf("expected []chatContentPart, got %T", out[0].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text != "describe this:" {
		t.Fatalf("unexpected first part: %+v", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != "data:image/jpeg;base64,/9j/4AAQ" {
		t.Fatalf("unexpected second part: %+v", parts[1])
	}
}

// TestMapMessagesMultipleImages verifies multiple image parts in one user
// message are emitted as multiple image_url parts in original order.
func TestMapMessagesMultipleImages(t *testing.T) {
	out := mapMessages("", []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart("page summary:"),
				message.ImagePart("image/jpeg", "page1Data"),
				message.ImagePart("image/jpeg", "page2Data"),
				message.ImagePart("image/jpeg", "page3Data"),
			},
		},
	})

	parts, ok := out[0].Content.([]chatContentPart)
	if !ok {
		t.Fatalf("expected []chatContentPart, got %T", out[0].Content)
	}
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(parts))
	}
	expected := []string{
		"data:image/jpeg;base64,page1Data",
		"data:image/jpeg;base64,page2Data",
		"data:image/jpeg;base64,page3Data",
	}
	for i, want := range expected {
		got := parts[i+1]
		if got.Type != "image_url" || got.ImageURL == nil || got.ImageURL.URL != want {
			t.Fatalf("part %d = %+v, want image_url URL=%q", i+1, got, want)
		}
	}
}

// TestMapMessagesDocumentDegradeChatCompletions verifies DocumentPart entries
// are silently skipped (with a logger warning) while sibling text and image
// parts continue to render through the OpenAI Chat Completions content array.
func TestMapMessagesDocumentDegradeChatCompletions(t *testing.T) {
	out := mapMessages("", []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart("read this PDF:"),
				message.DocumentPart("application/pdf", "JVBERi0x"),
				message.ImagePart("image/jpeg", "fallbackPageData"),
			},
		},
	})

	parts, ok := out[0].Content.([]chatContentPart)
	if !ok {
		t.Fatalf("expected []chatContentPart, got %T", out[0].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts (text + image, document skipped), got %d", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text != "read this PDF:" {
		t.Fatalf("unexpected first part: %+v", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != "data:image/jpeg;base64,fallbackPageData" {
		t.Fatalf("unexpected second part: %+v", parts[1])
	}
}

// TestMapMessagesToolResultRouting confirms tool_result parts go to a separate
// tool-role message while sibling images are attached to the user message.
func TestMapMessagesToolResultRouting(t *testing.T) {
	out := mapMessages("", []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.ToolResultPart("call_1", "tool output", false),
				message.ImagePart("image/png", "afterToolImage"),
			},
		},
	})

	if len(out) != 2 {
		t.Fatalf("expected 2 messages (user + tool), got %d", len(out))
	}
	parts, ok := out[0].Content.([]chatContentPart)
	if !ok {
		t.Fatalf("expected user Content to be []chatContentPart, got %T", out[0].Content)
	}
	if len(parts) != 1 || parts[0].Type != "image_url" {
		t.Fatalf("unexpected user parts: %+v", parts)
	}
	if out[1].Role != string(message.RoleTool) {
		t.Fatalf("expected tool role, got %q", out[1].Role)
	}
	if s, ok := out[1].Content.(string); !ok || s != "tool output" {
		t.Fatalf("expected tool string Content, got %T %v", out[1].Content, out[1].Content)
	}
	if out[1].ToolCallID != "call_1" {
		t.Fatalf("ToolCallID = %q, want call_1", out[1].ToolCallID)
	}
}

// contains is a tiny byte-slice substring helper used by JSON shape assertions.
func contains(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// rewriteTransport rewrites HTTPS requests to HTTP and directs them to a
// local test server host. This lets tests set BaseURL to the official OpenAI
// endpoint (triggering isOfficialOpenAI()) while routing traffic locally.
type rewriteTransport struct{ host string }

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.host
	return http.DefaultTransport.RoundTrip(req)
}
