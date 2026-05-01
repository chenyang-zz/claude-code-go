package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// TestMapMessagesImageBlock verifies that image content parts are mapped to
// Anthropic image blocks with base64 source.
func TestMapMessagesImageBlock(t *testing.T) {
	messages := []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart("look at this"),
				message.ImagePart("image/png", "abc123"),
			},
		},
	}

	result := mapMessages(messages, false)
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if len(result[0].Content) != 2 {
		t.Fatalf("len(content) = %d, want 2", len(result[0].Content))
	}

	textBlock := result[0].Content[0]
	if textBlock.Type != "text" || textBlock.Text != "look at this" {
		t.Fatalf("text block = %+v", textBlock)
	}

	imageBlock := result[0].Content[1]
	if imageBlock.Type != "image" {
		t.Fatalf("image block type = %q, want image", imageBlock.Type)
	}
	if imageBlock.Source == nil {
		t.Fatal("image block Source = nil")
	}
	if imageBlock.Source.Type != "base64" {
		t.Fatalf("source.type = %q, want base64", imageBlock.Source.Type)
	}
	if imageBlock.Source.MediaType != "image/png" {
		t.Fatalf("source.media_type = %q, want image/png", imageBlock.Source.MediaType)
	}
	if imageBlock.Source.Data != "abc123" {
		t.Fatalf("source.data = %q, want abc123", imageBlock.Source.Data)
	}
}

// TestMapMessagesImageBlockSerialization verifies the JSON wire format for
// image blocks matches Anthropic API expectations.
func TestMapMessagesImageBlockSerialization(t *testing.T) {
	messages := []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.ImagePart("image/jpeg", "dGVzdA=="),
			},
		},
	}

	result := mapMessages(messages, false)
	if len(result) != 1 || len(result[0].Content) != 1 {
		t.Fatalf("unexpected result length")
	}

	block := result[0].Content[0]
	got, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(got, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if raw["type"] != "image" {
		t.Fatalf("type = %q, want image", raw["type"])
	}
	if raw["text"] != nil {
		t.Fatalf("text should be omitted for image blocks, got %v", raw["text"])
	}

	source, ok := raw["source"].(map[string]any)
	if !ok {
		t.Fatalf("source = %#v, want object", raw["source"])
	}
	if source["type"] != "base64" {
		t.Fatalf("source.type = %q, want base64", source["type"])
	}
	if source["media_type"] != "image/jpeg" {
		t.Fatalf("source.media_type = %q, want image/jpeg", source["media_type"])
	}
	if source["data"] != "dGVzdA==" {
		t.Fatalf("source.data = %q, want dGVzdA==", source["data"])
	}
}

// TestMapMessagesToolResultWithImage verifies that when a tool_result is
// followed by an image part, both are preserved in the mapped output.
func TestMapMessagesToolResultWithImage(t *testing.T) {
	messages := []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.ToolResultPart("toolu_1", "Read image (1.2 MB)", false),
				message.ImagePart("image/jpeg", "base64data"),
			},
		},
	}

	result := mapMessages(messages, false)
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if len(result[0].Content) != 2 {
		t.Fatalf("len(content) = %d, want 2", len(result[0].Content))
	}

	tr := result[0].Content[0]
	if tr.Type != "tool_result" || tr.ToolUseID != "toolu_1" || tr.Content != "Read image (1.2 MB)" {
		t.Fatalf("tool_result block = %+v", tr)
	}

	img := result[0].Content[1]
	if img.Type != "image" || img.Source == nil || img.Source.Data != "base64data" {
		t.Fatalf("image block = %+v", img)
	}
}
