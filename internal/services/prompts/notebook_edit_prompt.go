package prompts

import "context"

// NotebookEditPromptSection provides usage guidance for the NotebookEdit tool.
type NotebookEditPromptSection struct{}

// Name returns the section identifier.
func (s NotebookEditPromptSection) Name() string { return "notebook_edit_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s NotebookEditPromptSection) IsVolatile() bool { return false }

// Compute generates the NotebookEdit tool usage guidance.
func (s NotebookEditPromptSection) Compute(ctx context.Context) (string, error) {
	return `Completely replaces the contents of a specific cell in a Jupyter notebook (.ipynb file) with new source. Jupyter notebooks are interactive documents that combine code, text, and visualizations, commonly used for data analysis and scientific computing. The notebook_path parameter must be an absolute path, not a relative path. The cell_number is 0-indexed. Use edit_mode=insert to add a new cell at the index specified by cell_number. Use edit_mode=delete to delete the cell at the index specified by cell_number.`, nil
}
