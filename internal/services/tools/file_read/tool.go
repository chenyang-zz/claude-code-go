package file_read

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registry identifier used by the migrated FileReadTool.
	Name = "Read"
	// defaultMaxFileSizeBytes applies the first-batch large-file protection for full-file reads.
	defaultMaxFileSizeBytes = 256 * 1024
	// defaultOffset mirrors the user-facing 1-based default start line.
	defaultOffset = 1
	// fileUnchangedStub mirrors the source tool's duplicate-read reminder.
	fileUnchangedStub = "File unchanged since last read. The content from the earlier Read tool_result in this conversation is still current - refer to that instead of re-reading."
	// pdfMaxPagesPerRead limits how many pages can be extracted in a single request.
	pdfMaxPagesPerRead = 25
	// pdfAtMentionInlineThreshold is the page count above which full PDF reads are rejected.
	pdfAtMentionInlineThreshold = 100
	// pdfExtractSizeThreshold is the size above which PDFs are page-extracted instead of inline read.
	pdfExtractSizeThreshold = 5 * 1024 * 1024
)

// Tool implements the first-batch text-only FileReadTool.
type Tool struct {
	// fs provides metadata lookups and streaming file access.
	fs platformfs.FileSystem
	// policy performs the minimal read-permission check before any file access.
	policy *corepermission.FilesystemPolicy
	// maxFileSizeBytes caps full-file reads when the caller does not request a limited range.
	maxFileSizeBytes int64
	// maxTokens caps the estimated token count of returned content.
	maxTokens int
	// freshnessTracker manages per-file change-detection state.
	freshnessTracker *FreshnessTracker
	// freshnessWatcher watches files for external changes via fsnotify.
	freshnessWatcher *FileFreshnessWatcher
}

// Input is the typed request payload accepted by the migrated FileReadTool.
type Input struct {
	// FilePath stores the file path requested by the caller.
	FilePath string `json:"file_path"`
	// Offset stores the 1-based starting line number.
	Offset int `json:"offset,omitempty"`
	// Limit optionally caps how many lines are returned.
	Limit int `json:"limit,omitempty"`
	// Pages optionally specifies a page range for PDF files (e.g., "1-5", "3").
	Pages string `json:"pages,omitempty"`
}

// Output is the structured text read result returned in tool metadata.
type Output struct {
	// FilePath stores the caller-facing path of the file that was read.
	FilePath string `json:"filePath"`
	// Content stores the selected text slice without line numbers.
	Content string `json:"content"`
	// NumLines reports how many lines are included in Content.
	NumLines int `json:"numLines"`
	// StartLine reports the 1-based line number of the first returned line.
	StartLine int `json:"startLine"`
	// TotalLines reports the total number of lines observed in the file.
	TotalLines int `json:"totalLines"`
}

// UnchangedOutput is the structured metadata returned when the file content matches an earlier full read.
type UnchangedOutput struct {
	// Type identifies the source-aligned file_unchanged result branch.
	Type string `json:"type"`
	// FilePath stores the caller-facing path for the unchanged file.
	FilePath string `json:"filePath"`
}

// ImageOutput is the structured metadata returned for image file reads.
type ImageOutput struct {
	// Type identifies the image result branch.
	Type string `json:"type"`
	// Base64 stores the base64-encoded image data.
	Base64 string `json:"base64"`
	// MediaType stores the MIME type of the image (e.g., image/jpeg).
	MediaType string `json:"media_type"`
	// OriginalSize stores the original file size in bytes.
	OriginalSize int `json:"originalSize"`
	// OriginalWidth stores the original image width in pixels (optional).
	OriginalWidth int `json:"originalWidth,omitempty"`
	// OriginalHeight stores the original image height in pixels (optional).
	OriginalHeight int `json:"originalHeight,omitempty"`
	// DisplayWidth stores the displayed width after resizing (optional).
	DisplayWidth int `json:"displayWidth,omitempty"`
	// DisplayHeight stores the displayed height after resizing (optional).
	DisplayHeight int `json:"displayHeight,omitempty"`
}

