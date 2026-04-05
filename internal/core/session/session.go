package session

import (
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// Session stores the minimum persisted conversation state required for resume.
type Session struct {
	// ID identifies one logical CLI conversation.
	ID string
	// ProjectPath identifies the workspace path this session belongs to.
	ProjectPath string
	// Messages stores the normalized conversation history that should be restored on resume.
	Messages []message.Message
	// UpdatedAt records when the session snapshot was last overwritten.
	UpdatedAt time.Time
}

// Clone returns a detached copy of the session so callers can safely mutate it.
func (s Session) Clone() Session {
	cloned := make([]message.Message, len(s.Messages))
	copy(cloned, s.Messages)

	return Session{
		ID:          s.ID,
		ProjectPath: s.ProjectPath,
		Messages:    cloned,
		UpdatedAt:   s.UpdatedAt,
	}
}
