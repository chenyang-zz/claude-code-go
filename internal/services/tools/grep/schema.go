package grep

import coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"

// inputSchema declares the GrepTool input contract for the second migration pass.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"pattern": {
				Type:        coretool.ValueKindString,
				Description: "The regular expression pattern to search for in file contents.",
				Required:    true,
			},
			"path": {
				Type:        coretool.ValueKindString,
				Description: "Optional file or directory to search in.",
			},
			"glob": {
				Type:        coretool.ValueKindString,
				Description: "Optional glob filter passed through to ripgrep.",
			},
			"output_mode": {
				Type:        coretool.ValueKindString,
				Description: `Optional result mode: "files_with_matches" (default), "content", or "count".`,
			},
			"-B": {
				Type:        coretool.ValueKindInteger,
				Description: `Optional number of lines to show before each match in "content" mode.`,
			},
			"-A": {
				Type:        coretool.ValueKindInteger,
				Description: `Optional number of lines to show after each match in "content" mode.`,
			},
			"-C": {
				Type:        coretool.ValueKindInteger,
				Description: `Optional alias for symmetric context lines in "content" mode.`,
			},
			"context": {
				Type:        coretool.ValueKindInteger,
				Description: `Optional number of lines to show before and after each match in "content" mode.`,
			},
			"-n": {
				Type:        coretool.ValueKindBoolean,
				Description: `Optional toggle for line numbers in "content" mode; defaults to true.`,
			},
			"-i": {
				Type:        coretool.ValueKindBoolean,
				Description: "Optional case-insensitive search flag.",
			},
			"type": {
				Type:        coretool.ValueKindString,
				Description: "Optional ripgrep file type filter.",
			},
			"head_limit": {
				Type:        coretool.ValueKindInteger,
				Description: "Optional limit for returned rows; zero disables pagination.",
			},
			"offset": {
				Type:        coretool.ValueKindInteger,
				Description: "Optional number of rows to skip before head_limit is applied.",
			},
			"multiline": {
				Type:        coretool.ValueKindBoolean,
				Description: "Optional toggle for ripgrep multiline mode (-U --multiline-dotall).",
			},
		},
	}
}
