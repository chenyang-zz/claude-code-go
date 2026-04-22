package message

// ContentPart is the minimal provider-agnostic content block carried in one conversation message.
type ContentPart struct {
	// Type identifies the normalized content variant such as text, tool_use, or tool_result.
	Type string `json:"type"`
	// Text stores the plain text payload for text blocks and the rendered content for tool_result blocks.
	Text string `json:"text,omitempty"`
	// IsMeta reports whether a text block carries runtime metadata rather than a natural-language user turn.
	IsMeta bool `json:"is_meta,omitempty"`
	// ToolUseID stores the correlation identifier shared by tool_use and tool_result blocks.
	ToolUseID string `json:"tool_use_id,omitempty"`
	// ToolName stores the provider-visible tool name for tool_use blocks.
	ToolName string `json:"tool_name,omitempty"`
	// ToolInput stores the decoded JSON arguments for tool_use blocks.
	ToolInput map[string]any `json:"tool_input,omitempty"`
	// IsError reports whether a tool_result block represents an errored tool execution.
	IsError bool `json:"is_error,omitempty"`
}

// TextPart builds one normalized text content block.
func TextPart(text string) ContentPart {
	return ContentPart{
		Type: "text",
		Text: text,
	}
}

// MetaTextPart builds one text content block that should be delivered to the model
// but ignored when deriving natural-language user turn semantics.
func MetaTextPart(text string) ContentPart {
	return ContentPart{
		Type:   "text",
		Text:   text,
		IsMeta: true,
	}
}

// ToolUsePart builds one normalized assistant tool_use content block.
func ToolUsePart(id, name string, input map[string]any) ContentPart {
	return ContentPart{
		Type:      "tool_use",
		ToolUseID: id,
		ToolName:  name,
		ToolInput: input,
	}
}

// ToolResultPart builds one normalized user tool_result content block.
func ToolResultPart(toolUseID, content string, isError bool) ContentPart {
	return ContentPart{
		Type:      "tool_result",
		Text:      content,
		ToolUseID: toolUseID,
		IsError:   isError,
	}
}
