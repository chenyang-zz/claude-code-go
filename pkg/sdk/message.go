package sdk

// StreamEvent represents a streaming event message, emitted for each
// assistant text delta (corresponds to TypeMessageDelta).
type StreamEvent struct {
	Base
	Event           any     `json:"event"`
	ParentToolUseID *string `json:"parent_tool_use_id"`
}

func (StreamEvent) isSDKMessage() {}

// Assistant represents an assistant message in the conversation.
type Assistant struct {
	Base
	Message         any     `json:"message"`
	ParentToolUseID *string `json:"parent_tool_use_id"`
	Error           *string `json:"error,omitempty"`
}

func (Assistant) isSDKMessage() {}

// User represents a user message in the conversation.
type User struct {
	Base
	Message         any     `json:"message"`
	ParentToolUseID *string `json:"parent_tool_use_id"`
	IsSynthetic     bool    `json:"is_synthetic,omitempty"`
	ToolUseResult   any     `json:"tool_use_result,omitempty"`
	Priority        string  `json:"priority,omitempty"`
	Timestamp       string  `json:"timestamp,omitempty"`
}

func (User) isSDKMessage() {}

// Result represents the final outcome of a conversation turn.
// Subtype discriminates between "success" and error variants.
type Result struct {
	Base
	Subtype           string         `json:"subtype"`
	DurationMs        int64          `json:"duration_ms"`
	DurationApiMs     int64          `json:"duration_api_ms"`
	IsError           bool           `json:"is_error"`
	NumTurns          int            `json:"num_turns"`
	Result            string         `json:"result,omitempty"`
	StopReason        *string        `json:"stop_reason"`
	TotalCostUSD      float64        `json:"total_cost_usd"`
	Usage             any            `json:"usage"`
	ModelUsage        map[string]any `json:"model_usage"`
	PermissionDenials []any          `json:"permission_denials"`
	Errors            []string       `json:"errors,omitempty"`
	StructuredOutput  any            `json:"structured_output,omitempty"`
}

func (Result) isSDKMessage() {}

// SystemInit represents the system initialization message.
type SystemInit struct {
	Base
	Subtype           string      `json:"subtype"`
	Agents            []string    `json:"agents,omitempty"`
	APIKeySource      string      `json:"api_key_source"`
	Betas             []string    `json:"betas,omitempty"`
	ClaudeCodeVersion string      `json:"claude_code_version"`
	CWD               string      `json:"cwd"`
	Tools             []string    `json:"tools"`
	MCPServers        []MCPServer `json:"mcp_servers"`
	Model             string      `json:"model"`
	PermissionMode    string      `json:"permission_mode"`
	SlashCommands     []string    `json:"slash_commands"`
	OutputStyle       string      `json:"output_style"`
	Skills            []string    `json:"skills"`
	Plugins           []Plugin    `json:"plugins"`
}

func (SystemInit) isSDKMessage() {}

// ToolProgress represents a tool execution progress update.
type ToolProgress struct {
	Base
	ToolUseID       string  `json:"tool_use_id"`
	ToolName        string  `json:"tool_name"`
	ParentToolUseID *string `json:"parent_tool_use_id"`
	ElapsedTimeSec  float64 `json:"elapsed_time_seconds"`
	TaskID          string  `json:"task_id,omitempty"`
	Progress        any     `json:"progress,omitempty"`
}

func (ToolProgress) isSDKMessage() {}
