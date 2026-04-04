package event

type MessageDeltaPayload struct {
	Text string
}

type ToolCallPayload struct {
	ID    string
	Name  string
	Input map[string]any
}

type ErrorPayload struct {
	Message string
}
