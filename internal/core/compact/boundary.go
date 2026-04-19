package compact

import (
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// BoundaryTrigger identifies how a compaction was initiated.
type BoundaryTrigger string

const (
	TriggerAuto   BoundaryTrigger = "auto"
	TriggerManual BoundaryTrigger = "manual"
)

// CompactMetadata stores metadata about a compaction event, attached to
// the boundary marker message. Aligns with TS compactMetadata.
type CompactMetadata struct {
	// Trigger indicates whether compaction was automatic or manual.
	Trigger BoundaryTrigger
	// PreTokens is the estimated token count before compaction.
	PreTokens int
	// MessagesSummarized counts how many messages were replaced.
	MessagesSummarized int
}

// IsCompactBoundary checks whether a message is a compact boundary marker.
// New boundaries use RoleUser so they can be sent to Anthropic unchanged,
// but legacy RoleSystem boundaries remain recognized for compatibility.
// Aligns with TS isCompactBoundaryMessage.
func IsCompactBoundary(msg *message.Message) bool {
	if msg == nil {
		return false
	}
	if msg.Role != message.RoleUser && msg.Role != message.RoleSystem {
		return false
	}
	for _, part := range msg.Content {
		if part.Type == "text" && part.Text == "[compact_boundary]" {
			return true
		}
	}
	return false
}

// CreateBoundaryMessage builds a compact boundary marker message inserted
// at the top of the post-compact message sequence. The boundary serves as
// a marker so the runtime can identify where compaction occurred.
// Aligns with TS createCompactBoundaryMessage.
func CreateBoundaryMessage(trigger BoundaryTrigger, preTokens int, messagesSummarized int) message.Message {
	return message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart("[compact_boundary]"),
			message.TextPart(string(trigger)),
		},
	}
}

// CreateSummaryMessage wraps the compact summary text into a user message
// that replaces the pre-compact conversation history.
// Aligns with TS getCompactUserSummaryMessage.
func CreateSummaryMessage(summary string, transcriptPath string) message.Message {
	text := "This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.\n\n"
	text += summary
	if transcriptPath != "" {
		text += "\n\nIf you need specific details from before compaction (like exact code snippets, error messages, or content you generated), read the full transcript at: " + transcriptPath
	}
	return message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart(text),
		},
	}
}

// FindLastCompactBoundary scans messages in reverse and returns the index
// of the most recent compact boundary marker. Returns -1 if none found.
// Aligns with TS findLastCompactBoundaryIndex.
func FindLastCompactBoundary(messages []message.Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if IsCompactBoundary(&messages[i]) {
			return i
		}
	}
	return -1
}

// GetMessagesAfterCompactBoundary returns messages from the last compact
// boundary onward (including the boundary). If no boundary exists, returns
// all messages. Aligns with TS getMessagesAfterCompactBoundary.
func GetMessagesAfterCompactBoundary(messages []message.Message) []message.Message {
	idx := FindLastCompactBoundary(messages)
	if idx == -1 {
		return messages
	}
	return messages[idx:]
}

// BuildPostCompactMessages assembles the final message list after compaction:
// boundary marker + summary message(s). This replaces the original
// compacted history while allowing the active turn tail to remain intact.
func BuildPostCompactMessages(boundary message.Message, messages ...message.Message) []message.Message {
	result := make([]message.Message, 0, 1+len(messages))
	result = append(result, boundary)
	result = append(result, messages...)
	return result
}

// FormatCompactTime returns an ISO-8601 timestamp for the compaction event.
func FormatCompactTime() string {
	return time.Now().UTC().Format(time.RFC3339)
}
