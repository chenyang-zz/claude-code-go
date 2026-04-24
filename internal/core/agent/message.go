package agent

import "time"

// Message represents an inter-agent communication payload.
type Message struct {
	// From is the sender agent identifier.
	From string
	// To is the recipient agent identifier.
	To string
	// Content is the message body.
	Content string
	// Timestamp records when the message was created.
	Timestamp time.Time
}
