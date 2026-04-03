package model

import "github.com/sheepzhao/claude-code-go/internal/core/message"

type Request struct {
	Model    string
	System   string
	Messages []message.Message
}

type ToolUse struct {
	ID    string
	Name  string
	Input map[string]any
}
