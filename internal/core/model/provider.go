package model

import "github.com/sheepzhao/claude-code-go/internal/core/message"

// Request describes the minimum single-turn model request supported by batch-07.
type Request struct {
	Model    string
	System   string
	Messages []message.Message
}

// ToolUse keeps the existing tool-use shape available for later engine expansion.
type ToolUse struct {
	ID    string
	Name  string
	Input map[string]any
}
