package tokenestimation

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestEstimateTokensForText_Empty(t *testing.T) {
	if got := EstimateTokensForText(""); got != 0 {
		t.Fatalf("EstimateTokensForText(\"\") = %d, want 0", got)
	}
}

func TestEstimateTokensForText_Short(t *testing.T) {
	// "hello" = 5 chars / 4 = 1 token (ceiling)
	got := EstimateTokensForText("hello")
	if got != 2 {
		t.Fatalf("EstimateTokensForText(\"hello\") = %d, want 2", got)
	}
}

func TestEstimateTokensForText_Exact(t *testing.T) {
	// 4 chars = 1 token
	got := EstimateTokensForText("abcd")
	if got != 1 {
		t.Fatalf("EstimateTokensForText(\"abcd\") = %d, want 1", got)
	}
}

func TestEstimateTokensForText_Long(t *testing.T) {
	// 100 chars / 4 = 25 tokens
	input := make([]byte, 100)
	for i := range input {
		input[i] = 'a'
	}
	got := EstimateTokensForText(string(input))
	if got != 25 {
		t.Fatalf("EstimateTokensForText(100*'a') = %d, want 25", got)
	}
}

func TestBytesPerTokenForFileType_Default(t *testing.T) {
	if got := BytesPerTokenForFileType("go"); got != 4 {
		t.Fatalf("BytesPerTokenForFileType(\"go\") = %d, want 4", got)
	}
	if got := BytesPerTokenForFileType("txt"); got != 4 {
		t.Fatalf("BytesPerTokenForFileType(\"txt\") = %d, want 4", got)
	}
	if got := BytesPerTokenForFileType("py"); got != 4 {
		t.Fatalf("BytesPerTokenForFileType(\"py\") = %d, want 4", got)
	}
}

func TestBytesPerTokenForFileType_Json(t *testing.T) {
	if got := BytesPerTokenForFileType("json"); got != 2 {
		t.Fatalf("BytesPerTokenForFileType(\"json\") = %d, want 2", got)
	}
	if got := BytesPerTokenForFileType("JSON"); got != 2 {
		t.Fatalf("BytesPerTokenForFileType(\"JSON\") = %d, want 2", got)
	}
	if got := BytesPerTokenForFileType("jsonl"); got != 2 {
		t.Fatalf("BytesPerTokenForFileType(\"jsonl\") = %d, want 2", got)
	}
	if got := BytesPerTokenForFileType("jsonc"); got != 2 {
		t.Fatalf("BytesPerTokenForFileType(\"jsonc\") = %d, want 2", got)
	}
}

func TestEstimateTokensForFileType_Default(t *testing.T) {
	// 8 chars / 4 = 2 tokens
	got := EstimateTokensForFileType("hello world", "txt")
	if got != 3 {
		t.Fatalf("EstimateTokensForFileType(\"hello world\", \"txt\") = %d, want 3", got)
	}
}

func TestEstimateTokensForFileType_Json(t *testing.T) {
	// 8 chars / 2 = 4 tokens
	got := EstimateTokensForFileType(`{"a":1}`, "json")
	if got != 4 {
		t.Fatalf("EstimateTokensForFileType(`{\"a\":1}`, \"json\") = %d, want 4", got)
	}
}

func TestEstimateTokens_Nil(t *testing.T) {
	if got := EstimateTokens(nil); got != 0 {
		t.Fatalf("EstimateTokens(nil) = %d, want 0", got)
	}
}

func TestEstimateTokens_Empty(t *testing.T) {
	if got := EstimateTokens([]message.Message{}); got != 0 {
		t.Fatalf("EstimateTokens([]) = %d, want 0", got)
	}
}

func TestEstimateTokens_SingleTextMessage(t *testing.T) {
	msgs := []message.Message{
		{
			Role: "user",
			Content: []message.ContentPart{
				message.TextPart("hello world"),
			},
		},
	}
	// "hello world" = 11 chars / 4 = 3 (ceiling)
	got := EstimateTokens(msgs)
	if got != 3 {
		t.Fatalf("EstimateTokens(user \"hello world\") = %d, want 3", got)
	}
}

