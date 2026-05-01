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

// TestMapMessagesDocumentBlock verifies that document content parts are mapped
// to Anthropic document blocks with base64 source.
func TestMapMessagesDocumentBlock(t *testing.T) {
	messages := []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart("read this PDF"),
				message.DocumentPart("application/pdf", "JVBERi0xLjQK"),
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
	if textBlock.Type != "text" || textBlock.Text != "read this PDF" {
		t.Fatalf("text block = %+v", textBlock)
	}

	docBlock := result[0].Content[1]
	if docBlock.Type != "document" {
		t.Fatalf("document block type = %q, want document", docBlock.Type)
	}
	if docBlock.Source == nil {
		t.Fatal("document block Source = nil")
	}
	if docBlock.Source.Type != "base64" {
		t.Fatalf("source.type = %q, want base64", docBlock.Source.Type)
	}
	if docBlock.Source.MediaType != "application/pdf" {
		t.Fatalf("source.media_type = %q, want application/pdf", docBlock.Source.MediaType)
	}
	if docBlock.Source.Data != "JVBERi0xLjQK" {
		t.Fatalf("source.data = %q, want JVBERi0xLjQK", docBlock.Source.Data)
	}
}

// TestMapMessagesDocumentBlockSerialization verifies the JSON wire format for
// document blocks matches Anthropic API expectations.
func TestMapMessagesDocumentBlockSerialization(t *testing.T) {
	messages := []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.DocumentPart("application/pdf", "JVBERi0xLjQK"),
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

	if raw["type"] != "document" {
		t.Fatalf("type = %q, want document", raw["type"])
	}
	if raw["text"] != nil {
		t.Fatalf("text should be omitted for document blocks, got %v", raw["text"])
	}

	source, ok := raw["source"].(map[string]any)
	if !ok {
		t.Fatalf("source = %#v, want object", raw["source"])
	}
	if source["type"] != "base64" {
		t.Fatalf("source.type = %q, want base64", source["type"])
	}
	if source["media_type"] != "application/pdf" {
		t.Fatalf("source.media_type = %q, want application/pdf", source["media_type"])
	}
	if source["data"] != "JVBERi0xLjQK" {
		t.Fatalf("source.data = %q, want JVBERi0xLjQK", source["data"])
	}
}

// TestMapMessagesToolResultWithDocument verifies that when a tool_result is
// followed by a document part (PDF), both are preserved alongside each other.
func TestMapMessagesToolResultWithDocument(t *testing.T) {
	messages := []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.ToolResultPart("toolu_2", "Read PDF (2.1 MB)", false),
				message.DocumentPart("application/pdf", "JVBERi0xLjQK"),
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
	if tr.Type != "tool_result" || tr.ToolUseID != "toolu_2" {
		t.Fatalf("tool_result block = %+v", tr)
	}

	doc := result[0].Content[1]
	if doc.Type != "document" || doc.Source == nil || doc.Source.Data != "JVBERi0xLjQK" {
		t.Fatalf("document block = %+v", doc)
	}
}

// TestMapMessagesMixedMediaBlocks verifies that text + multiple images +
// document can coexist in the same user message under the same-message
// append paradigm used by the engine.
func TestMapMessagesMixedMediaBlocks(t *testing.T) {
	messages := []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.ToolResultPart("toolu_3", "Read PDF pages: 3 page(s) extracted", false),
				message.ImagePart("image/jpeg", "page1"),
				message.ImagePart("image/jpeg", "page2"),
				message.ImagePart("image/jpeg", "page3"),
			},
		},
	}

	result := mapMessages(messages, false)
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if len(result[0].Content) != 4 {
		t.Fatalf("len(content) = %d, want 4 (1 tool_result + 3 images)", len(result[0].Content))
	}

	if result[0].Content[0].Type != "tool_result" {
		t.Fatalf("first block type = %q, want tool_result", result[0].Content[0].Type)
	}
	for i := 1; i < 4; i++ {
		if result[0].Content[i].Type != "image" {
			t.Fatalf("block %d type = %q, want image", i, result[0].Content[i].Type)
		}
	}
}
