package conversation

import "github.com/sheepzhao/claude-code-go/internal/core/message"

// History stores the normalized message sequence sent back to the model across one runtime turn.
type History struct {
	// Messages keeps the ordered conversation items accumulated so far.
	Messages []message.Message
}

// Clone returns a detached copy of the conversation history.
func (h History) Clone() History {
	cloned := make([]message.Message, len(h.Messages))
	copy(cloned, h.Messages)
	return History{Messages: cloned}
}

// Append pushes one message to the tail of the conversation history.
func (h *History) Append(msg message.Message) {
	if h == nil {
		return
	}
	h.Messages = append(h.Messages, msg)
}
