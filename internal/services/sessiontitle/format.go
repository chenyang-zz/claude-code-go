package sessiontitle

import (
	"strings"
	"unicode/utf8"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// maxConversationText caps the extracted conversation text at 1000
// characters, mirroring the TS MAX_CONVERSATION_TEXT constant.
const maxConversationText = 1000

// ExtractConversationText flattens a message array into a single text
// string for Haiku title input. Skips meta content blocks and extracts
// only text parts. Tail-slices to the last 1000 characters so recent
// context wins when the conversation is long.
//
// Mirrors src/utils/sessionTitle.ts extractConversationText.
func ExtractConversationText(messages []message.Message) string {
	parts := make([]string, 0, len(messages)*2)
	for _, msg := range messages {
		if msg.Role != message.RoleUser && msg.Role != message.RoleAssistant {
			continue
		}
		for _, part := range msg.Content {
			if part.Type != "text" {
				continue
			}
			if part.IsMeta {
				continue
			}
			if part.Text != "" {
				parts = append(parts, part.Text)
			}
		}
	}
	text := strings.Join(parts, "\n")
	if utf8.RuneCountInString(text) > maxConversationText {
		runes := []rune(text)
		return string(runes[len(runes)-maxConversationText:])
	}
	return text
}
