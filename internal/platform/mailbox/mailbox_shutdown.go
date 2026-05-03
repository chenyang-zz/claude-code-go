package mailbox

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const teamLeadName = "team-lead"

// generateRequestID creates a deterministic request ID for protocol messages.
// The format is consistent with the TS generateRequestId pattern.
func generateRequestID(prefix, target string) string {
	return fmt.Sprintf("%s-%s-%d", prefix, target, time.Now().UnixMilli())
}

// senderName returns the current agent's display name. Priority:
// CLAUDE_CODE_AGENT_NAME env var, then the default team-lead.
func resolveSenderName() string {
	if name := os.Getenv("CLAUDE_CODE_AGENT_NAME"); name != "" {
		return name
	}
	return teamLeadName
}

// resolveTeammateColor returns the current agent's assigned color, if set.
func resolveTeammateColor() string {
	return os.Getenv("CLAUDE_CODE_AGENT_COLOR")
}

// SendShutdownRequestToMailbox sends a shutdown request to a teammate's
// mailbox. It resolves the sender name from the environment, creates a
// ShutdownRequestMessage, serializes it to JSON, and writes it to the
// target's inbox via WriteToMailbox. Returns the request ID on success.
func SendShutdownRequestToMailbox(targetName, teamName, homeDir, reason string) (string, error) {
	sender := resolveSenderName()
	requestID := generateRequestID("shutdown", targetName)

	msg := CreateShutdownRequestMessage(requestID, sender, reason)
	text, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("mailbox: marshal shutdown request: %w", err)
	}

	color := resolveTeammateColor()
	mailboxMsg := Message{
		From:      sender,
		Text:      string(text),
		Color:     color,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if err := WriteToMailbox(targetName, mailboxMsg, teamName, homeDir); err != nil {
		return "", fmt.Errorf("mailbox: send shutdown request to %s: %w", targetName, err)
	}

	return requestID, nil
}
