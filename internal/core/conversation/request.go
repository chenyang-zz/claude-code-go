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
	// TurnTokenBudget sets a target output token budget for the current turn.
	// When positive, the engine will auto-continue the model until the budget
	// is reached or diminishing returns are detected. Zero or negative means
	// no budget tracking.
	TurnTokenBudget int
	// CWD is the current working directory used for hook execution context.
	CWD string
	// PermissionMode is the active permission mode (e.g. "default", "plan").
	PermissionMode string
}
