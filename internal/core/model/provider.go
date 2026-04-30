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

	// PreviousResponseID enables stateful conversation tracking for the
	// OpenAI Responses API. When set, the server automatically includes
	// the referenced previous turn in the conversation context.
	PreviousResponseID *string

	// Store controls whether the response is stored server-side for later
	// retrieval. When nil the provider default applies (typically true).
	Store *bool

	// ReasoningEffort controls reasoning behaviour for supported models.
	// Accepted values are "low", "medium", and "high".
	ReasoningEffort *string

	// Temperature controls sampling randomness in the range [0, 2].
	// When nil the provider default (1.0) is used.
	Temperature *float64

	// TopP controls nucleus sampling in the range [0, 1].
	// When nil the provider default (1.0) is used.
	TopP *float64

	// ToolChoice controls how the model selects tools.
	// Supported values: "auto", "none", "required", or "function:<name>".
	ToolChoice *string

	// Metadata is a map of custom key-value pairs (max 16 pairs,
	// 128 characters each) attached to the request.
	Metadata map[string]string

	// Instructions is an alternative to System for the OpenAI Responses API.
	// When both are set, Instructions takes precedence for Responses API.
	Instructions *string

	// User is an end-user identifier for monitoring and abuse detection.
	User *string

	// ExtraToolSchemas carries provider-defined server-side tool schemas
	// (e.g. web_search_20250305) that the model executes internally before
	// returning results. Unlike Tools, these schemas are passed through
	// to the provider without client-side tool registration.
	ExtraToolSchemas []map[string]any
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
