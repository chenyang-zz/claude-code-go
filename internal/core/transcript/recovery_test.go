package transcript

import (
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestRecoverEntries_Empty(t *testing.T) {
	result := RecoverEntries(nil)
	if len(result.Messages) != 0 {
		t.Fatalf("message count = %d, want 0", len(result.Messages))
	}
	if len(result.Summaries) != 0 {
		t.Fatalf("summary count = %d, want 0", len(result.Summaries))
	}
	if len(result.CompactBoundaries) != 0 {
		t.Fatalf("boundary count = %d, want 0", len(result.CompactBoundaries))
	}
}

func TestRecoverEntries_UserAndAssistant(t *testing.T) {
	entries := []any{
		UserEntry{
			Type:    "user",
			Message: message.Message{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		},
		AssistantEntry{
			Type:    "assistant",
			Message: message.Message{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("hi")}},
		},
	}

	result := RecoverEntries(entries)
	if len(result.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(result.Messages))
	}
	if result.Messages[0].Role != message.RoleUser {
		t.Fatalf("messages[0].role = %q, want user", result.Messages[0].Role)
	}
	if result.Messages[1].Role != message.RoleAssistant {
		t.Fatalf("messages[1].role = %q, want assistant", result.Messages[1].Role)
	}
}

func TestRecoverEntries_SkipsRedundantToolEntries(t *testing.T) {
	entries := []any{
		AssistantEntry{
			Type: "assistant",
			Message: message.Message{
				Role: message.RoleAssistant,
				Content: []message.ContentPart{
					message.TextPart("thinking"),
					message.ToolUsePart("tu1", "Read", map[string]any{"file_path": "main.go"}),
				},
			},
		},
		ToolUseEntry{Type: "tool_use", ToolUseID: "tu1", Name: "Read", Input: map[string]any{"file_path": "main.go"}},
		UserEntry{
			Type: "user",
			Message: message.Message{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					message.ToolResultPart("tu1", "contents", false),
				},
			},
		},
		ToolResultEntry{Type: "tool_result", ToolUseID: "tu1", Output: "contents", IsError: false},
	}

	result := RecoverEntries(entries)
	if len(result.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(result.Messages))
	}
	if len(result.Messages[0].Content) != 2 {
		t.Fatalf("assistant content count = %d, want 2", len(result.Messages[0].Content))
	}
	if len(result.Messages[1].Content) != 1 {
		t.Fatalf("user content count = %d, want 1", len(result.Messages[1].Content))
	}
}

func TestRecoverEntries_CompactBoundary(t *testing.T) {
	entries := []any{
		UserEntry{
			Type:    "user",
			Message: message.Message{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("before")}},
		},
		SystemEntry{
			Type:    "system",
			Subtype: "compact_boundary",
			CompactMetadata: &CompactMetadata{
				Trigger:        "auto",
				PreTokenCount:  5000,
				PostTokenCount: 200,
			},
		},
		AssistantEntry{
			Type:    "assistant",
			Message: message.Message{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("after")}},
		},
	}

	result := RecoverEntries(entries)
	if len(result.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(result.Messages))
	}
	if len(result.CompactBoundaries) != 1 {
		t.Fatalf("boundary count = %d, want 1", len(result.CompactBoundaries))
	}
	b := result.CompactBoundaries[0]
	if b.Trigger != "auto" {
		t.Fatalf("trigger = %q, want auto", b.Trigger)
	}
	if b.PreTokenCount != 5000 {
		t.Fatalf("pre = %d, want 5000", b.PreTokenCount)
	}
	if b.PostTokenCount != 200 {
		t.Fatalf("post = %d, want 200", b.PostTokenCount)
	}
	if b.MessageIndex != 1 {
		t.Fatalf("messageIndex = %d, want 1", b.MessageIndex)
	}
}

func TestRecoverEntries_Summary(t *testing.T) {
	entries := []any{
		UserEntry{
			Type:    "user",
			Message: message.Message{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		},
		SummaryEntry{Type: "summary", Summary: "summary text"},
		AssistantEntry{
			Type:    "assistant",
			Message: message.Message{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("hi")}},
		},
	}

	result := RecoverEntries(entries)
	if len(result.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(result.Messages))
	}
	if len(result.Summaries) != 1 {
		t.Fatalf("summary count = %d, want 1", len(result.Summaries))
	}
	if result.Summaries[0].Summary != "summary text" {
		t.Fatalf("summary = %q, want 'summary text'", result.Summaries[0].Summary)
	}
}

func TestRecoverEntries_UnknownTypeSkipped(t *testing.T) {
	entries := []any{
		UserEntry{
			Type:    "user",
			Message: message.Message{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		},
		"not an entry", // unknown type — should be skipped
		AssistantEntry{
			Type:    "assistant",
			Message: message.Message{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("hi")}},
		},
	}

	result := RecoverEntries(entries)
	if len(result.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(result.Messages))
	}
}