// PDFOutput is the structured metadata returned for PDF file reads.
type PDFOutput struct {
	// Type identifies the pdf result branch.
	Type string `json:"type"`
	// FilePath stores the caller-facing path of the PDF file.
	FilePath string `json:"filePath"`
	// Base64 stores the base64-encoded PDF data.
	Base64 string `json:"base64"`
	// OriginalSize stores the original file size in bytes.
	OriginalSize int `json:"originalSize"`
}

// NotebookOutput is the structured metadata returned for notebook file reads.
type NotebookOutput struct {
	// Type identifies the notebook result branch.
	Type string `json:"type"`
	// FilePath stores the caller-facing path of the notebook file.
	FilePath string `json:"filePath"`
	// Cells stores the notebook cells as a JSON-serializable array.
	Cells []any `json:"cells"`
}

// PartsOutput is the structured metadata returned for PDF page extraction.
type PartsOutput struct {
	// Type identifies the parts result branch.
	Type string `json:"type"`
	// FilePath stores the caller-facing path of the PDF file.
	FilePath string `json:"filePath"`
	// OriginalSize stores the original file size in bytes.
	OriginalSize int `json:"originalSize"`
	// Count stores the number of pages extracted.
	Count int `json:"count"`
	// OutputDir stores the directory containing extracted page images.
	OutputDir string `json:"outputDir"`
}

// lineReadResult keeps the internal streaming result before caller-facing formatting.
type lineReadResult struct {
	// content stores the selected text slice without presentation markup.
	content string
	// lineCount reports how many lines were selected.
	lineCount int
	// totalLines reports how many lines exist in the file.
	totalLines int
}

// NewTool constructs a FileReadTool with explicit host dependencies.
func NewTool(fs platformfs.FileSystem, policy *corepermission.FilesystemPolicy) *Tool {
	limits := getDefaultFileReadingLimits()
	tracker := NewFreshnessTracker()
	return &Tool{
		fs:               fs,
		policy:           policy,
		maxFileSizeBytes: limits.MaxSizeBytes,
		maxTokens:        limits.MaxTokens,
		freshnessTracker: tracker,
		freshnessWatcher: NewFileFreshnessWatcher(tracker),
	}
}

// Name returns the stable registration name for this tool.
func (t *Tool) Name() string {
	return Name
}

// Description returns the summary exposed to callers and tests.
func (t *Tool) Description() string {
	return `Reads a file from the local filesystem. You can access any file directly by using this tool.
Assume this tool is able to read all files on the machine. If the User provides a path to a file assume that path is valid. It is okay to read a file that does not exist; an error will be returned.

Usage:
- The file_path parameter must be an absolute path, not a relative path
- By default, it reads up to 2000 lines starting from the beginning of the file
- You can optionally specify a line offset and limit (especially handy for long files), but it's recommended to read the whole file by not providing these parameters
- Results are returned using cat -n format, with line numbers starting at 1
- This tool allows Claude Code to read images (eg PNG, JPG, etc). When reading an image file the contents are presented visually as Claude Code is a multimodal LLM.
- This tool can read PDF files (.pdf). For large PDFs (more than 10 pages), you MUST provide the pages parameter to read specific page ranges (e.g., pages: "1-5"). Reading a large PDF without the pages parameter will fail. Maximum 20 pages per request.
- This tool can read Jupyter notebooks (.ipynb files) and returns all cells with their outputs, combining code, text, and visualizations.
- This tool can only read files, not directories. To read a directory, use an ls command via the Bash tool.
- You will regularly be asked to read screenshots. If the user provides a path to a screenshot, ALWAYS use this tool to view the file at the path. This tool will work with all temporary file paths.
- If you read a file that exists but has empty contents you will receive a system reminder warning in place of file contents.`
}

// InputSchema returns the FileReadTool input contract exposed to model providers.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
}

