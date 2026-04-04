package file_write

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	toolshared "github.com/sheepzhao/claude-code-go/internal/services/tools/shared"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registry identifier used by the migrated FileWriteTool.
	Name = "Write"
	// defaultFilePerm is the fallback file mode for newly created files.
	defaultFilePerm = 0o644
	// defaultDirPerm is the fallback directory mode for parent directories created by the tool.
	defaultDirPerm = 0o755
	// unreadBeforeWriteError mirrors the source tool's rejection when an existing file was not fully read first.
	unreadBeforeWriteError = "File has not been read yet. Read it first before writing to it."
	// modifiedSinceReadError mirrors the source tool's rejection when the file changed after the recorded read.
	modifiedSinceReadError = "File has been modified since read, either by the user or by a linter. Read it again before attempting to write it."
)

// Tool implements the first-batch FileWriteTool.
type Tool struct {
	// fs provides metadata lookups and write operations for one local filesystem target.
	fs platformfs.FileSystem
	// policy performs the minimal write-permission check before mutations happen.
	policy *corepermission.FilesystemPolicy
}

// Input is the typed request payload accepted by the migrated FileWriteTool.
type Input struct {
	// FilePath stores the caller-provided file path to create or overwrite.
	FilePath string `json:"file_path"`
	// Content stores the full replacement file content.
	Content string `json:"content"`
}

// Output is the structured write result returned in tool metadata.
type Output struct {
	// Type reports whether the tool created a new file or updated an existing one.
	Type string `json:"type"`
	// FilePath stores the caller-facing path of the file that was written.
	FilePath string `json:"filePath"`
	// Content stores the final content written to disk.
	Content string `json:"content"`
	// OriginalFile stores the previous file content when an existing file was overwritten.
	OriginalFile *string `json:"originalFile"`
	// StructuredPatch stores the minimal diff payload shared with other write-oriented tools.
	StructuredPatch []toolshared.Hunk `json:"structuredPatch"`
}

// NewTool constructs a FileWriteTool with explicit host dependencies.
func NewTool(fs platformfs.FileSystem, policy *corepermission.FilesystemPolicy) *Tool {
	return &Tool{
		fs:     fs,
		policy: policy,
	}
}

// Name returns the stable registration name for this tool.
func (t *Tool) Name() string {
	return Name
}

// Description returns the summary exposed to callers and tests.
func (t *Tool) Description() string {
	return "Write a file to the local filesystem by creating or replacing its full contents."
}

// InputSchema returns the FileWriteTool input contract exposed to model providers.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
}

// IsReadOnly reports that file writing mutates external state.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that writes to arbitrary files should not be assumed safe to parallelize.
func (t *Tool) IsConcurrencySafe() bool {
	return false
}