func TestRecoverEntries_LegacyProgressSkipped(t *testing.T) {
	entries := []any{
		UserEntry{
			Type:    "user",
			Message: message.Message{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		},
		ProgressEntry{
			Type:       "progress",
			UUID:       "p-1",
			ParentUUID: "u-1",
			Data:       map[string]any{"type": "mcp_progress"},
		},
		AssistantEntry{
			Type:    "assistant",
			Message: message.Message{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("hi")}},
		},
	}

	result := RecoverEntries(entries)
	if len(result.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(result.Messages))
	}
}

func TestRecoverFile_NotFound(t *testing.T) {
	_, err := RecoverFile("/nonexistent/transcript.jsonl")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestValidateEntry(t *testing.T) {
	tests := []struct {
		name    string
		entry   any
		wantErr bool
	}{
		{
			name:    "valid user",
			entry:   UserEntry{Type: "user", Message: message.Message{Role: message.RoleUser}},
			wantErr: false,
		},
		{
			name:    "user wrong role",
			entry:   UserEntry{Type: "user", Message: message.Message{Role: message.RoleAssistant}},
			wantErr: true,
		},
		{
			name:    "valid assistant",
			entry:   AssistantEntry{Type: "assistant", Message: message.Message{Role: message.RoleAssistant}},
			wantErr: false,
		},
		{
			name:    "assistant wrong role",
			entry:   AssistantEntry{Type: "assistant", Message: message.Message{Role: message.RoleUser}},
			wantErr: true,
		},
		{
			name:    "valid tool_use",
			entry:   ToolUseEntry{Type: "tool_use", ToolUseID: "tu1", Name: "Read"},
			wantErr: false,
		},
		{
			name:    "tool_use missing id",
			entry:   ToolUseEntry{Type: "tool_use", ToolUseID: ""},
			wantErr: true,
		},
		{
			name:    "valid tool_result",
			entry:   ToolResultEntry{Type: "tool_result", ToolUseID: "tu1", Output: "ok"},
			wantErr: false,
		},
		{
			name:    "tool_result missing id",
			entry:   ToolResultEntry{Type: "tool_result", ToolUseID: ""},
			wantErr: true,
		},
		{
			name:    "valid system",
			entry:   SystemEntry{Type: "system", Subtype: "compact_boundary"},
			wantErr: false,
		},
		{
			name:    "system missing subtype",
			entry:   SystemEntry{Type: "system", Subtype: ""},
			wantErr: true,
		},
		{
			name:    "valid summary",
			entry:   SummaryEntry{Type: "summary", Summary: "text"},
			wantErr: false,
		},
		{
			name:    "valid progress",
			entry:   ProgressEntry{Type: "progress", UUID: "p-1"},
			wantErr: false,
		},
		{
			name:    "summary empty",
			entry:   SummaryEntry{Type: "summary", Summary: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEntry(tt.entry)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRecoverEntries_SkipsInvalidEntries(t *testing.T) {
	entries := []any{
		UserEntry{
			Type:    "user",
			Message: message.Message{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("valid")}},
		},
		UserEntry{
			Type:    "user",
			Message: message.Message{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("invalid role")}},
		},
		AssistantEntry{
			Type:    "assistant",
			Message: message.Message{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("valid")}},
		},
	}

	result := RecoverEntries(entries)
	if len(result.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(result.Messages))
	}
	if result.Messages[0].Content[0].Text != "valid" {
		t.Fatalf("messages[0] text = %q, want valid", result.Messages[0].Content[0].Text)
	}
	if result.Messages[1].Content[0].Text != "valid" {
		t.Fatalf("messages[1] text = %q, want valid", result.Messages[1].Content[0].Text)
	}
}

func TestRecoverFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.jsonl"

	writer, err := NewWriter(path)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}

	ts := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("hi")}},
	}
	for _, msg := range messages {
		for _, entry := range EntriesFromMessage(ts, msg) {
			if err := writer.WriteEntry(entry); err != nil {
				t.Fatalf("write entry: %v", err)
			}
		}
	}
	if err := writer.WriteEntry(NewSummaryEntry(ts, "session summary")); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	result, err := RecoverFile(path)
	if err != nil {
		t.Fatalf("recover file: %v", err)
	}
	if len(result.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(result.Messages))
	}
	if result.Messages[0].Content[0].Text != "hello" {
		t.Fatalf("msg0 text = %q, want hello", result.Messages[0].Content[0].Text)
	}
	if result.Messages[1].Content[0].Text != "hi" {
		t.Fatalf("msg1 text = %q, want hi", result.Messages[1].Content[0].Text)
	}
	if len(result.Summaries) != 1 {
		t.Fatalf("summary count = %d, want 1", len(result.Summaries))
	}
}
