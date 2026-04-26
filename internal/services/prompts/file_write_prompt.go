package prompts

import "context"

// FileWritePromptSection provides detailed usage guidance for the FileWrite tool.
type FileWritePromptSection struct{}

// Name returns the section identifier.
func (s FileWritePromptSection) Name() string { return "file_write_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s FileWritePromptSection) IsVolatile() bool { return false }

// Compute generates the FileWrite tool usage guidance.
func (s FileWritePromptSection) Compute(ctx context.Context) (string, error) {
	return `# Write Tool

Writes a file to the local filesystem.

- This tool will overwrite the existing file if there is one at the provided path.
- If this is an existing file, you MUST use the Read tool first to read the file's contents. This tool will fail if you did not read the file first.
- Prefer the Edit tool for modifying existing files — it only sends the diff. Only use this tool to create new files or for complete rewrites.
- NEVER create documentation files (*.md) or README files unless explicitly requested by the User.
- Only use emojis if the user explicitly requests it. Avoid writing emojis to files unless asked.`, nil
}
