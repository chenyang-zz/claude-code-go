package event

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
	ID      string
	Name    string
	Output  string
	IsError bool
}

// ErrorPayload carries one runtime or provider error message.
type ErrorPayload struct {
	Message string
}
