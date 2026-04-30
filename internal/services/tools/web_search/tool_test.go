package web_search

import (
	"context"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// stubModelClient implements model.Client for testing, returning a pre-built event sequence.
type stubModelClient struct {
	events []model.Event
	err    error
}

func (s *stubModelClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	if s.err != nil {
		return nil, s.err
	}
	ch := make(chan model.Event)
	go func() {
		for _, evt := range s.events {
			ch <- evt
		}
		// Always emit a done event at the end.
		ch <- model.Event{Type: model.EventTypeDone}
		close(ch)
	}()
	return ch, nil
}

func TestTool_Name(t *testing.T) {
	tool := NewTool(&stubModelClient{}, "claude-sonnet-4-5")
	if tool.Name() != Name {
		t.Fatalf("Name() = %q, want %q", tool.Name(), Name)
	}
}

func TestTool_Description(t *testing.T) {
	tool := NewTool(&stubModelClient{}, "claude-sonnet-4-5")
	if tool.Description() == "" {
		t.Fatal("Description() returned empty string")
	}
}

func TestTool_InputSchema(t *testing.T) {
	tool := NewTool(&stubModelClient{}, "claude-sonnet-4-5")
	schema := tool.InputSchema()
	if _, ok := schema.Properties["query"]; !ok {
		t.Fatal("InputSchema missing required 'query' property")
	}
	queryField := schema.Properties["query"]
	if !queryField.Required {
		t.Fatal("InputSchema 'query' should be required")
	}
	if queryField.Type != coretool.ValueKindString {
		t.Fatalf("InputSchema 'query' type = %q, want string", queryField.Type)
	}
}

func TestTool_IsReadOnly(t *testing.T) {
	tool := NewTool(&stubModelClient{}, "claude-sonnet-4-5")
	if !tool.IsReadOnly() {
		t.Fatal("IsReadOnly() should return true")
	}
}

func TestTool_IsConcurrencySafe(t *testing.T) {
	tool := NewTool(&stubModelClient{}, "claude-sonnet-4-5")
	if !tool.IsConcurrencySafe() {
		t.Fatal("IsConcurrencySafe() should return true")
	}
}

func TestTool_Invoke_NilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{})
	if err == nil {
		t.Fatal("Invoke on nil receiver should return error")
	}
}

func TestTool_Invoke_NilClient(t *testing.T) {
	tool := NewTool(nil, "")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "test search"},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v, want nil (error in result)", err)
	}
	if result.Error == "" {
		t.Fatal("Invoke should return error in result when client is nil")
	}
}

func TestTool_Invoke_EmptyQuery(t *testing.T) {
	tool := NewTool(&stubModelClient{}, "")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": ""},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("Invoke should return error for empty query")
	}
}

func TestTool_Invoke_ShortQuery(t *testing.T) {
	tool := NewTool(&stubModelClient{}, "")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "x"},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("Invoke should return error for query shorter than 2 chars")
	}
}

func TestTool_Invoke_StreamError(t *testing.T) {
	tool := NewTool(&stubModelClient{err: context.DeadlineExceeded}, "")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "test search"},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("Invoke should return error in result when stream fails")
	}
}

func TestTool_Invoke_WebSearchResult(t *testing.T) {
	client := &stubModelClient{
		events: []model.Event{
			{
				Type: model.EventTypeWebSearchResult,
				WebSearchResult: &model.WebSearchResult{
					ToolUseID: "stu_1",
					Content: []model.WebSearchHit{
						{Title: "Example", URL: "https://example.com"},
						{Title: "Test Site", URL: "https://test.com"},
					},
				},
			},
		},
	}
	tool := NewTool(client, "claude-sonnet-4-5")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "test search"},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke result error = %q", result.Error)
	}
	if result.Output == "" {
		t.Fatal("Invoke output is empty")
	}
	// Verify output contains expected content.
	output, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatalf("Meta data is not Output type: %T", result.Meta["data"])
	}
	if output.Query != "test search" {
		t.Fatalf("Output query = %q, want test search", output.Query)
	}
	if output.DurationSeconds <= 0 {
		t.Fatal("Output duration should be positive")
	}
}

