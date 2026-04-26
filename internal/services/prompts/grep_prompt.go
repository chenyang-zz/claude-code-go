package prompts

import "context"

// GrepPromptSection provides detailed usage guidance for the Grep tool.
type GrepPromptSection struct{}

// Name returns the section identifier.
func (s GrepPromptSection) Name() string { return "grep_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s GrepPromptSection) IsVolatile() bool { return false }

// Compute generates the Grep tool usage guidance.
func (s GrepPromptSection) Compute(ctx context.Context) (string, error) {
	return `# Grep Tool

A powerful search tool built on ripgrep

- ALWAYS use Grep for search tasks. NEVER invoke ` + "`grep`" + ` or ` + "`rg`" + ` as a Bash command. The Grep tool has been optimized for correct permissions and access.
- Supports full regex syntax (e.g., "log.*Error", "function\\s+\\w+")
- Filter files with glob parameter (e.g., "*.js", "**/*.tsx") or type parameter (e.g., "js", "py", "rust")
- Output modes: "content" shows matching lines, "files_with_matches" shows only file paths (default), "count" shows match counts
- Use Agent tool for open-ended searches requiring multiple rounds
- Pattern syntax: Uses ripgrep (not grep) - literal braces need escaping (use ` + "`interface\\{\\}`" + ` to find ` + "`interface{}`" + ` in Go code)
- Multiline matching: By default patterns match within single lines only. For cross-line patterns like ` + "`struct \\{[\\s\\S]*?field`" + `, use ` + "`multiline: true`" + ``, nil
}
