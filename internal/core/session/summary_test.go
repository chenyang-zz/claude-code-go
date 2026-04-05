package session

import (
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// TestDerivePreviewUsesLatestUserText verifies recent-session previews prefer the newest user-authored text.
func TestDerivePreviewUsesLatestUserText(t *testing.T) {
	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("first prompt")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("reply")}},
		{Role: message.RoleUser, Content: []message.ContentPart{message.ToolResultPart("tool-1", "ignored tool output", false)}},
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("latest   prompt\nwith spacing")}},
	}

	got := DerivePreview(messages)
	if got != "latest prompt with spacing" {
		t.Fatalf("DerivePreview() = %q, want latest normalized user text", got)
	}
}

// TestDerivePreviewFallsBackToFirstUserText verifies older histories without a latest user text still expose a stable preview.
func TestDerivePreviewFallsBackToFirstUserText(t *testing.T) {
	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("first prompt")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.ToolUsePart("tool-1", "read", map[string]any{"path": "a.txt"})}},
	}

	got := DerivePreview(messages)
	if got != "first prompt" {
		t.Fatalf("DerivePreview() = %q, want first prompt fallback", got)
	}
}

// TestDerivePreviewTruncatesLongText verifies previews stay bounded for one-line list rendering.
func TestDerivePreviewTruncatesLongText(t *testing.T) {
	long := strings.Repeat("x", 100)
	got := DerivePreview([]message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart(long)}},
	})

	if len([]rune(got)) != summaryPreviewLimit {
		t.Fatalf("DerivePreview() rune len = %d, want %d", len([]rune(got)), summaryPreviewLimit)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("DerivePreview() = %q, want truncated ellipsis", got)
	}
}
