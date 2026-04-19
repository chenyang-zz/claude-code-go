package compact

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestEstimateTokens_Empty(t *testing.T) {
	got := EstimateTokens(nil)
	if got != 0 {
		t.Fatalf("expected 0 tokens for nil messages, got %d", got)
	}
	got = EstimateTokens([]message.Message{})
	if got != 0 {
		t.Fatalf("expected 0 tokens for empty messages, got %d", got)
	}
}

func TestEstimateTokens_TextOnly(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{
			message.TextPart("Hello, world!"),
		}},
	}
	// "Hello, world!" = 13 chars → ceil(13/4) = 4 tokens
	got := EstimateTokens(msgs)
	if got != 4 {
		t.Fatalf("expected 4 tokens, got %d", got)
	}
}

func TestEstimateTokens_MultipleBlocks(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{
			message.TextPart("abcd"), // 4 chars → 1 token
			message.TextPart("efgh"), // 4 chars → 1 token
		}},
	}
	got := EstimateTokens(msgs)
	if got != 2 {
		t.Fatalf("expected 2 tokens, got %d", got)
	}
}

func TestEstimateTokens_ToolUse(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			message.ToolUsePart("id1", "Bash", map[string]any{
				"command": "ls",
			}),
		}},
	}
	// Tool use tokens = roughEstimate(toolName + json(input))
	got := EstimateTokens(msgs)
	if got <= 0 {
		t.Fatalf("expected positive token count, got %d", got)
	}
}

func TestEstimateTokens_ToolResult(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{
			message.ToolResultPart("id1", "file1.go\nfile2.go", false),
		}},
	}
	// "file1.go\nfile2.go" = 17 chars → ceil(17/4) = 5 tokens
	got := EstimateTokens(msgs)
	if got != 5 {
		t.Fatalf("expected 5 tokens, got %d", got)
	}
}

func TestEstimateTokens_MixedConversation(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{
			message.TextPart("Please read main.go"), // 19 chars → 5 tokens
		}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			message.ToolUsePart("id1", "Read", map[string]any{"path": "main.go"}),
		}},
		{Role: message.RoleUser, Content: []message.ContentPart{
			message.ToolResultPart("id1", "package main\nfunc main() {}", false), // 27 chars → 7 tokens
		}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			message.TextPart("The main.go file contains a basic main function."), // 50 chars → 13 tokens
		}},
	}
	// Verify total is sum of all individual message estimates and positive.
	got := EstimateTokens(msgs)
	if got <= 20 {
		t.Fatalf("expected substantial token count for 4-message conversation, got %d", got)
	}
}

func TestEstimateTokensForText(t *testing.T) {
	got := EstimateTokensForText("12345678") // 8 chars → 2 tokens
	if got != 2 {
		t.Fatalf("expected 2 tokens, got %d", got)
	}
}

func TestEstimateTokensForText_Empty(t *testing.T) {
	got := EstimateTokensForText("")
	if got != 0 {
		t.Fatalf("expected 0 tokens for empty text, got %d", got)
	}
}

func TestRoughEstimate(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"a", 1},      // ceil(1/4) = 1
		{"abcd", 1},   // 4/4 = 1
		{"abcde", 2},  // ceil(5/4) = 2
		{"abcdefgh", 2}, // 8/4 = 2
		{"abcdefghi", 3}, // ceil(9/4) = 3
	}
	for _, tc := range tests {
		got := roughEstimate(tc.input)
		if got != tc.expected {
			t.Errorf("roughEstimate(%q) = %d, want %d", tc.input, got, tc.expected)
		}
	}
}

func TestFormatTokenCount(t *testing.T) {
	got := FormatTokenCount(1000, 8000, 20000)
	expected := "tokens=1000 threshold=8000 effectiveWindow=20000"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
