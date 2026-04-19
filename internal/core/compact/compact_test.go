package compact

import (
	"context"
	"errors"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// mockClient implements model.Client for testing.
type mockClient struct {
	// response is the text the mock client returns.
	response string
	// err is the error the mock client returns on Stream.
	err error
	// streamErr is the error returned mid-stream.
	streamErr string
	// usage is attached to the done event.
	usage *model.Usage
	// requests records incoming requests for assertions.
	requests []model.Request
}

func (m *mockClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	m.requests = append(m.requests, req)
	if m.err != nil {
		return nil, m.err
	}

	ch := make(chan model.Event, 10)
	go func() {
		defer close(ch)
		if m.streamErr != "" {
			ch <- model.Event{Type: model.EventTypeError, Error: m.streamErr}
			return
		}
		// Send response as text delta(s).
		if m.response != "" {
			ch <- model.Event{Type: model.EventTypeTextDelta, Text: m.response}
		}
		ch <- model.Event{Type: model.EventTypeDone, Usage: m.usage}
	}()
	return ch, nil
}

func TestCompactConversation_BasicFlow(t *testing.T) {
	client := &mockClient{
		response: "<analysis>Thinking...</analysis><summary>1. Primary Request: Fix bug\n2. Key Concepts: Testing</summary>",
		usage:    &model.Usage{InputTokens: 120, OutputTokens: 45},
	}

	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("Fix the login bug")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("I'll fix it.")}},
	}

	result, err := CompactConversation(context.Background(), client, CompactRequest{
		Messages: msgs,
		Model:    "claude-sonnet-4-20250514",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !IsCompactBoundary(&result.Boundary) {
		t.Error("expected boundary marker in result")
	}
	if len(result.SummaryMessages) != 1 {
		t.Fatalf("expected 1 summary message, got %d", len(result.SummaryMessages))
	}
	if result.Usage.InputTokens != 120 || result.Usage.OutputTokens != 45 {
		t.Fatalf("usage = %+v, want 120/45", result.Usage)
	}
	if result.PreTokenCount <= 0 {
		t.Error("expected positive pre-token count")
	}
}

func TestCompactConversation_RequestsLargeSummaryBudget(t *testing.T) {
	client := &mockClient{
		response: "summary",
	}

	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("Summarize this")}},
	}

	_, err := CompactConversation(context.Background(), client, CompactRequest{
		Messages: msgs,
		Model:    "claude-sonnet-4-20250514",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.requests) != 1 {
		t.Fatalf("request count = %d, want 1", len(client.requests))
	}
	if client.requests[0].MaxOutputTokens != MaxOutputTokensForSummary {
		t.Fatalf("MaxOutputTokens = %d, want %d", client.requests[0].MaxOutputTokens, MaxOutputTokensForSummary)
	}
}

func TestCompactConversation_PreservesLatestUserMessage(t *testing.T) {
	client := &mockClient{
		response: "<summary>summary</summary>",
	}

	latestPrompt := "Use `go test ./...` and do not modify generated files."
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("Earlier request")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("Earlier response")}},
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart(latestPrompt)}},
	}

	result, err := CompactConversation(context.Background(), client, CompactRequest{
		Messages: msgs,
		Model:    "claude-sonnet-4-20250514",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.requests) != 1 {
		t.Fatalf("request count = %d, want 1", len(client.requests))
	}
	reqMessages := client.requests[0].Messages
	if got := len(reqMessages); got != 3 {
		t.Fatalf("summary request message count = %d, want 3", got)
	}
	if got := reqMessages[len(reqMessages)-2].Content[0].Text; got != "Earlier response" {
		t.Fatalf("last conversation message summarized = %q, want Earlier response", got)
	}
	if got := reqMessages[len(reqMessages)-1].Content[0].Text; got == latestPrompt {
		t.Fatal("latest user prompt should not be included in summary input")
	}
	if got := len(result.SummaryMessages); got != 2 {
		t.Fatalf("SummaryMessages len = %d, want 2", got)
	}
	if got := result.SummaryMessages[1].Content[0].Text; got != latestPrompt {
		t.Fatalf("preserved latest user prompt = %q, want %q", got, latestPrompt)
	}
}

