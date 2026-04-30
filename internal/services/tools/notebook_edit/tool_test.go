package notebook_edit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

func makeValidNotebook() *NotebookContent {
	return &NotebookContent{
		Cells: []NotebookCell{
			{
				CellType: "code",
				ID:       "cell-0",
				Source:   json.RawMessage(`"print('hello')"`),
				Metadata: json.RawMessage("{}"),
				Outputs:  json.RawMessage("[]"),
			},
			{
				CellType: "markdown",
				ID:       "cell-1",
				Source:   json.RawMessage(`"# Title"`),
				Metadata: json.RawMessage("{}"),
			},
		},
		Metadata: NotebookMetadata{
			LanguageInfo: &NotebookLanguageInfo{Name: "python"},
		},
		Nbformat:      4,
		NbformatMinor: 5,
	}
}

func writeNotebook(t *testing.T, fs platformfs.FileSystem, path string, nb *NotebookContent) {
	t.Helper()
	data, err := json.MarshalIndent(nb, "", strings.Repeat(" ", notebookIndent))
	if err != nil {
		t.Fatalf("failed to marshal notebook: %v", err)
	}
	data = append(data, '\n')
	if err := fs.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("failed to write notebook: %v", err)
	}
}

// newTestPolicy creates a filesystem policy that allows writes within the given workspace.
func newTestPolicy(workspace string) (*corepermission.FilesystemPolicy, error) {
	return corepermission.NewFilesystemPolicy(corepermission.RuleSet{
		Write: []corepermission.Rule{
			{
				Source:   corepermission.RuleSourceUserSettings,
				Decision: corepermission.DecisionAllow,
				BaseDir:  workspace,
				Pattern:  "**",
			},
		},
	})
}

// mustNewPolicy creates a test policy or fails the test.
func mustNewPolicy(t *testing.T, workspace string) *corepermission.FilesystemPolicy {
	t.Helper()
	policy, err := newTestPolicy(workspace)
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}
	return policy
}

func TestName(t *testing.T) {
	tool := NewTool(nil, nil)
	if tool.Name() != Name {
		t.Errorf("Name() = %q, want %q", tool.Name(), Name)
	}
}

func TestDescription(t *testing.T) {
	tool := NewTool(nil, nil)
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestInputSchema(t *testing.T) {
	tool := NewTool(nil, nil)
	schema := tool.InputSchema()

	if _, ok := schema.Properties["notebook_path"]; !ok {
		t.Error("InputSchema should have notebook_path property")
	}
	if !schema.Properties["notebook_path"].Required {
		t.Error("notebook_path should be required")
	}
	if _, ok := schema.Properties["new_source"]; !ok {
		t.Error("InputSchema should have new_source property")
	}
	if !schema.Properties["new_source"].Required {
		t.Error("new_source should be required")
	}
	if _, ok := schema.Properties["cell_id"]; !ok {
		t.Error("InputSchema should have cell_id property")
	}
	if _, ok := schema.Properties["cell_type"]; !ok {
		t.Error("InputSchema should have cell_type property")
	}
	if _, ok := schema.Properties["edit_mode"]; !ok {
		t.Error("InputSchema should have edit_mode property")
	}
}

func TestIsReadOnly(t *testing.T) {
	tool := NewTool(nil, nil)
	if tool.IsReadOnly() {
		t.Error("NotebookEditTool should not be read-only")
	}
}

func TestIsConcurrencySafe(t *testing.T) {
	tool := NewTool(nil, nil)
	if !tool.IsConcurrencySafe() {
		t.Error("NotebookEditTool should be concurrency-safe")
	}
}

func TestRequiresUserInteraction(t *testing.T) {
	tool := NewTool(nil, nil)
	if !tool.RequiresUserInteraction() {
		t.Error("NotebookEditTool should require user interaction")
	}
}

// M3-1: Schema validation tests

func TestInvokeMissingNotebookPath(t *testing.T) {
	dir := t.TempDir()
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))
	call := coretool.Call{
		Input: map[string]any{
			"new_source": "print('test')",
		},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error == "" {
		t.Error("expected error for missing notebook_path")
	}
}

