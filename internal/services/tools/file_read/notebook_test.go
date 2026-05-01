package file_read

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

// TestReadNotebookFileReadsValidNotebook verifies a standard .ipynb is parsed
// and rendered as wrapped cell text. The expected Output now follows the
// `<cell id="X">…</cell id="X">` wrapping convention.
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

	// First cell: markdown wrapper with cell_type tag.
	if !strings.Contains(result.Output, `<cell id="cell-0"><cell_type>markdown</cell_type><source># Hello</source></cell id="cell-0">`) {
		t.Fatalf("readNotebookFile() output missing markdown cell wrapper, got: %q", result.Output)
	}
	// Second cell: code wrapper without cell_type tag, with stream output.
	if !strings.Contains(result.Output, `<cell id="cell-1"><source>print(1)</source><output>`) {
		t.Fatalf("readNotebookFile() output missing code cell wrapper, got: %q", result.Output)
	}
	if !strings.Contains(result.Output, "1\n") {
		t.Fatalf("readNotebookFile() output missing stream text, got: %q", result.Output)
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

// TestReadNotebookFileHandlesEmptyNotebook verifies an empty notebook returns
// an empty composed output and zero cells.
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

	if result.Output != "" {
		t.Fatalf("readNotebookFile() output = %q, want empty string for empty notebook", result.Output)
	}

	data, ok := result.Meta["data"].(NotebookOutput)
	if !ok {
		t.Fatalf("readNotebookFile() meta data type = %T", result.Meta["data"])
	}
	if len(data.Cells) != 0 {
		t.Fatalf("readNotebookFile() len(cells) = %d, want %d", len(data.Cells), 0)
	}

	if _, ok := result.Meta["images"]; ok {
		t.Fatal("readNotebookFile() Meta[\"images\"] should not be set for empty notebook")
	}
}

// TestReadNotebookFileReturnsErrorForInvalidJSON verifies malformed JSON is
// rejected gracefully.
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

// TestReadNotebookFileReturnsErrorWhenTooLarge verifies oversized notebook
// cells are rejected.
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

// TestReadNotebookFileViaInvoke verifies the full Invoke path dispatches to
// readNotebookFile.
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

