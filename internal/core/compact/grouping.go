package compact

import (
	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// MessageWithID wraps a message.Message with an ID field that identifies
// the API response round. Streaming chunks from the same API response
// share the same ID, so boundaries only fire at the start of a genuinely
// new round.
type MessageWithID struct {
	Message message.Message
	// MessageID is the unique identifier for the assistant's API response.
	// Streaming chunks from the same API response share this ID.
	MessageID string
}

// GroupMessagesByApiRound groups messages at API-round boundaries: one group
// per API round-trip. A boundary fires when a NEW assistant response begins
// (different MessageID from the prior assistant). For well-formed conversations
// this is an API-safe split point — the API contract requires every tool_use
// to be resolved before the next assistant turn, so pairing validity falls
// out of the assistant-id boundary.
//
// Replaces the prior human-turn grouping (boundaries only at real user
// prompts) with finer-grained API-round grouping, allowing reactive compact
// to operate on single-prompt agentic sessions (SDK/CCR/eval callers) where
// the entire workload is one human turn.
//
// Aligns with TS groupMessagesByApiRound (grouping.ts:22-63).
func GroupMessagesByApiRound(messages []MessageWithID) [][]MessageWithID {
	var groups [][]MessageWithID
	var current []MessageWithID
	var lastAssistantId string

	for _, msg := range messages {
		if msg.Message.Role == message.RoleAssistant &&
			msg.MessageID != lastAssistantId &&
			len(current) > 0 {
			groups = append(groups, current)
			current = []MessageWithID{msg}
		} else {
			current = append(current, msg)
		}
		if msg.Message.Role == message.RoleAssistant {
			lastAssistantId = msg.MessageID
		}
	}

	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}