// IsReadOnly reports that file reading never mutates external state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that independent file reads may run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// Invoke validates input, enforces read permissions, reads one text file, and returns line-numbered output.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("file read tool: nil receiver")
	}
	if t.fs == nil {
		return coretool.Result{}, fmt.Errorf("file read tool: filesystem is not configured")
	}
	if t.policy == nil {
		return coretool.Result{}, fmt.Errorf("file read tool: permission policy is not configured")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	offset := input.Offset
	if offset == 0 {
		offset = defaultOffset
	}
	if offset < 0 {
		return coretool.Result{Error: "Offset must be greater than or equal to 0"}, nil
	}
	if input.Limit < 0 {
		return coretool.Result{Error: "Limit must be greater than or equal to 0"}, nil
	}

	filePath, err := platformfs.ExpandPath(input.FilePath, call.Context.WorkingDir)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("file read tool: expand path: %v", err)}, nil
	}

	info, err := t.fs.Stat(filePath)
	if err != nil {
		// Try alternate screenshot path for macOS screenshots with different space characters.
		if altPath, ok := resolveScreenshotPath(t.fs, filePath); ok && altPath != filePath {
			filePath = altPath
			info, err = t.fs.Stat(filePath)
		}
		if err != nil {
			return coretool.Result{Error: fmt.Sprintf("File does not exist: %s", input.FilePath)}, nil
		}
	}
	if info.IsDir() {
		return coretool.Result{Error: fmt.Sprintf("Path is a directory, not a file: %s", input.FilePath)}, nil
	}

	evaluation := t.policy.CheckReadPermissionForTool(ctx, t.Name(), filePath, call.Context.WorkingDir)
	if err := evaluation.ToError(corepermission.FilesystemRequest{
		ToolName:   t.Name(),
		Path:       filePath,
		WorkingDir: call.Context.WorkingDir,
		Access:     corepermission.AccessRead,
	}); err != nil {
		return coretool.Result{}, err
	}

	ext := strings.TrimPrefix(filepath.Ext(filePath), ".")

	logger.DebugCF("file_read_tool", "starting file read", map[string]any{
		"file_path":   filePath,
		"offset":      offset,
		"limit":       input.Limit,
		"working_dir": call.Context.WorkingDir,
	})

	if unchangedResult, ok := buildFileUnchangedResult(call.Context, filePath, offset, input.Limit, info.ModTime()); ok {
		logger.DebugCF("file_read_tool", "skipping duplicate file read", map[string]any{
			"file_path":   filePath,
			"offset":      offset,
			"limit":       input.Limit,
			"working_dir": call.Context.WorkingDir,
		})
		return unchangedResult, nil
	}

	// Dispatch by file extension for multimedia support.
	if isImageExtension(ext) {
		return t.readImage(ctx, filePath, info.Size(), call.Context.WorkingDir)
	}
	if isNotebookExtension(ext) {
		return t.readNotebookFile(ctx, filePath, info.Size(), call.Context.WorkingDir)
	}
	if isPDFExtension(ext) {
		return t.readPDF(ctx, filePath, info.Size(), input.Pages, call.Context.WorkingDir)
	}

	if input.Limit == 0 && info.Size() > t.effectiveMaxFileSizeBytes() {
		return coretool.Result{
			Error: fmt.Sprintf(
				"File content (%s) exceeds maximum allowed size (%s). Use offset and limit parameters to read specific portions of the file, or search for specific content instead of reading the whole file.",
				formatByteSize(info.Size()),
				formatByteSize(t.effectiveMaxFileSizeBytes()),
			),
		}, nil
	}

	readResult, err := t.readTextRange(ctx, filePath, offset, input.Limit)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	if err := validateContentTokens(readResult.content, ext, t.maxTokens); err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	output := Output{
		FilePath:   platformfs.ToRelativePath(filePath, call.Context.WorkingDir),
		Content:    readResult.content,
		NumLines:   readResult.lineCount,
		StartLine:  offset,
		TotalLines: readResult.totalLines,
	}

	logger.DebugCF("file_read_tool", "file read finished", map[string]any{
		"file_path":   filePath,
		"num_lines":   output.NumLines,
		"start_line":  output.StartLine,
		"total_lines": output.TotalLines,
	})
	triggerMeta := buildReadPathTriggerMeta(filePath, call.Context.WorkingDir)

	// Ensure the freshness watcher is started.
	if t.freshnessWatcher != nil {
		_ = t.freshnessWatcher.Start()
	}

	// Record the file read for freshness tracking.
	t.freshnessTracker.RecordRead(filePath, info.ModTime())

	// Register the file for external change monitoring.
	if t.freshnessWatcher != nil {
		t.freshnessWatcher.AddFile(filePath)
	}

	// Build freshness reminders: memory files get their own reminder, all files get change detection.
	prefix := buildFreshnessPrefix(t.freshnessTracker, filePath, info.ModTime())

	// Notify read listeners (text-only reads; Magic Docs only cares about markdown).
	if len(readListeners) > 0 {
		notifyReadListeners(filePath, readResult.content)
	}

	return coretool.Result{
		Output: renderOutput(output, prefix),
		Meta: map[string]any{
			"data":                              output,
			"read_state":                        buildReadStateSnapshot(filePath, info.ModTime(), offset, input.Limit, time.Now()),
			"nested_memory_attachment_triggers": triggerMeta.NestedMemoryAttachmentTriggers,
			"dynamic_skill_dir_triggers":        triggerMeta.DynamicSkillDirTriggers,
		},
	}, nil
}

