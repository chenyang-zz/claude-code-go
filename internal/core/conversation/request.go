package conversation

import "github.com/sheepzhao/claude-code-go/internal/core/message"

// RunRequest describes one runtime execution request.
type RunRequest struct {
	// SessionID carries the caller session identifier used for tracing and future state hand-off.
	SessionID string
	// Input stores the raw user text used when Messages is empty.
	Input string
	// Messages optionally provides a fully constructed conversation history for the request.
	Messages []message.Message
}
