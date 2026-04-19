package event

import "time"

type Type string

const (
	TypeMessageDelta     Type = "message.delta"
	TypeToolCallStarted  Type = "tool.call.started"
	TypeToolCallFinished Type = "tool.call.finished"
	TypeApprovalRequired Type = "approval.required"
	TypeConversationDone Type = "conversation.done"
	TypeError            Type = "error"
	TypeUsage            Type = "usage"
	TypeRetryAttempted   Type = "retry.attempted"
	TypeModelFallback    Type = "model.fallback"
	TypeCompactDone      Type = "compact.done"
)

type Event struct {
	Type      Type
	Timestamp time.Time
	Payload   any
}
