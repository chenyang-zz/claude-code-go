package openai

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

func TestBuildResponsesRequest(t *testing.T) {
	req := model.Request{
		Model:           "o3-mini",
		System:          "You are a helpful assistant.",
		MaxOutputTokens: 2048,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "What is 2+2?"}}},
		},
		Tools: []model.ToolDefinition{
			{Name: "calc", Description: "Calculator", InputSchema: map[string]any{"type": "object"}},
		},
	}

	r := buildResponsesRequest(req)
	if r.Model != "o3-mini" {
		t.Errorf("model: got %q, want o3-mini", r.Model)
	}
	if len(r.Input) != 2 {
		t.Fatalf("input length: got %d, want 2", len(r.Input))
	}
	if r.Input[0].Role != "developer" || r.Input[0].Content != "You are a helpful assistant." {
		t.Errorf("system message mismatch: %+v", r.Input[0])
	}
	if r.Input[1].Role != "user" || r.Input[1].Content != "What is 2+2?" {
		t.Errorf("user message mismatch: %+v", r.Input[1])
	}
	if !r.Stream {
		t.Error("stream should be true")
	}
	if r.MaxOutputTokens != 2048 {
		t.Errorf("max_output_tokens: got %d, want 2048", r.MaxOutputTokens)
	}
	if len(r.Tools) != 1 || r.Tools[0].Function.Name != "calc" {
		t.Errorf("tools mismatch")
	}
}

func TestBuildResponsesRequestNoSystem(t *testing.T) {
	req := model.Request{
		Model:    "o3-mini",
		Messages: []message.Message{},
	}
	r := buildResponsesRequest(req)
	if len(r.Input) != 0 {
		t.Errorf("expected empty input, got %d items", len(r.Input))
	}
}

func TestMapMessagesToResponsesInputWithToolResults(t *testing.T) {
	msgs := []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				{Type: "text", Text: "What is the weather?"},
			},
		},
		{
			Role: message.RoleAssistant,
			Content: []message.ContentPart{
				{Type: "tool_use", ToolUseID: "call_1", ToolName: "get_weather", ToolInput: map[string]any{"city": "NYC"}},
			},
		},
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				{Type: "tool_result", ToolUseID: "call_1", Text: "Sunny, 72F"},
			},
		},
	}

	input := mapMessagesToResponsesInput("", msgs)
	if len(input) != 3 {
		t.Fatalf("input length: got %d, want 3", len(input))
	}
	if input[0].Role != "user" || input[0].Content != "What is the weather?" {
		t.Errorf("first message mismatch")
	}
	if input[1].Role != "user" {
		t.Errorf("expected user role for tool_use mapping, got %q", input[1].Role)
	}
	if input[2].Role != "user" {
		t.Errorf("expected user role for tool_result, got %q", input[2].Role)
	}
}

func TestParseResponsesOutputTextOnly(t *testing.T) {
	items := []responsesOutputItem{
		{
			Type: "message",
			ID:   "msg_1",
			Role: "assistant",
			Content: []responsesContentPart{
				{Type: "output_text", Text: "Hello"},
				{Type: "output_text", Text: " world"},
			},
		},
	}

	events, err := parseResponsesOutput(items)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("event count: got %d, want 2", len(events))
	}
	if events[0].Text != "Hello" || events[1].Text != " world" {
		t.Errorf("text mismatch: %+v", events)
	}
}

func TestParseResponsesOutputWithFunctionCall(t *testing.T) {
	items := []responsesOutputItem{
		{
			Type:      "function_call",
			ID:        "fc_1",
			CallID:    "call_1",
			Name:      "get_weather",
			Arguments: `{"city":"NYC"}`,
		},
	}

	events, err := parseResponsesOutput(items)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count: got %d, want 1", len(events))
	}
	if events[0].ToolUse == nil {
		t.Fatal("expected tool_use event")
	}
	if events[0].ToolUse.ID != "call_1" {
		t.Errorf("tool use id: got %q, want call_1", events[0].ToolUse.ID)
	}
	if events[0].ToolUse.Name != "get_weather" {
		t.Errorf("tool use name: got %q, want get_weather", events[0].ToolUse.Name)
	}
	city, ok := events[0].ToolUse.Input["city"].(string)
	if !ok || city != "NYC" {
		t.Errorf("tool input mismatch: %+v", events[0].ToolUse.Input)
	}
}

