package notebook_edit

import (
	"encoding/json"
	"strconv"
	"strings"
)

// NotebookCell represents a single cell in a Jupyter notebook.
// The Source field uses json.RawMessage to handle both string and []string formats.
type NotebookCell struct {
	CellType       string          `json:"cell_type"`
	ID             string          `json:"id,omitempty"`
	Source         json.RawMessage `json:"source"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	ExecutionCount *int            `json:"execution_count,omitempty"`
	Outputs        json.RawMessage `json:"outputs,omitempty"`
}

// NotebookLanguageInfo holds the kernel language metadata for a notebook.
type NotebookLanguageInfo struct {
	Name string `json:"name"`
}

// NotebookMetadata holds the top-level metadata for a notebook document.
type NotebookMetadata struct {
	LanguageInfo *NotebookLanguageInfo `json:"language_info,omitempty"`
}

// NotebookContent represents the top-level structure of a Jupyter notebook (.ipynb) file.
type NotebookContent struct {
	Cells         []NotebookCell   `json:"cells"`
	Metadata      NotebookMetadata `json:"metadata"`
	Nbformat      int              `json:"nbformat"`
	NbformatMinor int              `json:"nbformat_minor"`
}

// Language extracts the kernel language name from the notebook metadata, defaulting to "python".
func (n *NotebookContent) Language() string {
	if n.Metadata.LanguageInfo != nil && n.Metadata.LanguageInfo.Name != "" {
		return n.Metadata.LanguageInfo.Name
	}
	return "python"
}

// NeedsCellID reports whether the notebook format (nbformat >= 4.5) requires cell IDs.
func (n *NotebookContent) NeedsCellID() bool {
	return n.Nbformat > 4 || (n.Nbformat == 4 && n.NbformatMinor >= 5)
}

// parseCellID extracts the numeric index from a cell-N format string.
// Returns the parsed index and true, or 0 and false if the format does not match.
func parseCellID(cellID string) (int, bool) {
	if !strings.HasPrefix(cellID, "cell-") {
		return 0, false
	}
	numStr := strings.TrimPrefix(cellID, "cell-")
	index, err := strconv.Atoi(numStr)
	if err != nil || index < 0 {
		return 0, false
	}
	return index, true
}

// newSourceJSON marshals a string as a JSON value suitable for a notebook cell source field.
func newSourceJSON(source string) json.RawMessage {
	data, _ := json.Marshal(source)
	return json.RawMessage(data)
}