func TestInvokeEmptyNotebookPath(t *testing.T) {
	dir := t.TempDir()
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))
	call := coretool.Call{
		Input: map[string]any{
			"notebook_path": "",
			"new_source":    "print('test')",
		},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error == "" {
		t.Error("expected error for empty notebook_path")
	}
}

func TestInvokeMissingNewSource(t *testing.T) {
	dir := t.TempDir()
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))
	call := coretool.Call{
		Input: map[string]any{
			"notebook_path": filepath.Join(dir, "test.ipynb"),
		},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error == "" {
		t.Error("expected error for missing new_source")
	}
}

func TestInvokeNonIpynbExtension(t *testing.T) {
	dir := t.TempDir()
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))
	call := coretool.Call{
		Input: map[string]any{
			"notebook_path": filepath.Join(dir, "test.txt"),
			"new_source":    "print('test')",
		},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error != errNotIpynb {
		t.Errorf("expected error %q, got %q", errNotIpynb, result.Error)
	}
}

func TestInvokeInvalidEditMode(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	writeNotebook(t, fs, nbPath, makeValidNotebook())

	call := coretool.Call{
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "print('test')",
			"edit_mode":     "invalid",
		},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error != "Edit mode must be replace, insert, or delete." {
		t.Errorf("expected invalid edit mode error, got %q", result.Error)
	}
}

func TestInvokeInsertWithoutCellType(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	writeNotebook(t, fs, nbPath, makeValidNotebook())

	call := coretool.Call{
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "print('test')",
			"edit_mode":     "insert",
		},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error != errInsertNeedsType {
		t.Errorf("expected error %q, got %q", errInsertNeedsType, result.Error)
	}
}

// M3-2: validateInput tests

func TestInvokeFileNotExists(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "nonexistent.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	call := coretool.Call{
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "print('test')",
		},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error != errNotExists {
		t.Errorf("expected error %q, got %q", errNotExists, result.Error)
	}
}

func TestInvokeUnreadBeforeEdit(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	writeNotebook(t, fs, nbPath, makeValidNotebook())

	call := coretool.Call{
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "print('test')",
			"cell_id":       "cell-0",
		},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error != errUnreadBeforeEdit {
		t.Errorf("expected error %q, got %q", errUnreadBeforeEdit, result.Error)
	}
}

func TestInvokeModifiedSinceRead(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	writeNotebook(t, fs, nbPath, makeValidNotebook())

	call := coretool.Call{
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "print('test')",
			"cell_id":       "cell-0",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					nbPath: {
						ReadAt:          time.Now().Add(-1 * time.Hour),
						ObservedModTime: time.Now().Add(-2 * time.Hour),
					},
				},
			},
		},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error != errModifiedSinceRead {
		t.Errorf("expected error %q, got %q", errModifiedSinceRead, result.Error)
	}
}

func TestInvokeInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	if err := fs.WriteFile(nbPath, []byte("not json"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	info, err := fs.Stat(nbPath)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}

	call := coretool.Call{
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "print('test')",
			"cell_id":       "cell-0",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					nbPath: {
						ObservedModTime: info.ModTime(),
					},
				},
			},
		},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error != errNotValidJSON {
		t.Errorf("expected error %q, got %q", errNotValidJSON, result.Error)
	}
}

func TestInvokeCellNotFoundByID(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	writeNotebook(t, fs, nbPath, makeValidNotebook())
	info, _ := fs.Stat(nbPath)

	call := coretool.Call{
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "print('test')",
			"cell_id":       "nonexistent-cell",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					nbPath: {ObservedModTime: info.ModTime()},
				},
			},
		},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if !strings.Contains(result.Error, errCellNotFoundPrefix) {
		t.Errorf("expected cell not found error, got %q", result.Error)
	}
}

// M3-3: Invoke edit operation tests

