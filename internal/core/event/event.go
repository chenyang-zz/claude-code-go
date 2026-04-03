package event

import "time"

type Type string

const (
	TypeMessageDelta     Type = "message.delta"
	TypeToolCallStarted  Type = "tool.call.started"
	TypeToolCallFinished Type = "tool.call.finished"
	TypeApprovalRequired Type = "approval.required"
	TypeError            Type = "error"
)

type Event struct {
	Type      Type
	Timestamp time.Time
	Payload   any
}
