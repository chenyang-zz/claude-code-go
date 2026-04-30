package notebook_edit

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registry identifier for the NotebookEditTool.
	Name = "NotebookEdit"

	// maxNotebookSize is the maximum notebook file size (100 MiB) the tool will process.
	maxNotebookSize = 100 * 1024 * 1024

	// defaultFilePerm is the fallback file mode for written notebook files.
	defaultFilePerm = 0o644

	// notebookIndent is the JSON indentation used when serializing notebooks.
	notebookIndent = 1
)

// Error messages that mirror the TypeScript NotebookEditTool error codes.
const (
	errNotExists          = "Notebook file does not exist."
	errNotIpynb           = "File must be a Jupyter notebook (.ipynb file). For editing other file types, use the FileEdit tool."
	errNotValidJSON       = "Notebook is not valid JSON."
	errCellIDRequired     = "Cell ID must be specified when not inserting a new cell."
	errCellNotFoundPrefix = "Cell with ID"
	errCellNotFoundSuffix = "not found in notebook."
	errInsertNeedsType    = "Cell type is required when using edit_mode=insert."
	errUnreadBeforeEdit   = "File has not been read yet. Read it first before writing to it."
	errModifiedSinceRead  = "File has been modified since read, either by the user or by a linter. Read it again before attempting to write it."
)

// Tool implements the NotebookEditTool for editing Jupyter notebook cells.
type Tool struct {
	fs     platformfs.FileSystem
	policy *corepermission.FilesystemPolicy
}

// Input is the typed request payload for NotebookEditTool.
type Input struct {
	NotebookPath string `json:"notebook_path"`
	CellID       string `json:"cell_id,omitempty"`
	NewSource    string `json:"new_source"`
	CellType     string `json:"cell_type,omitempty"`
	EditMode     string `json:"edit_mode,omitempty"`
}

// Output is the structured result returned by NotebookEditTool.
type Output struct {
	NewSource      string `json:"new_source"`
	CellID         string `json:"cell_id,omitempty"`
	CellType       string `json:"cell_type"`
	Language       string `json:"language"`
	EditMode       string `json:"edit_mode"`
	Error          string `json:"error,omitempty"`
	NotebookPath   string `json:"notebook_path"`
	OriginalFile   string `json:"original_file"`
	UpdatedFile    string `json:"updated_file"`
}

// NewTool constructs a NotebookEditTool with explicit host dependencies.
func NewTool(fs platformfs.FileSystem, policy *corepermission.FilesystemPolicy) *Tool {
	return &Tool{
		fs:     fs,
		policy: policy,
	}
}

// Name returns the stable tool identifier.
func (t *Tool) Name() string {
	return Name
}

// Description returns a short human-readable summary for the tool.
func (t *Tool) Description() string {
	return "Completely replaces the contents of a specific cell in a Jupyter notebook (.ipynb file) with new source. Jupyter notebooks are interactive documents that combine code, text, and visualizations, commonly used for data analysis and scientific computing. The notebook_path parameter must be an absolute path, not a relative path. The cell_number is 0-indexed. Use edit_mode=insert to add a new cell at the index specified by cell_number. Use edit_mode=delete to delete the cell at the index specified by cell_number."
}

// InputSchema returns the declared input contract for NotebookEditTool.
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"notebook_path": {
				Type:        coretool.ValueKindString,
				Description: "The absolute path to the Jupyter notebook file to edit (must be absolute, not relative)",
				Required:    true,
			},
			"cell_id": {
				Type:        coretool.ValueKindString,
				Description: "The ID of the cell to edit. When inserting a new cell, the new cell will be inserted after the cell with this ID, or at the beginning if not specified.",
			},
			"new_source": {
				Type:        coretool.ValueKindString,
				Description: "The new source for the cell",
				Required:    true,
			},
			"cell_type": {
				Type:        coretool.ValueKindString,
				Description: "The type of the cell (code or markdown). If not specified, it defaults to the current cell type. If using edit_mode=insert, this is required.",
			},
			"edit_mode": {
				Type:        coretool.ValueKindString,
				Description: "The type of edit to make (replace, insert, delete). Defaults to replace.",
			},
		},
	}
}