func TestInvokeReplaceCell(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	nb := makeValidNotebook()
	writeNotebook(t, fs, nbPath, nb)
	info, _ := fs.Stat(nbPath)

	call := coretool.Call{
		ID: "test-replace",
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "print('hello world')",
			"cell_id":       "cell-0",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					nbPath: {ObservedModTime: info.ModTime()},
				},
			},
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	newData, _ := fs.ReadFile(nbPath)
	var updated NotebookContent
	if err := json.Unmarshal(newData, &updated); err != nil {
		t.Fatalf("failed to read updated notebook: %v", err)
	}
	if len(updated.Cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(updated.Cells))
	}

	var source string
	if err := json.Unmarshal(updated.Cells[0].Source, &source); err != nil {
		t.Fatalf("failed to unmarshal source: %v", err)
	}
	if source != "print('hello world')" {
		t.Errorf("expected source %q, got %q", "print('hello world')", source)
	}

	data, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatal("expected Output in metadata")
	}
	if data.EditMode != "replace" {
		t.Errorf("expected edit_mode 'replace', got %q", data.EditMode)
	}
	if data.OriginalFile == "" {
		t.Error("original_file should not be empty")
	}
	if data.UpdatedFile == "" {
		t.Error("updated_file should not be empty")
	}
}

func TestInvokeInsertCell(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	nb := makeValidNotebook()
	writeNotebook(t, fs, nbPath, nb)
	info, _ := fs.Stat(nbPath)

	call := coretool.Call{
		ID: "test-insert",
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "# New markdown cell",
			"cell_id":       "cell-0",
			"cell_type":     "markdown",
			"edit_mode":     "insert",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					nbPath: {ObservedModTime: info.ModTime()},
				},
			},
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	newData, _ := fs.ReadFile(nbPath)
	var updated NotebookContent
	if err := json.Unmarshal(newData, &updated); err != nil {
		t.Fatalf("failed to read updated notebook: %v", err)
	}
	if len(updated.Cells) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(updated.Cells))
	}
	if updated.Cells[1].CellType != "markdown" {
		t.Errorf("expected markdown type at index 1, got %q", updated.Cells[1].CellType)
	}
	if updated.Cells[1].ID == "" {
		t.Error("inserted cell should have an ID (nbformat >= 4.5)")
	}

	var source string
	json.Unmarshal(updated.Cells[1].Source, &source)
	if source != "# New markdown cell" {
		t.Errorf("expected source %q, got %q", "# New markdown cell", source)
	}

	data, _ := result.Meta["data"].(Output)
	if data.EditMode != "insert" {
		t.Errorf("expected edit_mode 'insert', got %q", data.EditMode)
	}
}

func TestInvokeDeleteCell(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	nb := makeValidNotebook()
	writeNotebook(t, fs, nbPath, nb)
	info, _ := fs.Stat(nbPath)

	call := coretool.Call{
		ID: "test-delete",
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "",
			"cell_id":       "cell-0",
			"edit_mode":     "delete",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					nbPath: {ObservedModTime: info.ModTime()},
				},
			},
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	newData, _ := fs.ReadFile(nbPath)
	var updated NotebookContent
	if err := json.Unmarshal(newData, &updated); err != nil {
		t.Fatalf("failed to read updated notebook: %v", err)
	}
	if len(updated.Cells) != 1 {
		t.Fatalf("expected 1 cell, got %d", len(updated.Cells))
	}
}

func TestInvokeReplaceWithCellNIndex(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	nb := makeValidNotebook()
	writeNotebook(t, fs, nbPath, nb)
	info, _ := fs.Stat(nbPath)

	call := coretool.Call{
		ID: "test-cellN",
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "# Updated title",
			"cell_id":       "cell-1",
			"edit_mode":     "replace",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					nbPath: {ObservedModTime: info.ModTime()},
				},
			},
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	newData, _ := fs.ReadFile(nbPath)
	var updated NotebookContent
	json.Unmarshal(newData, &updated)

	var source string
	json.Unmarshal(updated.Cells[1].Source, &source)
	if source != "# Updated title" {
		t.Errorf("expected source %q, got %q", "# Updated title", source)
	}
}

