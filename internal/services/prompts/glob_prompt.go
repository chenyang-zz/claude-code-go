package prompts

import "context"

// GlobPromptSection provides detailed usage guidance for the Glob tool.
type GlobPromptSection struct{}

// Name returns the section identifier.
func (s GlobPromptSection) Name() string { return "glob_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s GlobPromptSection) IsVolatile() bool { return false }

// Compute generates the Glob tool usage guidance.
func (s GlobPromptSection) Compute(ctx context.Context) (string, error) {
	return `# Glob Tool

- Fast file pattern matching tool that works with any codebase size
- Supports glob patterns like "**/*.js" or "src/**/*.ts"
- Returns matching file paths sorted by modification time
- Use this tool when you need to find files by name patterns
- When you are doing an open ended search that may require multiple rounds of globbing and grepping, use the Agent tool instead`, nil
}