// buildFreshnessPrefix composes the memory freshness reminder and the generic change-detection reminder.
func buildFreshnessPrefix(tracker *FreshnessTracker, filePath string, modTime time.Time) string {
	var parts []string
	if mem := memoryFreshnessReminder(filePath, modTime); mem != "" {
		parts = append(parts, mem)
	}
	if change := tracker.BuildReminder(filePath, modTime); change != "" {
		parts = append(parts, change)
	}
	return strings.Join(parts, "")
}

// buildFileUnchangedResult reuses prior full-read state to suppress duplicate text payloads.
func buildFileUnchangedResult(context coretool.UseContext, filePath string, offset int, limit int, currentModTime time.Time) (coretool.Result, bool) {
	state, ok := context.LookupReadState(filePath)
	if !ok || state.IsPartial || state.ReadOffset == 0 {
		return coretool.Result{}, false
	}
	if state.ReadOffset != offset || state.ReadLimit != limit {
		return coretool.Result{}, false
	}
	if state.ObservedModTime.IsZero() || !state.ObservedModTime.Equal(currentModTime) {
		return coretool.Result{}, false
	}

	output := UnchangedOutput{
		Type:     "file_unchanged",
		FilePath: platformfs.ToRelativePath(filePath, context.WorkingDir),
	}

	return coretool.Result{
		Output: fileUnchangedStub,
		Meta: map[string]any{
			"data": output,
		},
	}, true
}

// inputSchema declares the FileReadTool input contract for the first migration pass.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"file_path": {
				Type:        coretool.ValueKindString,
				Description: "The file path to read.",
				Required:    true,
			},
			"offset": {
				Type:        coretool.ValueKindInteger,
				Description: "The 1-based line number to start reading from.",
			},
			"limit": {
				Type:        coretool.ValueKindInteger,
				Description: "The number of lines to read.",
			},
			"pages": {
				Type:        coretool.ValueKindString,
				Description: "Page range for PDF files (e.g., \"1-5\", \"3\"). Only applicable to PDF files.",
			},
		},
	}
}

// readTextRange streams one text file and keeps only the requested line window.
func (t *Tool) readTextRange(ctx context.Context, filePath string, offset int, limit int) (lineReadResult, error) {
	reader, err := t.fs.OpenRead(filePath)
	if err != nil {
		return lineReadResult{}, err
	}
	defer func() {
		_ = reader.Close()
	}()

	buffered := bufio.NewReader(reader)
	selectedLines := make([]string, 0, maxInt(limit, 16))
	currentLine := 1
	startLine := max(offset, 1)
	endLine := -1
	if limit > 0 {
		endLine = startLine + limit - 1
	}

	for {
		if err := ctx.Err(); err != nil {
			return lineReadResult{}, err
		}

		lineBytes, readErr := buffered.ReadBytes('\n')
		if len(lineBytes) > 0 {
			line := normalizeLine(lineBytes, currentLine == 1)
			if currentLine >= startLine && (endLine == -1 || currentLine <= endLine) {
				if !utf8.ValidString(line) {
					return lineReadResult{}, fmt.Errorf("This tool cannot read binary files. The file appears to contain non-text content.")
				}
				selectedLines = append(selectedLines, line)
			}
			currentLine++
		}

		if readErr == nil {
			continue
		}
		if readErr == io.EOF {
			break
		}
		return lineReadResult{}, readErr
	}

	totalLines := currentLine - 1

	return lineReadResult{
		content:    strings.Join(selectedLines, "\n"),
		lineCount:  len(selectedLines),
		totalLines: totalLines,
	}, nil
}

