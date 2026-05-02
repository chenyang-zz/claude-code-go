// Package betas provides the Anthropic beta header composition engine.
package betas

// Beta header constants used by the Anthropic API.
// These mirror the constants defined in src/constants/betas.ts.
const (
	// ClaudeCodeBeta is the general Claude Code beta header applied to
	// non-Haiku models by default.
	ClaudeCodeBeta = "claude-code-20250219"

	// InterleavedThinkingBeta enables interleaved thinking support.
	InterleavedThinkingBeta = "interleaved-thinking-2025-05-14"

	// Context1MBeta enables the 1M context window.
	Context1MBeta = "context-1m-2025-08-07"

	// ContextManagementBeta enables tool clearing and thinking preservation.
	ContextManagementBeta = "context-management-2025-06-27"

	// StructuredOutputsBeta enables structured output (strict tools).
	StructuredOutputsBeta = "structured-outputs-2025-12-15"

	// WebSearchBeta enables web search (Vertex/Foundry only).
	WebSearchBeta = "web-search-2025-03-05"

	// ToolSearch1PBeta is the tool search header for first-party and Foundry.
	ToolSearch1PBeta = "advanced-tool-use-2025-11-20"

	// ToolSearch3PBeta is the tool search header for Vertex and Bedrock.
	ToolSearch3PBeta = "tool-search-tool-2025-10-19"

	// EffortBeta enables effort control.
	EffortBeta = "effort-2025-11-24"

	// TaskBudgetsBeta enables the task_budget feature.
	// Already hardcoded in client.go; will be migrated to composer.
	TaskBudgetsBeta = "task-budgets-2026-03-13"

	// PromptCachingScopeBeta enables global prompt caching scope.
	PromptCachingScopeBeta = "prompt-caching-scope-2026-01-05"

	// FastModeBeta enables fast mode.
	FastModeBeta = "fast-mode-2026-02-01"

	// RedactThinkingBeta replaces thinking summaries with redacted blocks.
	RedactThinkingBeta = "redact-thinking-2026-02-12"

	// TokenEfficientToolsBeta enables JSON tool_use format (FC v3).
	TokenEfficientToolsBeta = "token-efficient-tools-2026-03-28"

	// SummarizeConnectorTextBeta enables connector text summarization (ant-only).
	SummarizeConnectorTextBeta = "summarize-connector-text-2026-03-13"

	// AFKModeBeta enables AFK mode (feature-flag gated).
	AFKModeBeta = "afk-mode-2026-01-31"

	// CLIInternalBeta is the CLI internal beta (ant-only).
	CLIInternalBeta = "cli-internal-2026-02-09"

	// AdvisorBeta enables the advisor tool.
	AdvisorBeta = "advisor-tool-2026-03-01"

	// OAuthBeta is sent for Claude.ai subscribers.
	OAuthBeta = "oauth-2025-04-20"
)

// FilesAPIBetaHeader is the combined beta header used by the Files API.
const FilesAPIBetaHeader = "files-api-2025-04-14,oauth-2025-04-20"

// BedrockExtraParamsHeaders contains beta headers that must be sent via
// extraBodyParams instead of HTTP headers when using AWS Bedrock.
var BedrockExtraParamsHeaders = map[string]bool{
	InterleavedThinkingBeta: true,
	Context1MBeta:           true,
	ToolSearch3PBeta:        true,
}

// VertexCountTokensAllowedBetas contains betas allowed on the Vertex
// countTokens API. Other betas will cause 400 errors.
var VertexCountTokensAllowedBetas = map[string]bool{
	ClaudeCodeBeta:           true,
	InterleavedThinkingBeta:  true,
	ContextManagementBeta:    true,
}

// AllowedSDKBetas lists betas that API key users may pass via SDK options.
var AllowedSDKBetas = map[string]bool{
	Context1MBeta: true,
}
