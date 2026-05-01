package file_read

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

// largeOutputThreshold caps the per-cell normalized output size (text bytes
// plus image base64 bytes). When exceeded, the cell's outputs are replaced
// with a stub instructing the model to read them via Bash + jq.
const largeOutputThreshold = 10 * 1024

// notebookOutputWhitespaceRe strips whitespace from base64 image payloads
// embedded in notebook outputs, mirroring the TS-side `replace(/\s/g, '')`.
var notebookOutputWhitespaceRe = regexp.MustCompile(`\s+`)

// NotebookCell represents a single cell in a Jupyter notebook.
type NotebookCell struct {
	// ID is the cell identifier introduced in nbformat 4.5+. Optional.
	ID string `json:"id,omitempty"`
	// CellType identifies the cell variant (markdown / code / raw).
	CellType string `json:"cell_type"`
	// Source carries cell source code or markdown as an array of lines
	// (each entry typically already terminated by "\n").
	Source []string `json:"source"`
	// Outputs is the list of execution outputs for code cells.
	Outputs []any `json:"outputs,omitempty"`
	// Metadata is the opaque per-cell metadata blob.
	Metadata any `json:"metadata,omitempty"`
}

// NotebookFile represents the top-level structure of a Jupyter notebook.
type NotebookFile struct {
	Cells         []NotebookCell  `json:"cells"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	Nbformat      int             `json:"nbformat,omitempty"`
	NbformatMinor int             `json:"nbformat_minor,omitempty"`
}

// readNotebookFile reads a Jupyter notebook (.ipynb) file, parses its cells,
// normalizes each cell into a wrapped text representation, extracts image
// outputs (image/png / image/jpeg) into Meta["images"], and returns the
// composed tool result. The runtime engine appends the image entries as
// independent ImagePart blocks alongside the tool_result text.
func (t *Tool) readNotebookFile(ctx context.Context, filePath string, size int64, workingDir string) (coretool.Result, error) {
	content, err := t.fs.ReadFile(filePath)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("Failed to read notebook file: %v", err)}, nil
	}

	var notebook NotebookFile
	if err := json.Unmarshal(content, &notebook); err != nil {
		return coretool.Result{Error: fmt.Sprintf("Invalid notebook JSON: %v", err)}, nil
	}

	// Build the legacy cells slice for backwards-compatible NotebookOutput.
	cells := make([]any, 0, len(notebook.Cells))
	for _, cell := range notebook.Cells {
		cellMap := map[string]any{
			"cell_type": cell.CellType,
			"source":    cell.Source,
		}
		if cell.ID != "" {
			cellMap["id"] = cell.ID
		}
		if cell.Outputs != nil {
			cellMap["outputs"] = cell.Outputs
		}
		if cell.Metadata != nil {
			cellMap["metadata"] = cell.Metadata
		}
		cells = append(cells, cellMap)
	}

	// Serialize cells to JSON to enforce the size cap (preserves batch-99
	// behavior even after we switched the user-visible Output to wrapped
	// cell text).
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

	// Normalize each cell: wrapped text + extracted images.
	relativePath := platformfs.ToRelativePath(filePath, workingDir)
	cellTexts := make([]string, 0, len(notebook.Cells))
	var allImages []coretool.ImageData
	for index, cell := range notebook.Cells {
		text, images := processNotebookCell(cell, index, relativePath)
		cellTexts = append(cellTexts, text)
		allImages = append(allImages, images...)
	}

	// Compose the final user-visible Output by concatenating cell wrappers.
	// A separating newline keeps each cell readable when the model renders
	// the tool_result text.
	composedText := strings.Join(cellTexts, "\n")

	// Validate token count against the wrapped output (matches the bytes the
	// model actually sees).
	if err := validateContentTokens(composedText, "ipynb", t.maxTokens); err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	// Get file modification time for read state.
	info, err := t.fs.Stat(filePath)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("Failed to stat notebook file: %v", err)}, nil
	}

	output := NotebookOutput{
		Type:     "notebook",
		FilePath: relativePath,
		Cells:    cells,
	}

	result := coretool.Result{
		Output: composedText,
		Meta: map[string]any{
			"data":       output,
			"read_state": buildReadStateSnapshot(filePath, info.ModTime(), 1, 0, time.Now()),
		},
	}
	if len(allImages) > 0 {
		result.Meta["images"] = allImages
	}

	return result, nil
}

// processNotebookCell normalizes one cell into its wrapped text representation
// and the image outputs found within. The text uses a `<cell id="X">…</cell id="X">`
// wrapper around the source and any rendered output text. When the cell's
// combined output size (text bytes + image base64 bytes) exceeds
// largeOutputThreshold, outputs are replaced with a stub and images are
// dropped to avoid blowing up the conversation history.
func processNotebookCell(cell NotebookCell, index int, filePath string) (string, []coretool.ImageData) {
	cellID := cell.ID
	if cellID == "" {
		cellID = fmt.Sprintf("cell-%d", index)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<cell id=%q>", cellID)
	if cell.CellType != "" && cell.CellType != "code" {
		fmt.Fprintf(&b, "<cell_type>%s</cell_type>", cell.CellType)
	}
	fmt.Fprintf(&b, "<source>%s</source>", strings.Join(cell.Source, ""))

	var outputsBuilder strings.Builder
	var cellImages []coretool.ImageData
	var totalSize int
	truncated := false
	for _, raw := range cell.Outputs {
		outMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		text, image := processOutput(outMap)
		segmentSize := len(text)
		if image != nil {
			segmentSize += len(image.Base64)
		}
		if totalSize+segmentSize > largeOutputThreshold {
			truncated = true
			break
		}
		totalSize += segmentSize
		outputsBuilder.WriteString(text)
		if image != nil {
			cellImages = append(cellImages, *image)
		}
	}

	if truncated {
		fmt.Fprintf(&b,
			"<output>Outputs are too large to include. Use BashTool with: cat %q | jq '.cells[%d].outputs'</output>",
			filePath, index,
		)
		// Discard images for the truncated cell so the conversation history
		// stays bounded.
		cellImages = nil
	} else if outputsBuilder.Len() > 0 {
		fmt.Fprintf(&b, "<output>%s</output>", outputsBuilder.String())
	}

	fmt.Fprintf(&b, "</cell id=%q>", cellID)
	return b.String(), cellImages
}

// processOutput converts a single Jupyter cell output into its text
// representation and, when applicable, an extracted image (image/png
// preferred over image/jpeg). Supports the four nbformat 4 output types:
// stream, execute_result, display_data, error.
func processOutput(output map[string]any) (string, *coretool.ImageData) {
	outputType, _ := output["output_type"].(string)
	switch outputType {
	case "stream":
		return "\n" + extractMultilineString(output["text"]), nil
	case "execute_result", "display_data":
		data, ok := output["data"].(map[string]any)
		if !ok {
			return "", nil
		}
		text := ""
		if t, ok := data["text/plain"]; ok {
			text = "\n" + extractMultilineString(t)
		}
		// Prefer PNG over JPEG to mirror TS-side priority.
		if rawImg, ok := data["image/png"].(string); ok {
			return text, &coretool.ImageData{
				MediaType: "image/png",
				Base64:    cleanBase64(rawImg),
			}
		}
		if rawImg, ok := data["image/jpeg"].(string); ok {
			return text, &coretool.ImageData{
				MediaType: "image/jpeg",
				Base64:    cleanBase64(rawImg),
			}
		}
		return text, nil
	case "error":
		ename, _ := output["ename"].(string)
		evalue, _ := output["evalue"].(string)
		traceback := extractMultilineString(output["traceback"])
		return fmt.Sprintf("\n%s: %s\n%s", ename, evalue, traceback), nil
	}
	return "", nil
}

// extractMultilineString normalizes notebook fields that may carry their
// payload as a string or a string array per the nbformat spec (e.g., source,
// stream.text, display_data.data["text/plain"], error.traceback).
func extractMultilineString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []string:
		return strings.Join(s, "")
	case []any:
		var b strings.Builder
		for _, item := range s {
			if str, ok := item.(string); ok {
				b.WriteString(str)
			}
		}
		return b.String()
	}
	return ""
}

// cleanBase64 strips whitespace from a base64 string before forwarding it to
// the runtime engine. Jupyter notebooks frequently wrap base64 image data
// across multiple lines and Anthropic rejects inputs containing whitespace.
func cleanBase64(s string) string {
	return notebookOutputWhitespaceRe.ReplaceAllString(s, "")
}
