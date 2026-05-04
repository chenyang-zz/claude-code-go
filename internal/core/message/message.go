package message

// Message is the minimal provider-agnostic conversation item exchanged with the runtime.
type Message struct {
	// Role identifies who authored the message in the normalized conversation history.
	Role Role `json:"role"`
	// Content stores the ordered content blocks associated with the message.
	Content []ContentPart `json:"content"`
	// Timestamp is an optional ISO 8601 timestamp for the message, used by
	// time-based microcompact to compute the gap since the last assistant turn.
	Timestamp string `json:"timestamp,omitempty"`
}
