package model

// EventType identifies the supported provider stream event variants.
type EventType string

const (
	// EventTypeTextDelta carries one assistant text chunk.
	EventTypeTextDelta EventType = "text_delta"
	// EventTypeError carries a provider-facing failure converted into stream form.
	EventTypeError EventType = "error"
	// EventTypeDone marks the end of a provider stream.
	EventTypeDone EventType = "done"
)

// Event carries one provider stream item in a provider-agnostic form.
type Event struct {
	Type    EventType
	Text    string
	Error   string
	ToolUse *ToolUse
}

// Stream is the asynchronous event channel returned by a model client.
type Stream <-chan Event