func TestCompactConversation_PreservesActiveToolLoopMessages(t *testing.T) {
	client := &mockClient{
		response: "<summary>summary</summary>",
	}

	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("Earlier request")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("Earlier response")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			message.ToolUsePart("call_1", "Read", map[string]any{"path": "main.go"}),
		}},
		{Role: message.RoleUser, Content: []message.ContentPart{
			message.ToolResultPart("call_1", "package main", false),
		}},
	}

	result, err := CompactConversation(context.Background(), client, CompactRequest{
		Messages: msgs,
		Model:    "claude-sonnet-4-20250514",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	reqMessages := client.requests[0].Messages
	if got := len(reqMessages); got != 3 {
		t.Fatalf("summary request message count = %d, want 3", got)
	}
	if len(result.SummaryMessages) != 3 {
		t.Fatalf("SummaryMessages len = %d, want 3", len(result.SummaryMessages))
	}
	if result.SummaryMessages[1].Role != message.RoleAssistant {
		t.Fatalf("preserved assistant tool call role = %s, want assistant", result.SummaryMessages[1].Role)
	}
	if result.SummaryMessages[2].Role != message.RoleUser {
		t.Fatalf("preserved tool result role = %s, want user", result.SummaryMessages[2].Role)
	}
}

func TestCompactConversation_EmptyMessages(t *testing.T) {
	client := &mockClient{}
	_, err := CompactConversation(context.Background(), client, CompactRequest{
		Messages: []message.Message{},
	})
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
}

func TestCompactConversation_StreamError(t *testing.T) {
	client := &mockClient{
		err: errors.New("connection failed"),
	}

	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
	}

	_, err := CompactConversation(context.Background(), client, CompactRequest{
		Messages: msgs,
		Model:    "claude-sonnet-4-20250514",
	})
	if err == nil {
		t.Fatal("expected error from stream failure")
	}
}

func TestCompactConversation_MidStreamError(t *testing.T) {
	client := &mockClient{
		streamErr: "prompt_too_long",
	}

	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
	}

	_, err := CompactConversation(context.Background(), client, CompactRequest{
		Messages: msgs,
		Model:    "claude-sonnet-4-20250514",
	})
	if err == nil {
		t.Fatal("expected error from mid-stream failure")
	}
}

func TestCompactConversation_EmptySummaryResponse(t *testing.T) {
	client := &mockClient{
		response: "", // Empty response.
	}

	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
	}

	_, err := CompactConversation(context.Background(), client, CompactRequest{
		Messages: msgs,
		Model:    "claude-sonnet-4-20250514",
	})
	if err == nil {
		t.Fatal("expected error for empty summary response")
	}
}

func TestStripImagesFromMessages_NoImages(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
	}
	result := stripImagesFromMessages(msgs)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	if result[0].Content[0].Text != "hello" {
		t.Error("text content should be unchanged")
	}
}

func TestStripImagesFromMessages_WithImages(t *testing.T) {
	msgs := []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart("Here's a screenshot:"),
				{Type: "image", Text: "base64data..."},
				{Type: "document", Text: "base64doc..."},
			},
		},
	}
	result := stripImagesFromMessages(msgs)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	if len(result[0].Content) != 3 {
		t.Fatalf("expected 3 content parts, got %d", len(result[0].Content))
	}
	if result[0].Content[1].Text != "[image]" {
		t.Errorf("expected [image] marker, got %q", result[0].Content[1].Text)
	}
	if result[0].Content[2].Text != "[document]" {
		t.Errorf("expected [document] marker, got %q", result[0].Content[2].Text)
	}
}

func TestStripImagesFromMessages_AssistantUnchanged(t *testing.T) {
	msgs := []message.Message{
		{
			Role: message.RoleAssistant,
			Content: []message.ContentPart{
				{Type: "image", Text: "should not be stripped"},
			},
		},
	}
	result := stripImagesFromMessages(msgs)
	// Assistant messages should not be modified.
	if result[0].Content[0].Text != "should not be stripped" {
		t.Error("assistant image blocks should not be stripped")
	}
}