func TestParseResponsesOutputMixed(t *testing.T) {
	items := []responsesOutputItem{
		{Type: "message", ID: "msg_1", Content: []responsesContentPart{{Type: "output_text", Text: "Let me check"}}},
		{Type: "function_call", ID: "fc_1", CallID: "call_1", Name: "calc", Arguments: `{"a":1,"b":2}`},
	}

	events, err := parseResponsesOutput(items)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("event count: got %d, want 2", len(events))
	}
	if events[0].Text != "Let me check" {
		t.Errorf("first event text mismatch")
	}
	if events[1].ToolUse == nil || events[1].ToolUse.Name != "calc" {
		t.Errorf("second event tool use mismatch")
	}
}

func TestParseResponsesOutputInvalidJSON(t *testing.T) {
	items := []responsesOutputItem{
		{Type: "function_call", ID: "fc_1", CallID: "call_1", Name: "calc", Arguments: `{invalid}`},
	}
	_, err := parseResponsesOutput(items)
	if err == nil {
		t.Error("expected error for invalid JSON arguments")
	}
}

// TestResponsesMapperPureText confirms plain text user messages still serialize
// as a string Content for the Responses API, preserving back-compat.
func TestResponsesMapperPureText(t *testing.T) {
	input := mapMessagesToResponsesInput("", []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart("hello"),
			},
		},
	})

	if len(input) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(input))
	}
	if s, ok := input[0].Content.(string); !ok || s != "hello" {
		t.Fatalf("expected string Content \"hello\", got %T %v", input[0].Content, input[0].Content)
	}
}

// TestResponsesMapperInputImage verifies a single ImagePart is serialized into
// the Responses API input_image content part with a string image_url.
func TestResponsesMapperInputImage(t *testing.T) {
	input := mapMessagesToResponsesInput("", []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.ImagePart("image/png", "iVBORw0K"),
			},
		},
	})

	if len(input) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(input))
	}
	parts, ok := input[0].Content.([]responsesInputContentPart)
	if !ok {
		t.Fatalf("expected []responsesInputContentPart, got %T", input[0].Content)
	}
	if len(parts) != 1 {
		t.Fatalf("expected 1 content part, got %d", len(parts))
	}
	if parts[0].Type != "input_image" {
		t.Fatalf("expected input_image part, got %q", parts[0].Type)
	}
	if got, want := parts[0].ImageURL, "data:image/png;base64,iVBORw0K"; got != want {
		t.Fatalf("ImageURL = %q, want %q", got, want)
	}
}

// TestResponsesMapperMixedTextAndImage verifies text and image parts coexist as
// a structured input array using input_text + input_image element types.
func TestResponsesMapperMixedTextAndImage(t *testing.T) {
	input := mapMessagesToResponsesInput("", []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart("describe:"),
				message.ImagePart("image/jpeg", "/9j/4AAQ"),
			},
		},
	})

	parts, ok := input[0].Content.([]responsesInputContentPart)
	if !ok {
		t.Fatalf("expected []responsesInputContentPart, got %T", input[0].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0].Type != "input_text" || parts[0].Text != "describe:" {
		t.Fatalf("unexpected first part: %+v", parts[0])
	}
	if parts[1].Type != "input_image" || parts[1].ImageURL != "data:image/jpeg;base64,/9j/4AAQ" {
		t.Fatalf("unexpected second part: %+v", parts[1])
	}
}

// TestResponsesMapperDocumentDegrade verifies DocumentPart entries are silently
// skipped (with a logger warning) on the Responses API path while sibling
// image / text parts continue to render normally.
func TestResponsesMapperDocumentDegrade(t *testing.T) {
	input := mapMessagesToResponsesInput("", []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart("PDF preview:"),
				message.DocumentPart("application/pdf", "JVBERi0x"),
				message.ImagePart("image/jpeg", "pageOne"),
			},
		},
	})

	parts, ok := input[0].Content.([]responsesInputContentPart)
	if !ok {
		t.Fatalf("expected []responsesInputContentPart, got %T", input[0].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts (text + image, document skipped), got %d", len(parts))
	}
	if parts[0].Type != "input_text" || parts[0].Text != "PDF preview:" {
		t.Fatalf("unexpected first part: %+v", parts[0])
	}
	if parts[1].Type != "input_image" || parts[1].ImageURL != "data:image/jpeg;base64,pageOne" {
		t.Fatalf("unexpected second part: %+v", parts[1])
	}
}