func TestEstimateTokens_ToolUse(t *testing.T) {
	msgs := []message.Message{
		{
			Role: "assistant",
			Content: []message.ContentPart{
				message.ToolUsePart("call1", "BashTool", map[string]any{
					"command": "echo hello",
				}),
			},
		},
	}
	got := EstimateTokens(msgs)
	if got <= 0 {
		t.Fatalf("EstimateTokens(tool_use) = %d, want > 0", got)
	}
}

func TestEstimateTokens_MultipleMessages(t *testing.T) {
	msgs := []message.Message{
		{Role: "user", Content: []message.ContentPart{message.TextPart("a")}},
		{Role: "assistant", Content: []message.ContentPart{message.TextPart("b")}},
		{Role: "user", Content: []message.ContentPart{message.TextPart("c")}},
	}
	// each message has 1 char = 1/4 = 1 token ceiling → 3 total
	got := EstimateTokens(msgs)
	if got != 3 {
		t.Fatalf("EstimateTokens(3 short messages) = %d, want 3", got)
	}
}

func TestEstimateContentTokens_Text(t *testing.T) {
	block := message.TextPart("test")
	got := EstimateContentTokens(block)
	if got != 1 {
		t.Fatalf("EstimateContentTokens(text \"test\") = %d, want 1", got)
	}
}

func TestEstimateContentTokens_ToolUse(t *testing.T) {
	block := message.ToolUsePart("id1", "ReadTool", map[string]any{"path": "/tmp/test"})
	got := EstimateContentTokens(block)
	if got <= 0 {
		t.Fatalf("EstimateContentTokens(tool_use) = %d, want > 0", got)
	}
}

func TestEstimateMessagesTokens(t *testing.T) {
	msgs := []message.Message{
		{Role: "user", Content: []message.ContentPart{message.TextPart("a")}},
	}
	got := EstimateMessagesTokens(msgs)
	if got != 1 {
		t.Fatalf("EstimateMessagesTokens() = %d, want 1", got)
	}
}

func TestEstimateToolsTokens_Empty(t *testing.T) {
	if got := EstimateToolsTokens(nil); got != 0 {
		t.Fatalf("EstimateToolsTokens(nil) = %d, want 0", got)
	}
	if got := EstimateToolsTokens([]map[string]any{}); got != 0 {
		t.Fatalf("EstimateToolsTokens([]) = %d, want 0", got)
	}
}

func TestEstimateToolsTokens_Single(t *testing.T) {
	tools := []map[string]any{
		{
			"name":        "test_tool",
			"description": "A test tool",
			"input_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"param": map[string]any{"type": "string"},
				},
			},
		},
	}
	got := EstimateToolsTokens(tools)
	if got <= 0 {
		t.Fatalf("EstimateToolsTokens(single) = %d, want > 0", got)
	}
}

func TestEstimateToolsTokens_Multiple(t *testing.T) {
	tools := []map[string]any{
		{"name": "tool_a", "description": "a"},
		{"name": "tool_b", "description": "b"},
	}
	a := EstimateToolsTokens(tools[:1])
	b := EstimateToolsTokens(tools[1:])
	ab := EstimateToolsTokens(tools)
	if a+b != ab {
		t.Fatalf("EstimateToolsTokens not additive: %d + %d = %d, want %d = %d", a, b, a+b, ab, a+b)
	}
}

func TestRoughEstimate_ZeroBytesPerToken(t *testing.T) {
	// This tests the internal roughEstimateWithRatio with bytesPerToken <= 0
	// Since BytesPerTokenForFileType never returns <= 0, this is a safety check
	if got := EstimateTokensForText(""); got != 0 {
		t.Fatalf("EstimateTokensForText(\"\") = %d, want 0", got)
	}
}

func TestEstimateTokensForText_Large(t *testing.T) {
	// 4000 chars / 4 = 1000 tokens
	input := make([]byte, 4000)
	for i := range input {
		input[i] = 'x'
	}
	got := EstimateTokensForText(string(input))
	if got != 1000 {
		t.Fatalf("EstimateTokensForText(4000*'x') = %d, want 1000", got)
	}
}

func TestEstimateTokensForFileType_UnknownExtension(t *testing.T) {
	got := EstimateTokensForFileType("test content", "")
	if got <= 0 {
		t.Fatalf("EstimateTokensForFileType with empty extension = %d, want > 0", got)
	}
}
