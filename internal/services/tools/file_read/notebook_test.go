package file_read

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

// TestReadNotebookFileReadsValidNotebook verifies a standard .ipynb is parsed and returned.
func TestReadNotebookFileReadsValidNotebook(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "test.ipynb")
	content := `{
  "cells": [
    {"cell_type": "markdown", "source": ["# Hello"], "metadata": {}},
    {"cell_type": "code", "source": ["print(1)"], "outputs": [{"output_type": "stream", "text": ["1\n"]}], "metadata": {}}
  ],
  "metadata": {},
  "nbformat": 4,
  "nbformat_minor": 2
}`
	mustWriteFile(t, filePath, content)

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)
	result, err := tool.readNotebookFile(context.Background(), filePath, int64(len(content)), projectDir)
	if err != nil {
		t.Fatalf("readNotebookFile() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("readNotebookFile() result.Error = %q", result.Error)
	}

	wantOutput := "Read notebook with 2 cells"
	if result.Output != wantOutput {
		t.Fatalf("readNotebookFile() output = %q, want %q", result.Output, wantOutput)
	}

	data, ok := result.Meta["data"].(NotebookOutput)
	if !ok {
		t.Fatalf("readNotebookFile() meta data type = %T", result.Meta["data"])
	}
	if data.Type != "notebook" {
		t.Fatalf("readNotebookFile() data.Type = %q, want %q", data.Type, "notebook")
	}
	if data.FilePath != "test.ipynb" {
		t.Fatalf("readNotebookFile() data.FilePath = %q, want %q", data.FilePath, "test.ipynb")
	}
	if len(data.Cells) != 2 {
		t.Fatalf("readNotebookFile() len(cells) = %d, want %d", len(data.Cells), 2)
	}

	firstCell, ok := data.Cells[0].(map[string]any)
	if !ok {
		t.Fatalf("readNotebookFile() first cell type = %T", data.Cells[0])
	}
	if firstCell["cell_type"] != "markdown" {
		t.Fatalf("readNotebookFile() first cell type = %q, want %q", firstCell["cell_type"], "markdown")
	}

	readState, ok := result.Meta["read_state"].(coretool.ReadStateSnapshot)
	if !ok {
		t.Fatalf("readNotebookFile() read_state type = %T", result.Meta["read_state"])
	}
	state, ok := readState.Lookup(filePath)
	if !ok {
		t.Fatalf("readNotebookFile() missing read state for %q", filePath)
	}
	if state.IsPartial {
		t.Fatal("readNotebookFile() read state IsPartial = true, want false")
	}
	if state.ReadOffset != 1 {
		t.Fatalf("readNotebookFile() read state ReadOffset = %d, want %d", state.ReadOffset, 1)
	}
}

// TestReadNotebookFileHandlesEmptyNotebook verifies an empty notebook returns zero cells.
func TestReadNotebookFileHandlesEmptyNotebook(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "empty.ipynb")
	content := `{"cells": [], "metadata": {}, "nbformat": 4, "nbformat_minor": 2}`
	mustWriteFile(t, filePath, content)

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)
	result, err := tool.readNotebookFile(context.Background(), filePath, int64(len(content)), projectDir)
	if err != nil {
		t.Fatalf("readNotebookFile() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("readNotebookFile() result.Error = %q", result.Error)
	}

	if result.Output != "Read notebook with 0 cells" {
		t.Fatalf("readNotebookFile() output = %q, want %q", result.Output, "Read notebook with 0 cells")
	}

	data, ok := result.Meta["data"].(NotebookOutput)
	if !ok {
		t.Fatalf("readNotebookFile() meta data type = %T", result.Meta["data"])
	}
	if len(data.Cells) != 0 {
		t.Fatalf("readNotebookFile() len(cells) = %d, want %d", len(data.Cells), 0)
	}
}

// TestReadNotebookFileReturnsErrorForInvalidJSON verifies malformed JSON is rejected gracefully.
func TestReadNotebookFileReturnsErrorForInvalidJSON(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "bad.ipynb")
	content := `{"cells": [invalid json here]}`
	mustWriteFile(t, filePath, content)

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)
	result, err := tool.readNotebookFile(context.Background(), filePath, int64(len(content)), projectDir)
	if err != nil {
		t.Fatalf("readNotebookFile() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("readNotebookFile() expected error, got none")
	}
	if !strings.Contains(result.Error, "Invalid notebook JSON") {
		t.Fatalf("readNotebookFile() error = %q, want containing %q", result.Error, "Invalid notebook JSON")
	}
}

// TestReadNotebookFileReturnsErrorWhenTooLarge verifies oversized notebook cells are rejected.
func TestReadNotebookFileReturnsErrorWhenTooLarge(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "huge.ipynb")

	// Build a notebook with one cell whose source is a very large string.
	largeSource := strings.Repeat("a", 300*1024)
	content := fmt.Sprintf(`{"cells": [{"cell_type": "code", "source": [%q], "metadata": {}}], "metadata": {}, "nbformat": 4, "nbformat_minor": 2}`, largeSource)
	mustWriteFile(t, filePath, content)

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)
	result, err := tool.readNotebookFile(context.Background(), filePath, int64(len(content)), projectDir)
	if err != nil {
		t.Fatalf("readNotebookFile() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("readNotebookFile() expected error for oversized notebook, got none")
	}
	if !strings.Contains(result.Error, "exceeds maximum allowed size") {
		t.Fatalf("readNotebookFile() error = %q, want containing %q", result.Error, "exceeds maximum allowed size")
	}
	if !strings.Contains(result.Error, "jq") {
		t.Fatalf("readNotebookFile() error = %q, want containing jq hint", result.Error)
	}
}

// TestReadNotebookFileViaInvoke verifies the full Invoke path dispatches to readNotebookFile.
func TestReadNotebookFileViaInvoke(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notebook.ipynb")
	content := `{
  "cells": [
    {"cell_type": "code", "source": ["x = 1"], "outputs": [], "metadata": {}}
  ],
  "metadata": {},
  "nbformat": 4,
  "nbformat_minor": 2
}`
	mustWriteFile(t, filePath, content)

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path": "notebook.ipynb",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}

	data, ok := result.Meta["data"].(NotebookOutput)
	if !ok {
		t.Fatalf("Invoke() meta data type = %T", result.Meta["data"])
	}
	if data.Type != "notebook" {
		t.Fatalf("Invoke() data.Type = %q, want %q", data.Type, "notebook")
	}
	if len(data.Cells) != 1 {
		t.Fatalf("Invoke() len(cells) = %d, want %d", len(data.Cells), 1)
	}
}
