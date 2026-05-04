package microcompact

// TimeBasedTriggerResult holds the result of a successful time-based trigger evaluation.
type TimeBasedTriggerResult struct {
	// GapMinutes is the measured gap since the last assistant message.
	GapMinutes float64
	// Config is the active time-based MC config used for the trigger.
	Config TimeBasedMCConfig
}

// MicrocompactResult holds the output of a microcompact operation.
// Aligns with TS MicrocompactResult.
type MicrocompactResult struct {
	// Messages is the (possibly compacted) message list.
	Messages []Message
}

// Message is the minimal message interface consumed by the microcompact service.
// It mirrors the fields from core/message.Message that microcompact needs,
// avoiding a direct dependency on the core package.
type Message struct {
	// Type identifies the message role: "user", "assistant", or "system".
	Type string
	// Content holds the content parts (text, tool_result, tool_use, etc.).
	Content []ContentPart
	// Timestamp is an optional ISO 8601 timestamp for the message.
	Timestamp string
}

// ContentPart is a single content block within a message.
type ContentPart struct {
	// Type identifies the content block type: "text", "tool_result", "tool_use", etc.
	Type string `json:"type"`
	// Text holds the text content (for text and tool_result blocks).
	Text string `json:"text,omitempty"`
	// ToolUseID is the correlation ID shared by tool_use and tool_result blocks.
	ToolUseID string `json:"tool_use_id,omitempty"`
	// ToolName is the tool identifier for tool_use blocks.
	ToolName string `json:"tool_name,omitempty"`
}

// CompactableToolSet defines the tool names whose results may be compacted.
// Aligns with TS COMPACTABLE_TOOLS (src/services/compact/microCompact.ts:41-50).
var CompactableToolSet = map[string]bool{
	"Bash":         true,
	"Glob":         true,
	"Grep":         true,
	"FileRead":     true,
	"FileWrite":    true,
	"FileEdit":     true,
	"WebFetch":     true,
	"WebSearch":    true,
	"NotebookEdit": true,
}
