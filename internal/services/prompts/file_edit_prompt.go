package prompts

import "context"

// FileEditPromptSection provides detailed usage guidance for the FileEdit tool.
type FileEditPromptSection struct{}

// Name returns the section identifier.
func (s FileEditPromptSection) Name() string { return "file_edit_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s FileEditPromptSection) IsVolatile() bool { return false }

// Compute generates the FileEdit tool usage guidance.
func (s FileEditPromptSection) Compute(ctx context.Context) (string, error) {
	return `# Edit Tool

Performs exact string replacements in files.

- You must use your Read tool at least once in the conversation before editing. This tool will error if you attempt an edit without reading the file.
- When editing text from Read tool output, ensure you preserve the exact indentation (tabs/spaces) as it appears AFTER the line number prefix. The line number prefix format is: line number + tab. Everything after that is the actual file content to match. Never include any part of the line number prefix in the old_string or new_string.
- ALWAYS prefer editing existing files in the codebase. NEVER write new files unless explicitly required.
- Only use emojis if the user explicitly requests it. Avoid adding emojis to files unless asked.
- The edit will FAIL if old_string is not unique in the file. Either provide a larger string with more surrounding context to make it unique or use replace_all to change every instance of old_string.
- Use replace_all for replacing and renaming strings across the file. This parameter is useful if you want to rename a variable for instance.`, nil
}
