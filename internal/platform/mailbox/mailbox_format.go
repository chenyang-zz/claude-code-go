package mailbox

import "strings"

// TeammateMessageTag is the XML tag name used to wrap teammate messages in
// attachment displays, matching the TS TEAMMATE_MESSAGE_TAG constant.
const TeammateMessageTag = "teammate-message"

// FormatTeammateMessages formats a slice of mailbox messages as XML string
// suitable for attachment display. Each message is wrapped in a
// <teammate-message> element with teammate_id, optional color, and optional
// summary attributes. Multiple messages are joined with double newlines.
func FormatTeammateMessages(messages []Message) string {
	parts := make([]string, 0, len(messages))
	for _, m := range messages {
		var sb strings.Builder
		sb.WriteString("<")
		sb.WriteString(TeammateMessageTag)
		sb.WriteString(` teammate_id="`)
		sb.WriteString(m.From)
		sb.WriteString(`"`)
		if m.Color != "" {
			sb.WriteString(` color="`)
			sb.WriteString(m.Color)
			sb.WriteString(`"`)
		}
		if m.Summary != "" {
			sb.WriteString(` summary="`)
			sb.WriteString(m.Summary)
			sb.WriteString(`"`)
		}
		sb.WriteString(">\n")
		sb.WriteString(m.Text)
		sb.WriteString("\n</")
		sb.WriteString(TeammateMessageTag)
		sb.WriteString(">")
		parts = append(parts, sb.String())
	}
	return strings.Join(parts, "\n\n")
}

// FormatTeammateMessage formats a single message as an XML element.
// Convenience wrapper around FormatTeammateMessages for single-message use.
func FormatTeammateMessage(from, text, color, summary string) string {
	return FormatTeammateMessages([]Message{
		{From: from, Text: text, Color: color, Summary: summary},
	})
}