func TestTool_Invoke_WebSearchResultError(t *testing.T) {
	client := &stubModelClient{
		events: []model.Event{
			{
				Type: model.EventTypeWebSearchResult,
				WebSearchResult: &model.WebSearchResult{
					ToolUseID: "stu_1",
					ErrorCode: "search_failed",
				},
			},
		},
	}
	tool := NewTool(client, "claude-sonnet-4-5")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "test search"},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke result error = %q (search errors are embedded, not fatal)", result.Error)
	}
}

func TestTool_Invoke_StreamErrorEvent(t *testing.T) {
	client := &stubModelClient{
		events: []model.Event{
			{
				Type:  model.EventTypeError,
				Error: "provider error",
			},
		},
	}
	tool := NewTool(client, "claude-sonnet-4-5")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "test search"},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("Invoke should return error for stream error event")
	}
}

func TestTool_Invoke_TextDeltas(t *testing.T) {
	client := &stubModelClient{
		events: []model.Event{
			{Type: model.EventTypeTextDelta, Text: "Here are the search results: "},
			{Type: model.EventTypeTextDelta, Text: "I found several relevant pages."},
			{
				Type: model.EventTypeWebSearchResult,
				WebSearchResult: &model.WebSearchResult{
					ToolUseID: "stu_1",
					Content: []model.WebSearchHit{
						{Title: "Result", URL: "https://result.com"},
					},
				},
			},
		},
	}
	tool := NewTool(client, "claude-sonnet-4-5")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "test search"},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke result error = %q", result.Error)
	}
	output := result.Output
	if !contains(output, "Here are the search results:") {
		t.Fatal("Output should contain text delta content")
	}
	if !contains(output, "https://result.com") {
		t.Fatal("Output should contain search hit URL")
	}
	if !contains(output, "REMINDER: You MUST include the sources") {
		t.Fatal("Output should contain sources reminder")
	}
}

func TestTool_Invoke_ExtraToolSchemasInRequest(t *testing.T) {
	var capturedReq model.Request
	client := &captureClient{
		capture: func(req model.Request) {
			capturedReq = req
		},
		events: []model.Event{},
	}
	tool := NewTool(client, "claude-sonnet-4-5")
	_, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "test search"},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	if len(capturedReq.ExtraToolSchemas) != 1 {
		t.Fatalf("ExtraToolSchemas length = %d, want 1", len(capturedReq.ExtraToolSchemas))
	}
	schema := capturedReq.ExtraToolSchemas[0]
	if schema["type"] != "web_search_20250305" {
		t.Fatalf("ExtraToolSchemas[0].type = %q, want web_search_20250305", schema["type"])
	}
	if schema["name"] != "web_search" {
		t.Fatalf("ExtraToolSchemas[0].name = %q, want web_search", schema["name"])
	}
	if schema["max_uses"] != float64(maxSearchUses) {
		t.Fatalf("ExtraToolSchemas[0].max_uses = %v, want %d", schema["max_uses"], maxSearchUses)
	}
	if len(capturedReq.Messages) != 1 {
		t.Fatalf("Request messages length = %d, want 1", len(capturedReq.Messages))
	}
}

