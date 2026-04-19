package model

import "github.com/sheepzhao/claude-code-go/internal/core/message"

// Request describes the minimum model request supported by the migrated runtime.
type Request struct {
	Model  string
	System string
	// MaxOutputTokens optionally overrides the provider default output cap.
	// Zero means "use the client's default".
	MaxOutputTokens int
	Messages        []message.Message
	Tools           []ToolDefinition
}

// ToolUse keeps the existing tool-use shape available for later engine expansion.
type ToolUse struct {
	ID    string
	Name  string
	Input map[string]any
}

// ToolDefinition carries the minimal provider-agnostic tool declaration attached to a model request.
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]any
}
