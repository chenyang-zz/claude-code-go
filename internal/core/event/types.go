package event

import (
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// MessageDeltaPayload carries one assistant text chunk rendered to the caller.
type MessageDeltaPayload struct {
	Text string
}

// ToolCallPayload describes one tool_use event surfaced to the runtime caller.
type ToolCallPayload struct {
	ID    string
	Name  string
	Input map[string]any
}

// ToolResultPayload describes one completed tool execution inside the runtime loop.
type ToolResultPayload struct {
	ID                string
	Name              string
	Output            string
	AdditionalContext string
	IsError           bool
}

// ApprovalPayload describes one runtime approval request emitted before a guarded tool operation can continue.
type ApprovalPayload struct {
	CallID   string
	ToolName string
	Path     string
	Action   string
	Message  string
}

// ErrorPayload carries one runtime or provider error message.
type ErrorPayload struct {
	Message string
}

// ConversationDonePayload carries the final normalized history produced by one runtime turn.
type ConversationDonePayload struct {
	History conversation.History
	Usage   model.Usage
}

// UsagePayload carries per-turn and cumulative token usage metrics.
type UsagePayload struct {
	TurnUsage       model.Usage
	CumulativeUsage model.Usage
	StopReason      string
}

// RetryAttemptedPayload carries information about one retry attempt.
type RetryAttemptedPayload struct {
	Attempt     int
	MaxAttempts int
	BackoffMs   int64
	Error       string
}

// ModelFallbackPayload carries information about a model fallback switch.
type ModelFallbackPayload struct {
	OriginalModel string
	FallbackModel string
}

// CompactDonePayload carries information about a completed auto-compaction.
type CompactDonePayload struct {
	PreTokenCount  int
	PostTokenCount int
}
