package agent

// Input holds the parameters for an Agent tool invocation.
// It mirrors the TypeScript AgentToolInput type.
type Input struct {
	// Description is a short (3-5 word) description of the task.
	Description string `json:"description"`
	// Prompt is the task for the agent to perform.
	Prompt string `json:"prompt"`
	// SubagentType is the type of specialized agent to use.
	SubagentType string `json:"subagent_type,omitempty"`
	// Model is an optional model override.
	Model string `json:"model,omitempty"`
	// RunInBackground indicates the agent should run in the background.
	RunInBackground bool `json:"run_in_background,omitempty"`
	// Name makes the spawned agent addressable via SendMessage.
	Name string `json:"name,omitempty"`
	// TeamName is the team name for spawning.
	TeamName string `json:"team_name,omitempty"`
	// Mode is the permission mode for the spawned agent.
	Mode string `json:"mode,omitempty"`
	// Isolation is the isolation mode: "worktree" or "remote".
	Isolation string `json:"isolation,omitempty"`
	// Cwd overrides the working directory for the agent.
	Cwd string `json:"cwd,omitempty"`
}

// TextBlock is a single text content block in the agent result.
type TextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// UsageStats holds token usage metrics for the agent run.
type UsageStats struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// Output holds the result of an Agent tool invocation.
// It mirrors the TypeScript AgentToolResult type.
type Output struct {
	// AgentID is the unique identifier of the spawned agent.
	AgentID string `json:"agentId"`
	// AgentType is the type of the spawned agent.
	AgentType string `json:"agentType,omitempty"`
	// Content is the textual result from the agent.
	Content []TextBlock `json:"content"`
	// TotalToolUseCount is the total number of tool uses during the run.
	TotalToolUseCount int `json:"totalToolUseCount"`
	// TotalDurationMs is the wall-clock duration of the agent run.
	TotalDurationMs int `json:"totalDurationMs"`
	// TotalTokens is the total number of tokens consumed.
	TotalTokens int `json:"totalTokens"`
	// Usage holds detailed token usage metrics.
	Usage UsageStats `json:"usage"`
}
