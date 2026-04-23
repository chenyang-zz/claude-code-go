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
	// TaskBudget specifies an API-side token budget for the model to pace itself.
	// When set, output_config.task_budget is sent to the API with the
	// task-budgets-2026-03-13 beta header. Only included for first-party
	// Anthropic requests.
	TaskBudget *TaskBudgetParam
	// EnablePromptCaching tells the Anthropic client to place a cache_control
	// marker on the last content block of the last message in the request.
	// When false (or when the provider is not Anthropic) the marker is omitted.
	EnablePromptCaching bool
}

// TaskBudgetParam represents the output_config.task_budget wire format sent
// to the Anthropic API so the model can pace itself against a token budget.
type TaskBudgetParam struct {
	// Type is always "tokens".
	Type string
	// Total is the total token budget for the agentic turn.
	Total int
	// Remaining is the remaining budget, set after the first compaction so
	// the server can correctly account for pre-compact context that was
	// summarized away. Nil means "not computed yet".
	Remaining *int
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