func TestSplitMessagesForCompaction_PreserveEntireCandidateTail(t *testing.T) {
	msgs := []message.Message{
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
	}

	toSummarize, preservedTail := splitMessagesForCompaction(msgs)

	if len(toSummarize) != 0 {
		t.Fatalf("messagesToSummarize = %d, want 0", len(toSummarize))
	}
	if len(preservedTail) != len(msgs) {
		t.Fatalf("preservedTail = %d messages, want %d", len(preservedTail), len(msgs))
	}
	if preservedTail[0].Role != message.RoleAssistant || preservedTail[1].Role != message.RoleUser {
		t.Fatalf("preservedTail roles = [%s, %s], want [assistant, user]", preservedTail[0].Role, preservedTail[1].Role)
	}
}

func TestAutoCompactIfNeeded_Disabled(t *testing.T) {
	t.Setenv("DISABLE_COMPACT", "1")
	client := &mockClient{response: "summary"}
	msgs := createLargeMessages()

	result, err := AutoCompactIfNeeded(context.Background(), client, msgs, "claude-sonnet-4-20250514", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result when disabled")
	}
}

func TestAutoCompactIfNeeded_BelowThreshold(t *testing.T) {
	client := &mockClient{response: "summary"}
	// Small messages won't trigger threshold.
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
	}

	result, err := AutoCompactIfNeeded(context.Background(), client, msgs, "claude-sonnet-4-20250514", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result for below-threshold messages")
	}
}

func TestAutoCompactIfNeeded_CircuitBreaker(t *testing.T) {
	client := &mockClient{response: "summary"}
	tracking := &TrackingState{
		ConsecutiveFailures: MaxConsecutiveAutoCompactFailures,
	}
	msgs := createLargeMessages()

	result, err := AutoCompactIfNeeded(context.Background(), client, msgs, "claude-sonnet-4-20250514", tracking, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result when circuit breaker is tripped")
	}
}

func TestAutoCompactIfNeeded_Success(t *testing.T) {
	client := &mockClient{
		response: "<analysis>analysis</analysis><summary>Summary content</summary>",
	}
	tracking := &TrackingState{}
	msgs := createLargeMessages()

	result, err := AutoCompactIfNeeded(context.Background(), client, msgs, "claude-sonnet-4-20250514", tracking, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for successful auto-compact")
	}
	if tracking.ConsecutiveFailures != 0 {
		t.Fatalf("expected failures reset to 0, got %d", tracking.ConsecutiveFailures)
	}
	if !tracking.Compacted {
		t.Fatal("expected tracking to show compacted")
	}
}

func TestAutoCompactIfNeeded_FailureIncrementsCounter(t *testing.T) {
	client := &mockClient{err: errors.New("stream failed")}
	tracking := &TrackingState{}
	msgs := createLargeMessages()

	result, err := AutoCompactIfNeeded(context.Background(), client, msgs, "claude-sonnet-4-20250514", tracking, "")
	if err == nil {
		t.Fatal("expected error from stream failure")
	}
	if result != nil {
		t.Fatal("expected nil result on failure")
	}
	if tracking.ConsecutiveFailures != 1 {
		t.Fatalf("expected 1 consecutive failure, got %d", tracking.ConsecutiveFailures)
	}
}

// createLargeMessages creates enough messages to exceed the auto-compact
// threshold for testing purposes.
func createLargeMessages() []message.Message {
	// Need enough tokens to exceed the threshold.
	// Threshold = effectiveWindow - 13000 = (200000 - 20000) - 13000 = 167000
	// At 4 chars/token, need 167000 * 4 = 668000 chars
	chunkSize := 10000
	numChunks := 70 // 700000 chars → ~175000 tokens, well above threshold
	msgs := make([]message.Message, numChunks)
	for i := range msgs {
		text := make([]byte, chunkSize)
		for j := range text {
			text[j] = 'x'
		}
		msgs[i] = message.Message{
			Role:    message.RoleUser,
			Content: []message.ContentPart{message.TextPart(string(text))},
		}
	}
	return msgs
}
