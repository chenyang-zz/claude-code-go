package session

import "github.com/sheepzhao/claude-code-go/internal/core/message"

type Session struct {
	ID       string
	Messages []message.Message
}
