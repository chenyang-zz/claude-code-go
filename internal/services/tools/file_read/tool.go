package file_read

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
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
)

// Tool implements the first-batch text-only FileReadTool.
type Tool struct {
	// fs provides metadata lookups and streaming file access.
	fs platformfs.FileSystem
	// policy performs the minimal read-permission check before any file access.
	policy *corepermission.FilesystemPolicy
	// maxFileSizeBytes caps full-file reads when the caller does not request a limited range.
	maxFileSizeBytes int64
}

// Input is the typed request payload accepted by the migrated FileReadTool.
type Input struct {
	// FilePath stores the file path requested by the caller.
	FilePath string `json:"file_path"`
	// Offset stores the 1-based starting line number.
	Offset int `json:"offset,omitempty"`
	// Limit optionally caps how many lines are returned.
	Limit int `json:"limit,omitempty"`
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
	return &Tool{
		fs:               fs,
		policy:           policy,
		maxFileSizeBytes: defaultMaxFileSizeBytes,
	}
}

// Name returns the stable registration name for this tool.
func (t *Tool) Name() string {
	return Name
}

// Description returns the summary exposed to callers and tests.
func (t *Tool) Description() string {
	return "Read a text file from the local filesystem with optional offset and limit parameters."
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
		return coretool.Result{Error: fmt.Sprintf("File does not exist: %s", input.FilePath)}, nil
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

	return coretool.Result{
		Output: renderOutput(output),
		Meta: map[string]any{
			"data":       output,
			"read_state": buildReadStateSnapshot(filePath, info.ModTime(), offset, input.Limit, time.Now()),
		},
	}, nil
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
func renderOutput(output Output) string {
	if output.Content == "" {
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
	return strings.Join(rendered, "\n")
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
