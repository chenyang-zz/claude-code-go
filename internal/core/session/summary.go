package session

import (
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

const summaryPreviewLimit = 80

// DerivePreview extracts the minimum text summary used for recent-session discovery output.
func DerivePreview(messages []message.Message) string {
	preview := lastUserText(messages)
	if preview != "" {
		return truncatePreview(preview)
	}

	preview = firstUserText(messages)
	if preview != "" {
		return truncatePreview(preview)
	}
	return ""
}

// lastUserText returns the newest user-authored text content available in one session history.
func lastUserText(messages []message.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != message.RoleUser {
			continue
		}
		if text := messageText(messages[i]); text != "" {
			return text
		}
	}
	return ""
}

// firstUserText returns the earliest user-authored text content as an old-session fallback.
func firstUserText(messages []message.Message) string {
	for _, msg := range messages {
		if msg.Role != message.RoleUser {
			continue
		}
		if text := messageText(msg); text != "" {
			return text
		}
	}
	return ""
}

// messageText joins one message's plain-text parts into one normalized preview candidate.
func messageText(msg message.Message) string {
	parts := make([]string, 0, len(msg.Content))
	for _, part := range msg.Content {
		if part.Type != "text" || part.IsMeta {
			continue
		}
		if text := normalizePreview(part.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

// normalizePreview collapses preview whitespace so persisted summaries stay stable across formatting changes.
func normalizePreview(text string) string {
	fields := strings.Fields(text)
	return strings.Join(fields, " ")
}

// truncatePreview keeps the persisted preview short enough for one-line `/resume` output.
func truncatePreview(text string) string {
	runes := []rune(text)
	if len(runes) <= summaryPreviewLimit {
		return text
	}
	return string(runes[:summaryPreviewLimit-3]) + "..."
}
