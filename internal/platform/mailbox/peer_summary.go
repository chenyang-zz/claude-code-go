package mailbox

import (
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// sendMessageToolName is the tool name used for SendMessage tool calls.
const sendMessageToolName = "SendMessage"

// GetLastPeerDmSummary scans messages in reverse order and returns a
// "[to {name}] {summary}" string for the last assistant message that ended
// with a SendMessage tool_use targeting a peer (not broadcast and not the
// team lead). Returns empty string when no such message is found.
//
// The scan stops at the first user message with string content (wake-up
// boundary) so it only considers the current turn.
func GetLastPeerDmSummary(messages []message.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]

		// Stop at wake-up boundary: a user message with non-meta text content,
		// not tool results (which have type="tool_result").
		if msg.Role == message.RoleUser {
			hasUserText := false
			for _, block := range msg.Content {
				if block.Type == "text" && !block.IsMeta {
					hasUserText = true
					break
				}
			}
			if hasUserText {
				break
			}
		}

		if msg.Role != message.RoleAssistant {
			continue
		}

		for _, block := range msg.Content {
			if block.Type != "tool_use" {
				continue
			}
			if block.ToolName != sendMessageToolName {
				continue
			}
			if block.ToolInput == nil {
				continue
			}

			to, _ := block.ToolInput["to"].(string)
			if to == "" {
				continue
			}
			// Skip broadcast messages
			if to == "*" {
				continue
			}
			// Skip messages to team lead (main channel)
			if strings.EqualFold(to, teamLeadName) {
				continue
			}

			teammateMsg, _ := block.ToolInput["message"].(string)

			summary, hasSummary := block.ToolInput["summary"].(string)
			if !hasSummary || summary == "" {
				if len(teammateMsg) > 80 {
					summary = teammateMsg[:80]
				} else {
					summary = teammateMsg
				}
			}

			return fmt.Sprintf("[to %s] %s", to, summary)
		}
	}
	return ""
}