// Invoke validates input, enforces write permissions, writes one file, and returns the minimal structured result.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("file write tool: nil receiver")
	}
	if t.fs == nil {
		return coretool.Result{}, fmt.Errorf("file write tool: filesystem is not configured")
	}
	if t.policy == nil {
		return coretool.Result{}, fmt.Errorf("file write tool: permission policy is not configured")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}
	if strings.TrimSpace(input.FilePath) == "" {
		return coretool.Result{Error: "file_path is required"}, nil
	}

	filePath, err := platformfs.ExpandPath(input.FilePath, call.Context.WorkingDir)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("file write tool: expand path: %v", err)}, nil
	}

	evaluation := t.policy.CheckWritePermissionForTool(ctx, t.Name(), filePath, call.Context.WorkingDir)
	if err := evaluation.ToError(corepermission.FilesystemRequest{
		ToolName:   t.Name(),
		Path:       filePath,
		WorkingDir: call.Context.WorkingDir,
		Access:     corepermission.AccessWrite,
	}); err != nil {
		return coretool.Result{}, err
	}

	logger.DebugCF("file_write_tool", "starting file write", map[string]any{
		"file_path":   filePath,
		"working_dir": call.Context.WorkingDir,
		"content_len": len(input.Content),
	})

	writeType := "create"
	filePerm := os.FileMode(defaultFilePerm)
	var originalContent *string

	info, err := t.fs.Stat(filePath)
	switch {
	case err == nil:
		if info.IsDir() {
			return coretool.Result{Error: fmt.Sprintf("Path is a directory, not a file: %s", input.FilePath)}, nil
		}
		if err := validateExistingFileReadState(call.Context, filePath, info.ModTime()); err != nil {
			return coretool.Result{Error: err.Error()}, nil
		}
		writeType = "update"
		filePerm = info.Mode().Perm()

		previousContent, readErr := t.fs.ReadFile(filePath)
		if readErr != nil {
			return coretool.Result{Error: fmt.Sprintf("file write tool: read existing file: %v", readErr)}, nil
		}

		previousContentString := string(previousContent)
		originalContent = &previousContentString
	case platformfs.IsNotExist(err):
		// Creating a new file is allowed to proceed after the target path is confirmed missing.
	default:
		return coretool.Result{Error: fmt.Sprintf("file write tool: inspect target: %v", err)}, nil
	}

	parentDir := filepath.Dir(filePath)
	if err := t.fs.MkdirAll(parentDir, defaultDirPerm); err != nil {
		return coretool.Result{Error: fmt.Sprintf("file write tool: create parent directories: %v", err)}, nil
	}

	if err := t.fs.WriteFile(filePath, []byte(input.Content), filePerm); err != nil {
		return coretool.Result{Error: fmt.Sprintf("file write tool: write file: %v", err)}, nil
	}

	output := Output{
		Type:            writeType,
		FilePath:        platformfs.ToRelativePath(filePath, call.Context.WorkingDir),
		Content:         input.Content,
		OriginalFile:    originalContent,
		StructuredPatch: buildWriteStructuredPatch(writeType, originalContent, input.Content),
	}

	logger.DebugCF("file_write_tool", "file write finished", map[string]any{
		"file_path": filePath,
		"type":      output.Type,
	})

	return coretool.Result{
		Output: renderOutput(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// inputSchema declares the FileWriteTool input contract for the first migration pass.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"file_path": {
				Type:        coretool.ValueKindString,
				Description: "The file path to write.",
				Required:    true,
			},
			"content": {
				Type:        coretool.ValueKindString,
				Description: "The full file content to write.",
				Required:    true,
			},
		},
	}
}

// renderOutput converts the structured write result into the minimal caller-facing text payload.
func renderOutput(output Output) string {
	switch output.Type {
	case "create":
		return fmt.Sprintf("Created file: %s", output.FilePath)
	case "update":
		return fmt.Sprintf("Updated file: %s", output.FilePath)
	default:
		return fmt.Sprintf("Wrote file: %s", output.FilePath)
	}
}

// derefString converts an optional string pointer into a stable concrete value for shared patch generation.
func derefString(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

// buildWriteStructuredPatch keeps create/update patch semantics aligned with the source FileWriteTool.
func buildWriteStructuredPatch(writeType string, originalContent *string, newContent string) []toolshared.Hunk {
	if writeType == "create" {
		return []toolshared.Hunk{}
	}

	return toolshared.BuildStructuredPatch(derefString(originalContent), newContent)
}

// validateExistingFileReadState enforces the minimal "read before overwrite" and drift-protection guards for existing files.
func validateExistingFileReadState(context coretool.UseContext, filePath string, currentModTime time.Time) error {
	state, ok := context.LookupReadState(filePath)
	if !ok || state.IsPartial {
		return fmt.Errorf(unreadBeforeWriteError)
	}
	if !state.ObservedModTime.IsZero() && currentModTime.After(state.ObservedModTime) {
		return fmt.Errorf(modifiedSinceReadError)
	}

	return nil
}