// IsReadOnly reports that NotebookEditTool mutates filesystem state.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that independent notebook cell edits can run in parallel.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// RequiresUserInteraction indicates that the tool requires user approval before execution.
func (t *Tool) RequiresUserInteraction() bool {
	return true
}

// Invoke executes the notebook edit operation with validation and mutation.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("notebook edit tool: nil receiver")
	}
	if t.fs == nil {
		return coretool.Result{}, fmt.Errorf("notebook edit tool: filesystem is not configured")
	}
	if t.policy == nil {
		return coretool.Result{}, fmt.Errorf("notebook edit tool: permission policy is not configured")
	}

	input, err := coretool.DecodeInput[Input](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}
	if strings.TrimSpace(input.NotebookPath) == "" {
		return coretool.Result{Error: "notebook_path is required"}, nil
	}
	// Normalize edit_mode default.
	editMode := input.EditMode
	if editMode == "" {
		editMode = "replace"
	}
	if editMode != "replace" && editMode != "insert" && editMode != "delete" {
		return coretool.Result{Error: "Edit mode must be replace, insert, or delete."}, nil
	}

	// new_source is required for replace and insert modes. Delete mode ignores it.
	if editMode != "delete" && strings.TrimSpace(input.NewSource) == "" {
		return coretool.Result{Error: "new_source is required"}, nil
	}

	// Resolve the absolute path.
	fullPath, err := platformfs.ExpandPath(input.NotebookPath, call.Context.WorkingDir)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("notebook edit tool: expand path: %v", err)}, nil
	}

	// Validate .ipynb extension.
	if !strings.EqualFold(filepath.Ext(fullPath), ".ipynb") {
		return coretool.Result{Error: errNotIpynb}, nil
	}

	// Validate insert requires cell_type.
	if editMode == "insert" && input.CellType == "" {
		return coretool.Result{Error: errInsertNeedsType}, nil
	}

	// Permission check.
	evaluation := t.policy.CheckWritePermissionForTool(ctx, t.Name(), fullPath, call.Context.WorkingDir)
	if err := evaluation.ToError(corepermission.FilesystemRequest{
		ToolName:   t.Name(),
		Path:       fullPath,
		WorkingDir: call.Context.WorkingDir,
		Access:     corepermission.AccessWrite,
	}); err != nil {
		return coretool.Result{}, err
	}

	// Check file exists.
	info, err := t.fs.Stat(fullPath)
	if err != nil {
		if platformfs.IsNotExist(err) {
			return coretool.Result{Error: errNotExists}, nil
		}
		return coretool.Result{Error: fmt.Sprintf("notebook edit tool: stat file: %v", err)}, nil
	}
	if info.IsDir() {
		return coretool.Result{Error: "Path is a directory, not a file: " + input.NotebookPath}, nil
	}
	if info.Size() > maxNotebookSize {
		return coretool.Result{Error: fmt.Sprintf("Notebook file is too large (%d bytes).", info.Size())}, nil
	}

	// Validate read-before-edit.
	state, ok := call.Context.LookupReadState(fullPath)
	if !ok {
		return coretool.Result{Error: errUnreadBeforeEdit}, nil
	}
	modTime := info.ModTime()
	if !state.ObservedModTime.IsZero() && modTime.After(state.ObservedModTime) {
		return coretool.Result{Error: errModifiedSinceRead}, nil
	}

	// Read notebook content.
	originalBytes, err := t.fs.ReadFile(fullPath)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("notebook edit tool: read file: %v", err)}, nil
	}
	originalContent := string(originalBytes)

	// Parse notebook JSON.
	var notebook NotebookContent
	if err := json.Unmarshal(originalBytes, &notebook); err != nil {
		return coretool.Result{Error: errNotValidJSON}, nil
	}

	// Validate cell lookup when not inserting.
	if editMode != "insert" && input.CellID == "" {
		return coretool.Result{Error: errCellIDRequired}, nil
	}

	// Resolve cell index.
	cellIndex := -1
	if input.CellID != "" {
		for i := range notebook.Cells {
			if notebook.Cells[i].ID == input.CellID {
				cellIndex = i
				break
			}
		}
		if cellIndex == -1 {
			// Try parsing as cell-N format.
			if parsed, ok := parseCellID(input.CellID); ok {
				if parsed >= 0 && parsed <= len(notebook.Cells) {
					cellIndex = parsed
				} else {
					return coretool.Result{
						Error: fmt.Sprintf("Cell with index %d does not exist in notebook.", parsed),
					}, nil
				}
			} else {
				return coretool.Result{
					Error: fmt.Sprintf("%s \"%s\" %s", errCellNotFoundPrefix, input.CellID, errCellNotFoundSuffix),
				}, nil
			}
		}
	}

	// Execute the edit operation.
	language := notebook.Language()
	cellType := input.CellType
	newCellID := input.CellID

	switch editMode {
	case "delete":
		notebook.Cells = append(notebook.Cells[:cellIndex], notebook.Cells[cellIndex+1:]...)

	case "insert":
		if cellIndex == -1 {
			cellIndex = 0 // Default to beginning if no cell_id.
		} else {
			cellIndex++ // Insert after the specified cell.
		}
		if cellType == "" {
			cellType = "code"
		}
		// Generate ID for nbformat >= 4.5.
		cid := ""
		if notebook.NeedsCellID() {
			cid = generateCellID()
		}
		newCell := buildCell(cellType, cid, input.NewSource)
		notebook.Cells = append(notebook.Cells[:cellIndex], append([]NotebookCell{newCell}, notebook.Cells[cellIndex:]...)...)
		newCellID = cid

	case "replace":
		// Auto-convert replace to insert at end of cell list.
		if cellIndex == len(notebook.Cells) {
			if cellType == "" {
				cellType = "code"
			}
			cid := ""
			if notebook.NeedsCellID() {
				cid = generateCellID()
			}
			newCell := buildCell(cellType, cid, input.NewSource)
			notebook.Cells = append(notebook.Cells, newCell)
			newCellID = cid
			editMode = "insert"
		} else {
			targetCell := &notebook.Cells[cellIndex]
			targetCell.Source = newSourceJSON(input.NewSource)
			if targetCell.CellType == "code" {
				targetCell.ExecutionCount = nil
				targetCell.Outputs = nil
			}
			if cellType != "" && cellType != targetCell.CellType {
				targetCell.CellType = cellType
			}
		}
	}

	// Serialize and write back.
	updatedBytes, err := json.MarshalIndent(notebook, "", strings.Repeat(" ", notebookIndent))
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("notebook edit tool: serialize notebook: %v", err)}, nil
	}
	// Ensure trailing newline (Jupyter convention).
	updatedContent := string(updatedBytes) + "\n"

	perm := info.Mode().Perm()
	if perm == 0 {
		perm = defaultFilePerm
	}
	if err := t.fs.WriteFile(fullPath, []byte(updatedContent), perm); err != nil {
		return coretool.Result{Error: fmt.Sprintf("notebook edit tool: write file: %v", err)}, nil
	}

	logger.DebugCF("notebook_edit_tool", "notebook edit completed", map[string]any{
		"notebook_path": fullPath,
		"edit_mode":     editMode,
	})

	return coretool.Result{
		Output: fmt.Sprintf("Edited notebook cell (%s mode)", editMode),
		Meta: map[string]any{
			"data": Output{
				NewSource:    input.NewSource,
				CellID:       newCellID,
				CellType:     cellType,
				Language:     language,
				EditMode:     editMode,
				NotebookPath: fullPath,
				OriginalFile: originalContent,
				UpdatedFile:  updatedContent,
			},
		},
	}, nil
}

// buildCell creates a new NotebookCell with the given type, ID, and source.
func buildCell(cellType string, id string, source string) NotebookCell {
	cell := NotebookCell{
		CellType: cellType,
		Source:   newSourceJSON(source),
		Metadata: json.RawMessage("{}"),
	}
	if id != "" {
		cell.ID = id
	}
	if cellType == "code" {
		cell.ExecutionCount = nil
		cell.Outputs = json.RawMessage("[]")
	}
	return cell
}

// generateCellID creates a random cell ID for nbformat >= 4.5 notebooks.
func generateCellID() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	const length = 13
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return string(b)
}
