package transcript

import (
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// UserEntry stores one user message in the transcript JSONL stream.
type UserEntry struct {
	// Type is the transcript discriminator and is always "user".
	Type string `json:"type"`
	// Timestamp is the write time of the transcript entry.
	Timestamp time.Time `json:"timestamp"`
	// Message is the normalized user message payload.
	Message message.Message `json:"message"`
}

// AssistantEntry stores one assistant message in the transcript JSONL stream.
type AssistantEntry struct {
	// Type is the transcript discriminator and is always "assistant".
	Type string `json:"type"`
	// Timestamp is the write time of the transcript entry.
	Timestamp time.Time `json:"timestamp"`
	// Message is the normalized assistant message payload.
	Message message.Message `json:"message"`
}

// ToolUseEntry stores one tool_use content block extracted from assistant output.
type ToolUseEntry struct {
	// Type is the transcript discriminator and is always "tool_use".
	Type string `json:"type"`
	// Timestamp is the write time of the transcript entry.
	Timestamp time.Time `json:"timestamp"`
	// ToolUseID is the tool invocation identifier emitted by the model.
	ToolUseID string `json:"tool_use_id"`
	// Name is the provider-visible tool name.
	Name string `json:"name"`
	// Input stores the decoded tool arguments.
	Input map[string]any `json:"input,omitempty"`
}

// ToolResultEntry stores one tool_result content block written back to history.
type ToolResultEntry struct {
	// Type is the transcript discriminator and is always "tool_result".
	Type string `json:"type"`
	// Timestamp is the write time of the transcript entry.
	Timestamp time.Time `json:"timestamp"`
	// ToolUseID links this result to the originating tool_use block.
	ToolUseID string `json:"tool_use_id"`
	// Output is the rendered tool output sent back to the model.
	Output string `json:"output"`
	// IsError reports whether the tool result represents a failure.
	IsError bool `json:"is_error"`
}

// SystemEntry stores one system-level transcript record.
type SystemEntry struct {
	// Type is the transcript discriminator and is always "system".
	Type string `json:"type"`
	// Subtype refines the system event type, such as "compact_boundary".
	Subtype string `json:"subtype"`
	// Timestamp is the write time of the transcript entry.
	Timestamp time.Time `json:"timestamp"`
	// Content stores optional human-readable system text.
	Content string `json:"content,omitempty"`
	// CompactMetadata stores compaction details for compact boundary entries.
	CompactMetadata *CompactMetadata `json:"compact_metadata,omitempty"`
}

// CompactMetadata stores compact boundary details in transcript records.
type CompactMetadata struct {
	// Trigger identifies how compaction was triggered (for example "auto").
	Trigger string `json:"trigger,omitempty"`
	// PreTokenCount is the estimated token count before compaction.
	PreTokenCount int `json:"pre_token_count,omitempty"`
	// PostTokenCount is the estimated token count after compaction.
	PostTokenCount int `json:"post_token_count,omitempty"`
}

// SummaryEntry stores one compact summary record in the transcript stream.
type SummaryEntry struct {
	// Type is the transcript discriminator and is always "summary".
	Type string `json:"type"`
	// Timestamp is the write time of the transcript entry.
	Timestamp time.Time `json:"timestamp"`
	// Summary stores the compacted summary text.
	Summary string `json:"summary"`
}

// NewUserEntry builds a user transcript entry from one normalized message.
func NewUserEntry(timestamp time.Time, msg message.Message) UserEntry {
	return UserEntry{
		Type:      "user",
		Timestamp: timestamp,
		Message:   msg,
	}
}

// NewAssistantEntry builds an assistant transcript entry from one normalized message.
func NewAssistantEntry(timestamp time.Time, msg message.Message) AssistantEntry {
	return AssistantEntry{
		Type:      "assistant",
		Timestamp: timestamp,
		Message:   msg,
	}
}

// NewToolUseEntry builds one tool_use transcript entry.
func NewToolUseEntry(timestamp time.Time, id string, name string, input map[string]any) ToolUseEntry {
	return ToolUseEntry{
		Type:      "tool_use",
		Timestamp: timestamp,
		ToolUseID: id,
		Name:      name,
		Input:     input,
	}
}

// NewToolResultEntry builds one tool_result transcript entry.
func NewToolResultEntry(timestamp time.Time, toolUseID string, output string, isError bool) ToolResultEntry {
	return ToolResultEntry{
		Type:      "tool_result",
		Timestamp: timestamp,
		ToolUseID: toolUseID,
		Output:    output,
		IsError:   isError,
	}
}

// NewSystemEntry builds one system transcript entry with an optional content payload.
func NewSystemEntry(timestamp time.Time, subtype string, content string) SystemEntry {
	return SystemEntry{
		Type:      "system",
		Subtype:   subtype,
		Timestamp: timestamp,
		Content:   content,
	}
}

// NewSummaryEntry builds one compact summary transcript entry.
func NewSummaryEntry(timestamp time.Time, summary string) SummaryEntry {
	return SummaryEntry{
		Type:      "summary",
		Timestamp: timestamp,
		Summary:   summary,
	}
}

// NewCompactBoundaryEntry builds one compact boundary system transcript entry.
func NewCompactBoundaryEntry(timestamp time.Time, trigger string, preTokenCount int, postTokenCount int) SystemEntry {
	return SystemEntry{
		Type:      "system",
		Subtype:   "compact_boundary",
		Timestamp: timestamp,
		CompactMetadata: &CompactMetadata{
			Trigger:        trigger,
			PreTokenCount:  preTokenCount,
			PostTokenCount: postTokenCount,
		},
	}
}

// EntriesFromMessage expands one normalized message into transcript entries.
// Besides the top-level user/assistant/system entry, it emits per-block tool_use
// and tool_result entries so downstream readers can index tool execution quickly.
func EntriesFromMessage(timestamp time.Time, msg message.Message) []any {
	entries := make([]any, 0, 1+len(msg.Content))

	switch msg.Role {
	case message.RoleUser:
		entries = append(entries, NewUserEntry(timestamp, msg))
	case message.RoleAssistant:
		entries = append(entries, NewAssistantEntry(timestamp, msg))
	case message.RoleSystem:
		entries = append(entries, NewSystemEntry(timestamp, "system", ""))
	}

	for _, part := range msg.Content {
		switch part.Type {
		case "tool_use":
			entries = append(entries, NewToolUseEntry(timestamp, part.ToolUseID, part.ToolName, part.ToolInput))
		case "tool_result":
			entries = append(entries, NewToolResultEntry(timestamp, part.ToolUseID, part.Text, part.IsError))
		}
	}

	return entries
}
