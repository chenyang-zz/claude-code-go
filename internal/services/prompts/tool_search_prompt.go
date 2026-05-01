package prompts

import "context"

// ToolSearchPromptSection provides usage guidance for the ToolSearch tool.
type ToolSearchPromptSection struct{}

// Name returns the section identifier.
func (s ToolSearchPromptSection) Name() string { return "tool_search_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s ToolSearchPromptSection) IsVolatile() bool { return false }

// Compute generates the ToolSearch tool usage guidance.
func (s ToolSearchPromptSection) Compute(ctx context.Context) (string, error) {
	return `# ToolSearch

Fetches full schema definitions for deferred tools so they can be called.

Deferred tools appear by name in system-reminder messages.

Until fetched, only the name is known — there is no parameter schema, so the tool cannot be invoked. This tool takes a query, matches it against the deferred tool list, and returns the matched tools' complete JSONSchema definitions inside a <functions> block. Once a tool's schema appears in that result, it is callable exactly like any tool defined at the top of the prompt.

Result format: each matched tool appears as one <function>{"description": "...", "name": "...", "parameters": {...}}</function> line inside the <functions> block — the same encoding as the tool list at the top of this prompt.

Query forms:
- "select:Read,Edit,Grep" — fetch these exact tools by name
- "notebook jupyter" — keyword search, up to max_results best matches
- "+slack send" — require "slack" in the name, rank by remaining terms`, nil
}