// TestNotebookCellWrapperUsesExplicitID verifies an explicit cell.id from
// nbformat 4.5+ takes precedence over the index-based fallback.
func TestNotebookCellWrapperUsesExplicitID(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "id.ipynb")
	content := `{
  "cells": [
    {"id": "abc-123", "cell_type": "code", "source": ["pass"], "outputs": [], "metadata": {}}
  ],
  "metadata": {},
  "nbformat": 4,
  "nbformat_minor": 5
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
	if !strings.Contains(result.Output, `<cell id="abc-123">`) {
		t.Fatalf("output should use explicit cell.id, got %q", result.Output)
	}
	if strings.Contains(result.Output, `<cell id="cell-0">`) {
		t.Fatalf("output should not fall back to cell-0 when id is set, got %q", result.Output)
	}
}

// TestNotebookExtractsImagePNG verifies image/png outputs in display_data
// cells are extracted into Meta["images"] with whitespace removed.
func TestNotebookExtractsImagePNG(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "image.ipynb")

	rawPng := base64.StdEncoding.EncodeToString([]byte("fake-png-bytes"))
	// Inject some whitespace into the base64 to verify cleanBase64 removes it.
	pngWithSpaces := rawPng[:5] + "\n" + rawPng[5:10] + " " + rawPng[10:]

	content := fmt.Sprintf(`{
  "cells": [
    {
      "cell_type": "code",
      "source": ["plt.show()"],
      "outputs": [
        {
          "output_type": "display_data",
          "data": {
            "text/plain": ["<Figure>"],
            "image/png": %q
          },
          "metadata": {}
        }
      ],
      "metadata": {}
    }
  ],
  "metadata": {},
  "nbformat": 4,
  "nbformat_minor": 2
}`, pngWithSpaces)
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

	images, ok := result.Meta["images"].([]coretool.ImageData)
	if !ok {
		t.Fatalf("Meta[\"images\"] type = %T, want []coretool.ImageData", result.Meta["images"])
	}
	if len(images) != 1 {
		t.Fatalf("len(images) = %d, want 1", len(images))
	}
	if images[0].MediaType != "image/png" {
		t.Fatalf("MediaType = %q, want image/png", images[0].MediaType)
	}
	if images[0].Base64 != rawPng {
		t.Fatalf("base64 should have whitespace stripped, got %q, want %q", images[0].Base64, rawPng)
	}
	if !strings.Contains(result.Output, "<Figure>") {
		t.Fatalf("output should contain text/plain payload, got %q", result.Output)
	}
}

// TestNotebookExtractsImageJPEGWhenNoPNG verifies image/jpeg is extracted only
// when image/png is absent (PNG takes priority).
func TestNotebookExtractsImageJPEGWhenNoPNG(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "jpeg.ipynb")

	jpegB64 := base64.StdEncoding.EncodeToString([]byte("fake-jpeg-bytes"))
	content := fmt.Sprintf(`{
  "cells": [
    {
      "cell_type": "code",
      "source": ["render()"],
      "outputs": [
        {
          "output_type": "execute_result",
          "data": {
            "image/jpeg": %q
          },
          "metadata": {},
          "execution_count": 1
        }
      ],
      "metadata": {}
    }
  ],
  "metadata": {},
  "nbformat": 4,
  "nbformat_minor": 2
}`, jpegB64)
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

	images, ok := result.Meta["images"].([]coretool.ImageData)
	if !ok {
		t.Fatalf("Meta[\"images\"] type = %T", result.Meta["images"])
	}
	if len(images) != 1 || images[0].MediaType != "image/jpeg" {
		t.Fatalf("expected single image/jpeg, got %+v", images)
	}
}

// TestNotebookErrorOutputFormatting verifies error outputs render ename,
// evalue, and joined traceback text inside the cell wrapper.
func TestNotebookErrorOutputFormatting(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "error.ipynb")
	content := `{
  "cells": [
    {
      "cell_type": "code",
      "source": ["1/0"],
      "outputs": [
        {
          "output_type": "error",
          "ename": "ZeroDivisionError",
          "evalue": "division by zero",
          "traceback": ["Traceback (most recent call last):\n", "  File \"<ipython>\", line 1\n", "ZeroDivisionError: division by zero"]
        }
      ],
      "metadata": {}
    }
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

	if !strings.Contains(result.Output, "ZeroDivisionError: division by zero") {
		t.Fatalf("output missing error header, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "Traceback (most recent call last):") {
		t.Fatalf("output missing traceback, got %q", result.Output)
	}
}

// TestNotebookLargeOutputsAreTruncated verifies that when a single cell's
// outputs exceed largeOutputThreshold (text + image base64), they are
// replaced with a stub message and images are dropped for that cell.
func TestNotebookLargeOutputsAreTruncated(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "big.ipynb")

	// 11 KB of base64 data exceeds the 10 KB cell threshold by itself.
	bigPng := strings.Repeat("A", 11*1024)
	content := fmt.Sprintf(`{
  "cells": [
    {
      "cell_type": "code",
      "source": ["plt.show()"],
      "outputs": [
        {
          "output_type": "display_data",
          "data": {
            "text/plain": ["<Figure>"],
            "image/png": %q
          },
          "metadata": {}
        }
      ],
      "metadata": {}
    }
  ],
  "metadata": {},
  "nbformat": 4,
  "nbformat_minor": 2
}`, bigPng)
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

	if !strings.Contains(result.Output, "Outputs are too large to include") {
		t.Fatalf("output should contain truncation stub, got %q", result.Output)
	}
	if !strings.Contains(result.Output, ".cells[0].outputs") {
		t.Fatalf("output should reference cell index, got %q", result.Output)
	}

	// Images should be dropped for the truncated cell.
	if _, ok := result.Meta["images"]; ok {
		t.Fatal("Meta[\"images\"] should not be set when the only cell with images is truncated")
	}
}

// TestNotebookMultipleCellsWithMixedOutputs verifies image ordering across
// multiple cells: images are accumulated in cell order in Meta["images"].
func TestNotebookMultipleCellsWithMixedOutputs(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "mixed.ipynb")

	pngA := base64.StdEncoding.EncodeToString([]byte("png-a"))
	pngB := base64.StdEncoding.EncodeToString([]byte("png-b"))

	content := fmt.Sprintf(`{
  "cells": [
    {
      "cell_type": "code",
      "source": ["a()"],
      "outputs": [
        {"output_type": "display_data", "data": {"image/png": %q}, "metadata": {}}
      ],
      "metadata": {}
    },
    {
      "cell_type": "markdown",
      "source": ["## Section"],
      "metadata": {}
    },
    {
      "cell_type": "code",
      "source": ["b()"],
      "outputs": [
        {"output_type": "display_data", "data": {"image/png": %q}, "metadata": {}}
      ],
      "metadata": {}
    }
  ],
  "metadata": {},
  "nbformat": 4,
  "nbformat_minor": 2
}`, pngA, pngB)
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

	images, ok := result.Meta["images"].([]coretool.ImageData)
	if !ok {
		t.Fatalf("Meta[\"images\"] type = %T", result.Meta["images"])
	}
	if len(images) != 2 {
		t.Fatalf("len(images) = %d, want 2", len(images))
	}
	if images[0].Base64 != pngA {
		t.Fatalf("images[0].Base64 = %q, want %q", images[0].Base64, pngA)
	}
	if images[1].Base64 != pngB {
		t.Fatalf("images[1].Base64 = %q, want %q", images[1].Base64, pngB)
	}

	// All three cells should appear in Output in order.
	if !strings.Contains(result.Output, `<cell id="cell-0">`) ||
		!strings.Contains(result.Output, `<cell id="cell-1">`) ||
		!strings.Contains(result.Output, `<cell id="cell-2">`) {
		t.Fatalf("output should contain all three cell wrappers, got %q", result.Output)
	}
}

// TestProcessOutputStreamText verifies stream output text is prefixed with a
// newline (matching TS-side behavior) and supports both string and []string.
func TestProcessOutputStreamText(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want string
	}{
		{
			name: "string text",
			in:   map[string]any{"output_type": "stream", "text": "hello\n"},
			want: "\nhello\n",
		},
		{
			name: "array text",
			in:   map[string]any{"output_type": "stream", "text": []any{"line1\n", "line2"}},
			want: "\nline1\nline2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, image := processOutput(tc.in)
			if text != tc.want {
				t.Errorf("text = %q, want %q", text, tc.want)
			}
			if image != nil {
				t.Errorf("image = %+v, want nil", image)
			}
		})
	}
}

// TestProcessOutputUnknownType verifies that unknown or missing output_type
// values are silently ignored to keep the parser forward-compatible.
func TestProcessOutputUnknownType(t *testing.T) {
	text, image := processOutput(map[string]any{"output_type": "future_type", "text": "ignored"})
	if text != "" || image != nil {
		t.Fatalf("unknown type should produce no payload, got text=%q image=%+v", text, image)
	}
}
