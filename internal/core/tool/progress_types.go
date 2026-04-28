package tool

// MCPProgressData carries MCP tool progress details emitted through ReportProgress.
type MCPProgressData struct {
	// Type is the progress subtype discriminator and is always "mcp_progress".
	Type string `json:"type"`
	// Status reports the MCP call phase (for example "started" or "finished").
	Status string `json:"status,omitempty"`
	// ServerName is the MCP server that owns the proxied tool.
	ServerName string `json:"server_name,omitempty"`
	// ToolName is the MCP tool name on the remote server.
	ToolName string `json:"tool_name,omitempty"`
	// ElapsedMs is the elapsed runtime in milliseconds for finish updates.
	ElapsedMs int64 `json:"elapsed_ms,omitempty"`
}

// AgentToolProgressData carries agent tool progress details emitted through ReportProgress.
type AgentToolProgressData struct {
	// Type is the progress subtype discriminator and is always "agent_tool_progress".
	Type string `json:"type"`
	// Status reports the agent lifecycle phase (for example "started" or "finished").
	Status string `json:"status,omitempty"`
	// AgentType is the selected agent kind (for example "explore").
	AgentType string `json:"agent_type,omitempty"`
	// DurationMs is the total agent runtime for finish updates.
	DurationMs int `json:"duration_ms,omitempty"`
	// TotalToolUseCount is the number of tool calls executed by the agent.
	TotalToolUseCount int `json:"total_tool_use_count,omitempty"`
	// TotalTokens is the total token usage observed by the agent run.
	TotalTokens int `json:"total_tokens,omitempty"`
}
