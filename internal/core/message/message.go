package message

// Message is the minimal provider-agnostic conversation item exchanged with the runtime.
type Message struct {
	// Role identifies who authored the message in the normalized conversation history.
	Role Role
	// Content stores the ordered content blocks associated with the message.
	Content []ContentPart
}