// effectiveMaxFileSizeBytes returns the configured size cap while preserving a sane default.
func (t *Tool) effectiveMaxFileSizeBytes() int64 {
	if t.maxFileSizeBytes > 0 {
		return t.maxFileSizeBytes
	}
	return defaultMaxFileSizeBytes
}

// renderOutput converts the structured read result into the minimal caller-facing text payload.
func renderOutput(output Output, prefix string) string {
	if output.Content == "" {
		if prefix != "" {
			return prefix + renderOutput(output, "")
		}
		if output.TotalLines == 0 {
			return "<system-reminder>Warning: the file exists but the contents are empty.</system-reminder>"
		}
		return fmt.Sprintf("<system-reminder>Warning: the file exists but is shorter than the provided offset (%d). The file has %d lines.</system-reminder>", output.StartLine, output.TotalLines)
	}

	lines := strings.Split(output.Content, "\n")
	rendered := make([]string, 0, len(lines))
	for index, line := range lines {
		rendered = append(rendered, fmt.Sprintf("%6d\t%s", output.StartLine+index, line))
	}
	result := strings.Join(rendered, "\n")
	if prefix == "" {
		return result
	}
	return prefix + result
}

// normalizeLine trims one streamed line into the same content shape expected by the line-number formatter.
func normalizeLine(line []byte, stripBOM bool) string {
	trimmed := bytes.TrimSuffix(line, []byte("\n"))
	trimmed = bytes.TrimSuffix(trimmed, []byte("\r"))
	if stripBOM {
		trimmed = bytes.TrimPrefix(trimmed, []byte{0xEF, 0xBB, 0xBF})
	}
	return string(trimmed)
}

// formatByteSize keeps large-file errors aligned with the source tool wording.
func formatByteSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	divisor := int64(unit)
	suffix := "KB"
	if size >= unit*unit {
		divisor = unit * unit
		suffix = "MB"
	}

	value := float64(size) / float64(divisor)
	if value == float64(int64(value)) {
		return fmt.Sprintf("%.0f %s", value, suffix)
	}
	return fmt.Sprintf("%.1f %s", value, suffix)
}

// maxInt keeps small slice-capacity setup readable without dragging in extra helpers.
func maxInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// isImageExtension reports whether ext is a supported image format.
func isImageExtension(ext string) bool {
	switch ext {
	case "png", "jpg", "jpeg", "gif", "webp":
		return true
	}
	return false
}

// isPDFExtension reports whether ext is a PDF file.
func isPDFExtension(ext string) bool {
	return ext == "pdf"
}

// isNotebookExtension reports whether ext is a Jupyter notebook file.
func isNotebookExtension(ext string) bool {
	return ext == "ipynb"
}

// buildReadStateSnapshot converts one successful read into the executor-facing read-state delta.
func buildReadStateSnapshot(path string, observedModTime time.Time, offset int, limit int, readAt time.Time) coretool.ReadStateSnapshot {
	snapshot := coretool.ReadStateSnapshot{}
	if path == "" {
		return snapshot
	}

	snapshot.Files = map[string]coretool.ReadState{
		path: {
			ReadAt:          readAt,
			ObservedModTime: observedModTime,
			ReadOffset:      offset,
			ReadLimit:       limit,
			IsPartial:       offset > defaultOffset || limit > 0,
		},
	}

	return snapshot
}
