package transcript

import (
	"errors"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// RecoverResult holds the reconstructed conversation state extracted from a
// transcript file.
type RecoverResult struct {
	// Messages is the conversation history reconstructed in chronological order.
	Messages []message.Message
	// Summaries collects all compact-summary entries found in the transcript.
	Summaries []SummaryEntry
	// CompactBoundaries records the positions and metadata of compaction events.
	CompactBoundaries []CompactBoundary
}

// CompactBoundary records one compaction event found while scanning a transcript.
type CompactBoundary struct {
	// MessageIndex is the index in Messages just before the boundary
	// (i.e. the boundary sits between Messages[MessageIndex-1] and
	// Messages[MessageIndex]).
	MessageIndex int
	// Trigger identifies how compaction was triggered (for example "auto").
	Trigger string
	// PreTokenCount is the estimated token count before compaction.
	PreTokenCount int
	// PostTokenCount is the estimated token count after compaction.
	PostTokenCount int
}

// RecoverFile opens a transcript file, reads all entries, and reconstructs
// the conversation.  Malformed lines are skipped with a warning.
func RecoverFile(path string) (RecoverResult, error) {
	reader, err := NewReader(path)
	if err != nil {
		return RecoverResult{}, fmt.Errorf("failed to open transcript for recovery: %w", err)
	}
	defer func() {
		if cerr := reader.Close(); cerr != nil {
			logger.WarnCF("transcript", "failed to close transcript reader", map[string]any{
				"path":  path,
				"error": cerr.Error(),
			})
		}
	}()

	entries, err := reader.ReadAll()
	if err != nil {
		return RecoverResult{}, fmt.Errorf("failed to read transcript entries: %w", err)
	}

	logger.DebugCF("transcript", "recovered entries from file", map[string]any{
		"path":         path,
		"entry_count":  len(entries),
	})

	return RecoverEntries(entries), nil
}

// RecoverEntries reconstructs conversation messages and metadata from a slice
// of transcript entries (as returned by Reader.ReadAll).
//
// The reconstruction strategy mirrors the inverse of EntriesFromMessage:
//   - UserEntry and AssistantEntry carry the full message.Message, so they
//     become the primary source for Messages.
//   - ToolUseEntry and ToolResultEntry are redundant index entries written for
//     fast downstream lookup; they are skipped here because the same data is
//     already present in the preceding AssistantEntry / UserEntry Content.
//   - SummaryEntry is collected separately.
//   - SystemEntry with subtype "compact_boundary" records a compaction point.
func RecoverEntries(entries []any) RecoverResult {
	var result RecoverResult
	var pending *message.Message

	flushPending := func() {
		if pending != nil {
			result.Messages = append(result.Messages, *pending)
			pending = nil
		}
	}

	for _, raw := range entries {
		if err := validateEntry(raw); err != nil {
			logger.WarnCF("transcript", "skipping invalid entry during recovery", map[string]any{
				"error": err.Error(),
			})
			continue
		}

		switch entry := raw.(type) {
		case UserEntry:
			flushPending()
			m := entry.Message
			pending = &m
		case AssistantEntry:
			flushPending()
			m := entry.Message
			pending = &m
		case SystemEntry:
			flushPending()
			if entry.Subtype == "compact_boundary" && entry.CompactMetadata != nil {
				result.CompactBoundaries = append(result.CompactBoundaries, CompactBoundary{
					MessageIndex:   len(result.Messages),
					Trigger:        entry.CompactMetadata.Trigger,
					PreTokenCount:  entry.CompactMetadata.PreTokenCount,
					PostTokenCount: entry.CompactMetadata.PostTokenCount,
				})
			}
		case SummaryEntry:
			flushPending()
			result.Summaries = append(result.Summaries, entry)
		case ToolUseEntry, ToolResultEntry:
			// Redundant index entries — the same content is already inside
			// the AssistantEntry / UserEntry Message.Content.
			continue
		default:
			// Unknown entry types are silently skipped so that future
			// transcript extensions do not break recovery.
			continue
		}
	}

	flushPending()

	logger.DebugCF("transcript", "recovered conversation", map[string]any{
		"message_count":  len(result.Messages),
		"summary_count":  len(result.Summaries),
		"boundary_count": len(result.CompactBoundaries),
	})

	return result
}

// validateEntry checks that a concrete entry carries the minimum required
// fields for recovery.  It returns a descriptive error when validation fails.
func validateEntry(entry any) error {
	switch e := entry.(type) {
	case UserEntry:
		if e.Type != "user" {
			return errors.New("user entry has wrong type discriminator")
		}
		if e.Message.Role != message.RoleUser {
			return errors.New("user entry message has wrong role")
		}
	case AssistantEntry:
		if e.Type != "assistant" {
			return errors.New("assistant entry has wrong type discriminator")
		}
		if e.Message.Role != message.RoleAssistant {
			return errors.New("assistant entry message has wrong role")
		}
	case ToolUseEntry:
		if e.Type != "tool_use" {
			return errors.New("tool_use entry has wrong type discriminator")
		}
		if e.ToolUseID == "" {
			return errors.New("tool_use entry missing tool_use_id")
		}
	case ToolResultEntry:
		if e.Type != "tool_result" {
			return errors.New("tool_result entry has wrong type discriminator")
		}
		if e.ToolUseID == "" {
			return errors.New("tool_result entry missing tool_use_id")
		}
	case SystemEntry:
		if e.Type != "system" {
			return errors.New("system entry has wrong type discriminator")
		}
		if e.Subtype == "" {
			return errors.New("system entry missing subtype")
		}
	case SummaryEntry:
		if e.Type != "summary" {
			return errors.New("summary entry has wrong type discriminator")
		}
		if e.Summary == "" {
			return errors.New("summary entry missing summary text")
		}
	default:
		return errors.New("unknown entry type")
	}
	return nil
}
