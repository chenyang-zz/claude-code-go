package model

// EventType identifies the supported provider stream event variants.
type EventType string

const (
	// EventTypeTextDelta carries one assistant text chunk.
	EventTypeTextDelta EventType = "text_delta"
	// EventTypeThinking carries one complete assistant thinking block.
	EventTypeThinking EventType = "thinking"
	// EventTypeToolUse carries one assistant tool-use block after its JSON input is complete.
	EventTypeToolUse EventType = "tool_use"
	// EventTypeError carries a provider-facing failure converted into stream form.
	EventTypeError EventType = "error"
	// EventTypeDone marks the end of a provider stream.
	EventTypeDone EventType = "done"
)

// StopReason describes why the model stopped generating tokens.
type StopReason string

const (
	// StopReasonEndTurn indicates the model finished its response naturally.
	StopReasonEndTurn StopReason = "end_turn"
	// StopReasonMaxTokens indicates the model hit the maximum output token limit.
	StopReasonMaxTokens StopReason = "max_tokens"
	// StopReasonToolUse indicates the model stopped to request tool execution.
	StopReasonToolUse StopReason = "tool_use"
	// StopReasonStopSequence indicates the model hit a configured stop sequence.
	StopReasonStopSequence StopReason = "stop_sequence"
)

// Event carries one provider stream item in a provider-agnostic form.
type Event struct {
	Type       EventType
	Text       string
	Thinking   string
	Signature  string
	Error      string
	ToolUse    *ToolUse
	StopReason StopReason
	Usage      *Usage
}

// Stream is the asynchronous event channel returned by a model client.
type Stream <-chan Event
