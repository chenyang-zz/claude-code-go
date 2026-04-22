package transcript

import (
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestEntriesFromMessage_AssistantWithToolUse(t *testing.T) {
	timestamp := time.Date(2026, 4, 22, 10, 30, 0, 0, time.UTC)
	msg := message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentPart{
			message.ToolUsePart("toolu_1", "Read", map[string]any{"file_path": "main.go"}),
		},
	}

	entries := EntriesFromMessage(timestamp, msg)
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}

	assistantEntry, ok := entries[0].(AssistantEntry)
	if !ok {
		t.Fatalf("entries[0] type = %T, want transcript.AssistantEntry", entries[0])
	}
	if assistantEntry.Type != "assistant" {
		t.Fatalf("assistant entry type = %q, want assistant", assistantEntry.Type)
	}
	if !assistantEntry.Timestamp.Equal(timestamp) {
		t.Fatalf("assistant entry timestamp = %v, want %v", assistantEntry.Timestamp, timestamp)
	}

	toolUseEntry, ok := entries[1].(ToolUseEntry)
	if !ok {
		t.Fatalf("entries[1] type = %T, want transcript.ToolUseEntry", entries[1])
	}
	if toolUseEntry.Type != "tool_use" {
		t.Fatalf("tool_use entry type = %q, want tool_use", toolUseEntry.Type)
	}
	if toolUseEntry.ToolUseID != "toolu_1" || toolUseEntry.Name != "Read" {
		t.Fatalf("tool_use entry = %#v, want id=toolu_1 name=Read", toolUseEntry)
	}
}

func TestEntriesFromMessage_UserWithToolResult(t *testing.T) {
	timestamp := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	msg := message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.ToolResultPart("toolu_1", "file contents", false),
		},
	}

	entries := EntriesFromMessage(timestamp, msg)
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}

	userEntry, ok := entries[0].(UserEntry)
	if !ok {
		t.Fatalf("entries[0] type = %T, want transcript.UserEntry", entries[0])
	}
	if userEntry.Type != "user" {
		t.Fatalf("user entry type = %q, want user", userEntry.Type)
	}

	toolResultEntry, ok := entries[1].(ToolResultEntry)
	if !ok {
		t.Fatalf("entries[1] type = %T, want transcript.ToolResultEntry", entries[1])
	}
	if toolResultEntry.ToolUseID != "toolu_1" {
		t.Fatalf("tool_result tool_use_id = %q, want toolu_1", toolResultEntry.ToolUseID)
	}
	if toolResultEntry.Output != "file contents" {
		t.Fatalf("tool_result output = %q, want file contents", toolResultEntry.Output)
	}
	if toolResultEntry.IsError {
		t.Fatal("tool_result is_error = true, want false")
	}
}

func TestNewCompactBoundaryEntry(t *testing.T) {
	timestamp := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	entry := NewCompactBoundaryEntry(timestamp, "auto", 1000, 120)

	if entry.Type != "system" {
		t.Fatalf("type = %q, want system", entry.Type)
	}
	if entry.Subtype != "compact_boundary" {
		t.Fatalf("subtype = %q, want compact_boundary", entry.Subtype)
	}
	if entry.CompactMetadata == nil {
		t.Fatal("compact metadata = nil, want non-nil")
	}
	if entry.CompactMetadata.Trigger != "auto" {
		t.Fatalf("trigger = %q, want auto", entry.CompactMetadata.Trigger)
	}
	if entry.CompactMetadata.PreTokenCount != 1000 {
		t.Fatalf("pre_token_count = %d, want 1000", entry.CompactMetadata.PreTokenCount)
	}
	if entry.CompactMetadata.PostTokenCount != 120 {
		t.Fatalf("post_token_count = %d, want 120", entry.CompactMetadata.PostTokenCount)
	}
	if !entry.Timestamp.Equal(timestamp) {
		t.Fatalf("timestamp = %v, want %v", entry.Timestamp, timestamp)
	}
}