func TestInvokeReplaceCellOutOfBoundsConvertsToInsert(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	nb := makeValidNotebook()
	writeNotebook(t, fs, nbPath, nb)
	info, _ := fs.Stat(nbPath)

	call := coretool.Call{
		ID: "test-oob",
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "print('new')",
			"cell_id":       "cell-2",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					nbPath: {ObservedModTime: info.ModTime()},
				},
			},
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	newData, _ := fs.ReadFile(nbPath)
	var updated NotebookContent
	json.Unmarshal(newData, &updated)

	if len(updated.Cells) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(updated.Cells))
	}
	data, _ := result.Meta["data"].(Output)
	if data.EditMode != "insert" {
		t.Errorf("expected edit_mode 'insert' (replace out-of-bounds → insert), got %q", data.EditMode)
	}
}

func TestInvokeCellNIndexOutOfBounds(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	nb := makeValidNotebook()
	writeNotebook(t, fs, nbPath, nb)
	info, _ := fs.Stat(nbPath)

	call := coretool.Call{
		ID: "test-oob-error",
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "print('test')",
			"cell_id":       "cell-5",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					nbPath: {ObservedModTime: info.ModTime()},
				},
			},
		},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if !strings.Contains(result.Error, "does not exist in notebook") {
		t.Errorf("expected out of bounds error, got %q", result.Error)
	}
}

// M3-4: Integration and edge case tests

func TestInvokeNilReceiver(t *testing.T) {
	var tool *Tool
	call := coretool.Call{Input: map[string]any{}}
	_, err := tool.Invoke(context.Background(), call)
	if err == nil {
		t.Error("expected error for nil receiver")
	}
}

func TestInvokeNoReadState(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	writeNotebook(t, fs, nbPath, makeValidNotebook())

	call := coretool.Call{
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "print('test')",
			"cell_id":       "cell-0",
		},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error != errUnreadBeforeEdit {
		t.Errorf("expected %q, got %q", errUnreadBeforeEdit, result.Error)
	}
}

func TestInvokeNotebookWithArraySource(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	nb := makeValidNotebook()
	nb.Cells[0].Source = json.RawMessage(`["print('hello')", ""]`)
	writeNotebook(t, fs, nbPath, nb)
	info, _ := fs.Stat(nbPath)

	call := coretool.Call{
		ID: "test-array-source",
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "print('replaced')",
			"cell_id":       "cell-0",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					nbPath: {ObservedModTime: info.ModTime()},
				},
			},
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	newData, _ := fs.ReadFile(nbPath)
	var updated NotebookContent
	json.Unmarshal(newData, &updated)

	var source string
	json.Unmarshal(updated.Cells[0].Source, &source)
	if source != "print('replaced')" {
		t.Errorf("expected 'print('replaced')', got %q", source)
	}
}

func TestParseCellID(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		ok       bool
	}{
		{"cell-0", 0, true},
		{"cell-1", 1, true},
		{"cell-123", 123, true},
		{"cell-abc", 0, false},
		{"cell-", 0, false},
		{"not-a-cell", 0, false},
		{"", 0, false},
		{"cell--1", 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			idx, ok := parseCellID(tc.input)
			if ok != tc.ok {
				t.Errorf("parseCellID(%q) ok = %v, want %v", tc.input, ok, tc.ok)
			}
			if idx != tc.expected {
				t.Errorf("parseCellID(%q) = %d, want %d", tc.input, idx, tc.expected)
			}
		})
	}
}

func TestNotebookLanguage(t *testing.T) {
	nb := &NotebookContent{
		Metadata: NotebookMetadata{
			LanguageInfo: &NotebookLanguageInfo{Name: "julia"},
		},
	}
	if nb.Language() != "julia" {
		t.Errorf("expected 'julia', got %q", nb.Language())
	}

	nb2 := &NotebookContent{}
	if nb2.Language() != "python" {
		t.Errorf("expected default 'python', got %q", nb2.Language())
	}
}

