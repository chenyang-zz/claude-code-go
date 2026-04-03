package event

type MessageDeltaPayload struct {
	Text string
}

type ToolCallPayload struct {
	Name string
}

type ErrorPayload struct {
	Message string
}