func TestTool_Invoke_DomainFiltersInRequest(t *testing.T) {
	var capturedReq model.Request
	client := &captureClient{
		capture: func(req model.Request) {
			capturedReq = req
		},
		events: []model.Event{},
	}
	tool := NewTool(client, "claude-sonnet-4-5")
	_, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"query":           "test search",
			"allowed_domains": []any{"example.com", "test.com"},
			"blocked_domains": []any{"spam.com"},
		},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	schema := capturedReq.ExtraToolSchemas[0]
	allowed, ok := schema["allowed_domains"].([]any)
	if !ok || len(allowed) != 2 {
		t.Fatalf("allowed_domains = %#v, want [example.com test.com]", schema["allowed_domains"])
	}
	blocked, ok := schema["blocked_domains"].([]any)
	if !ok || len(blocked) != 1 {
		t.Fatalf("blocked_domains = %#v, want [spam.com]", schema["blocked_domains"])
	}
}

// captureClient is a model.Client that captures the request in its callback
// and returns pre-built events.
type captureClient struct {
	capture func(model.Request)
	events  []model.Event
}

func (c *captureClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	c.capture(req)
	ch := make(chan model.Event)
	go func() {
		for _, evt := range c.events {
			ch <- evt
		}
		ch <- model.Event{Type: model.EventTypeDone}
		close(ch)
	}()
	return ch, nil
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestTool_Invoke_SchemaInvalidType(t *testing.T) {
	tool := NewTool(&stubModelClient{}, "")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": 123},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("Invoke should return validation error for non-string query")
	}
}

func TestTool_Invoke_OutputFormat(t *testing.T) {
	client := &stubModelClient{
		events: []model.Event{
			{
				Type: model.EventTypeWebSearchResult,
				WebSearchResult: &model.WebSearchResult{
					ToolUseID: "stu_1",
					Content: []model.WebSearchHit{
						{Title: "Go", URL: "https://go.dev"},
					},
				},
			},
		},
	}
	tool := NewTool(client, "claude-sonnet-4-5")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "go"},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	// Verify the output contains the expected sections.
	if !contains(result.Output, "go") {
		t.Fatal("Output should contain the search query")
	}
	if !contains(result.Output, "Links:") {
		t.Fatal("Output should contain Links section")
	}
	if !contains(result.Output, "go.dev") {
		t.Fatal("Output should contain search hit URLs")
	}
	// Verify Meta contains Output data.
	output, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatalf("Meta data type = %T, want Output", result.Meta["data"])
	}
	if output.Query != "go" {
		t.Fatalf("Output.Query = %q, want go", output.Query)
	}
}

func TestTool_Invoke_EmptyResults(t *testing.T) {
	client := &stubModelClient{
		events: []model.Event{
			{
				Type: model.EventTypeWebSearchResult,
				WebSearchResult: &model.WebSearchResult{
					ToolUseID: "stu_1",
					Content:   []model.WebSearchHit{},
				},
			},
		},
	}
	tool := NewTool(client, "claude-sonnet-4-5")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "no results"},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke result error = %q", result.Error)
	}
	if !contains(result.Output, "No links found") {
		t.Fatal("Output should contain 'No links found' for empty results")
	}
}

func TestTool_Invoke_SystemPromptPresent(t *testing.T) {
	var capturedReq model.Request
	client := &captureClient{
		capture: func(req model.Request) {
			capturedReq = req
		},
		events: []model.Event{},
	}
	tool := NewTool(client, "claude-sonnet-4-5")
	_, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "test"},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	if capturedReq.System == "" {
		t.Fatal("Request should have system prompt for web search assistant")
	}
	if len(capturedReq.Messages) != 1 {
		t.Fatalf("Request should have 1 message, got %d", len(capturedReq.Messages))
	}
	msg := capturedReq.Messages[0]
	if msg.Role != message.RoleUser {
		t.Fatalf("Message role = %q, want user", msg.Role)
	}
	if len(msg.Content) != 1 {
		t.Fatalf("Message content length = %d, want 1", len(msg.Content))
	}
}

func TestTool_Invoke_NoPanicOnEmptyStream(t *testing.T) {
	client := &stubModelClient{
		events: []model.Event{},
	}
	tool := NewTool(client, "claude-sonnet-4-5")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "test search"},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke result error = %q", result.Error)
	}
}
