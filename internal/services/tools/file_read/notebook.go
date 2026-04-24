package file_read

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

// NotebookCell represents a single cell in a Jupyter notebook.
type NotebookCell struct {
	CellType string   `json:"cell_type"`
	Source   []string `json:"source"`
	Outputs  []any    `json:"outputs,omitempty"`
	Metadata any      `json:"metadata,omitempty"`
}

// NotebookFile represents the top-level structure of a Jupyter notebook.
type NotebookFile struct {
	Cells    []NotebookCell  `json:"cells"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
	Nbformat int             `json:"nbformat,omitempty"`
	NbformatMinor int        `json:"nbformat_minor,omitempty"`
}

// readNotebookFile reads a Jupyter notebook (.ipynb) file, parses its cells,
// and returns the cell data as a structured result.
func (t *Tool) readNotebookFile(ctx context.Context, filePath string, size int64, workingDir string) (coretool.Result, error) {
	content, err := t.fs.ReadFile(filePath)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("Failed to read notebook file: %v", err)}, nil
	}

	var notebook NotebookFile
	if err := json.Unmarshal(content, &notebook); err != nil {
		return coretool.Result{Error: fmt.Sprintf("Invalid notebook JSON: %v", err)}, nil
	}

	// Extract cells as []any (map[string]any per cell) for JSON serialization.
	cells := make([]any, 0, len(notebook.Cells))
	for _, cell := range notebook.Cells {
		cellMap := map[string]any{
			"cell_type": cell.CellType,
			"source":    cell.Source,
		}
		if cell.Outputs != nil {
			cellMap["outputs"] = cell.Outputs
		}
		if cell.Metadata != nil {
			cellMap["metadata"] = cell.Metadata
		}
		cells = append(cells, cellMap)
	}

	// Serialize cells to JSON to check size.
	cellsJSON, err := json.Marshal(cells)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("Failed to serialize notebook cells: %v", err)}, nil
	}

	maxSize := t.effectiveMaxFileSizeBytes()
	if int64(len(cellsJSON)) > maxSize {
		return coretool.Result{
			Error: fmt.Sprintf(
				"Notebook content (%s) exceeds maximum allowed size (%s). Use Bash with jq to read specific portions:\n"+
					"  cat \"%s\" | jq '.cells[:20]' # First 20 cells\n"+
					"  cat \"%s\" | jq '.cells[100:120]' # Cells 100-120\n"+
					"  cat \"%s\" | jq '.cells | length' # Count total cells\n"+
					"  cat \"%s\" | jq '.cells[] | select(.cell_type==\"code\") | .source' # All code sources",
				formatByteSize(int64(len(cellsJSON))),
				formatByteSize(maxSize),
				filePath,
				filePath,
				filePath,
				filePath,
			),
		}, nil
	}

	// Get file modification time for read state.
	info, err := t.fs.Stat(filePath)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("Failed to stat notebook file: %v", err)}, nil
	}

	output := NotebookOutput{
		Type:     "notebook",
		FilePath: platformfs.ToRelativePath(filePath, workingDir),
		Cells:    cells,
	}

	return coretool.Result{
		Output: fmt.Sprintf("Read notebook with %d cells", len(cells)),
		Meta: map[string]any{
			"data":       output,
			"read_state": buildReadStateSnapshot(filePath, info.ModTime(), 1, 0, time.Now()),
		},
	}, nil
}
