package event

import "time"

type Type string

const (
	TypeMessageDelta     Type = "message.delta"
	TypeThinking         Type = "thinking"
	TypeToolCallStarted  Type = "tool.call.started"
	TypeToolCallFinished Type = "tool.call.finished"
	TypeApprovalRequired Type = "approval.required"
	TypeConversationDone Type = "conversation.done"
	TypeError            Type = "error"
	TypeUsage            Type = "usage"
	TypeRetryAttempted   Type = "retry.attempted"
	TypeModelFallback    Type = "model.fallback"
	TypeCompactDone      Type = "compact.done"
	TypeProgress         Type = "tool.progress"
)

type Event struct {
	Type      Type      `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload,omitempty"`
}