func TestNotebookNeedsCellID(t *testing.T) {
	tests := []struct {
		format int
		minor  int
		needs  bool
	}{
		{4, 4, false},
		{4, 5, true},
		{5, 0, true},
		{4, 6, true},
		{3, 0, false},
	}
	for _, tc := range tests {
		nb := &NotebookContent{Nbformat: tc.format, NbformatMinor: tc.minor}
		if nb.NeedsCellID() != tc.needs {
			t.Errorf("nbformat=%d minor=%d NeedsCellID() = %v, want %v",
				tc.format, tc.minor, nb.NeedsCellID(), tc.needs)
		}
	}
}

func TestInsertWithoutCellID(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	nb := makeValidNotebook()
	writeNotebook(t, fs, nbPath, nb)
	info, _ := fs.Stat(nbPath)

	call := coretool.Call{
		ID: "test-insert-no-id",
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "# First cell",
			"cell_type":     "markdown",
			"edit_mode":     "insert",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					nbPath: {ObservedModTime: info.ModTime()},
				},
			},
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	newData, _ := fs.ReadFile(nbPath)
	var updated NotebookContent
	json.Unmarshal(newData, &updated)
	if len(updated.Cells) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(updated.Cells))
	}
	var source string
	json.Unmarshal(updated.Cells[0].Source, &source)
	if source != "# First cell" {
		t.Errorf("expected '# First cell' at index 0, got %q", source)
	}
}

func TestInsertAtEnd(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	nb := makeValidNotebook()
	writeNotebook(t, fs, nbPath, nb)
	info, _ := fs.Stat(nbPath)

	call := coretool.Call{
		ID: "test-insert-end",
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "# Last cell",
			"cell_id":       "cell-1",
			"cell_type":     "markdown",
			"edit_mode":     "insert",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					nbPath: {ObservedModTime: info.ModTime()},
				},
			},
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	newData, _ := fs.ReadFile(nbPath)
	var updated NotebookContent
	json.Unmarshal(newData, &updated)
	if len(updated.Cells) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(updated.Cells))
	}
}

func TestInvokeDirectoryInsteadOfFile(t *testing.T) {
	dir := t.TempDir()
	dirPath := filepath.Join(dir, "notebook.ipynb")
	// Create a directory with .ipynb name to test directory rejection after passing extension check.
	if err := os.Mkdir(dirPath, 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	info, _ := fs.Stat(dirPath)
	call := coretool.Call{
		Input: map[string]any{
			"notebook_path": dirPath,
			"new_source":    "print('test')",
			"cell_id":       "cell-0",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					dirPath: {ObservedModTime: info.ModTime()},
				},
			},
		},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if !strings.Contains(result.Error, "directory") {
		t.Errorf("expected directory error, got %q", result.Error)
	}
}

func TestRelativePathResolution(t *testing.T) {
	dir := t.TempDir()
	nbPath := "relative.ipynb"
	absPath := filepath.Join(dir, nbPath)
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	writeNotebook(t, fs, absPath, makeValidNotebook())
	info, _ := fs.Stat(absPath)

	call := coretool.Call{
		ID: "test-relative",
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "print('relative')",
			"cell_id":       "cell-0",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					absPath: {ObservedModTime: info.ModTime()},
				},
			},
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
}

func TestDefaultEditMode(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	fs := platformfs.NewLocalFS()
	tool := NewTool(fs, mustNewPolicy(t, dir))

	nb := makeValidNotebook()
	writeNotebook(t, fs, nbPath, nb)
	info, _ := fs.Stat(nbPath)

	call := coretool.Call{
		ID: "test-default-edit-mode",
		Input: map[string]any{
			"notebook_path": nbPath,
			"new_source":    "print('default')",
			"cell_id":       "cell-0",
		},
		Context: coretool.UseContext{
			WorkingDir: dir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					nbPath: {ObservedModTime: info.ModTime()},
				},
			},
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data, _ := result.Meta["data"].(Output)
	if data.EditMode != "replace" {
		t.Errorf("expected default edit_mode 'replace', got %q", data.EditMode)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
