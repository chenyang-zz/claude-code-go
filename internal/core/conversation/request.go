package conversation

import "github.com/sheepzhao/claude-code-go/internal/core/message"

type RunRequest struct {
	SessionID string
	Input     string
	Messages  []message.Message
}
