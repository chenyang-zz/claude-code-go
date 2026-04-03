package file_edit

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	toolshared "github.com/sheepzhao/claude-code-go/internal/services/tools/shared"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registry identifier used by the migrated FileEditTool.
	Name = "Edit"
	// unreadBeforeEditError mirrors the source tool's rejection when an existing file was not fully read first.
	unreadBeforeEditError = "File has not been read yet. Read it first before writing to it."
	// modifiedSinceReadError mirrors the source tool's rejection when the file changed after the recorded read.
	modifiedSinceReadError = "File has been modified since read, either by the user or by a linter. Read it again before attempting to write it."
)

// Tool implements the first-batch FileEditTool.
type Tool struct {
	// fs provides the file read and write operations needed for in-place edits.
	fs platformfs.FileSystem
	// policy performs the minimal write-permission check before mutations happen.
	policy *corepermission.FilesystemPolicy
}

// Input is the typed request payload accepted by the migrated FileEditTool.
type Input struct {
	// FilePath stores the caller-provided file path to edit.
	FilePath string `json:"file_path"`
	// OldString stores the exact text that must be matched before editing.
	OldString string `json:"old_string"`
	// NewString stores the replacement text written back to the file.
	NewString string `json:"new_string"`
	// ReplaceAll controls whether every matching occurrence should be replaced.
	ReplaceAll bool `json:"replace_all,omitempty"`
}

// Output is the structured edit result returned in tool metadata.
type Output struct {
	// FilePath stores the caller-facing path of the edited file.
	FilePath string `json:"filePath"`
	// OldString stores the caller-provided search text.
	OldString string `json:"oldString"`
	// NewString stores the caller-provided replacement text.
	NewString string `json:"newString"`
	// Replacements reports how many occurrences were replaced.
	Replacements int `json:"replacements"`
	// Content stores the final file content after the edit succeeds.
	Content string `json:"content"`
	// OriginalContent stores the pre-edit file content for callers and tests.
	OriginalContent string `json:"originalContent"`
	// StructuredPatch stores the minimal diff payload shared with other write-oriented tools.
	StructuredPatch []toolshared.Hunk `json:"structuredPatch"`
}

// NewTool constructs a FileEditTool with explicit host dependencies.
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
	return "Edit an existing text file by replacing one exact string with another."
}

// IsReadOnly reports that file editing mutates external state.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that concurrent edits to arbitrary files should not be assumed safe.
func (t *Tool) IsConcurrencySafe() bool {
	return false
}

// Invoke validates input, enforces write permissions, applies one exact replacement, and writes the updated file back.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("file edit tool: nil receiver")
	}
	if t.fs == nil {
		return coretool.Result{}, fmt.Errorf("file edit tool: filesystem is not configured")
	}
	if t.policy == nil {
		return coretool.Result{}, fmt.Errorf("file edit tool: permission policy is not configured")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}
	if strings.TrimSpace(input.FilePath) == "" {
		return coretool.Result{Error: "file_path is required"}, nil
	}
	if input.OldString == "" {
		return coretool.Result{Error: "old_string must not be empty"}, nil
	}
	if input.OldString == input.NewString {
		return coretool.Result{Error: "No changes to make: old_string and new_string are exactly the same."}, nil
	}

	filePath, err := platformfs.ExpandPath(input.FilePath, call.Context.WorkingDir)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("file edit tool: expand path: %v", err)}, nil
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

	logger.DebugCF("file_edit_tool", "starting file edit", map[string]any{
		"file_path":   filePath,
		"working_dir": call.Context.WorkingDir,
		"replace_all": input.ReplaceAll,
	})

	info, err := t.fs.Stat(filePath)
	if err != nil {
		if platformfs.IsNotExist(err) {
			return coretool.Result{Error: fmt.Sprintf("File does not exist: %s", input.FilePath)}, nil
		}
		return coretool.Result{Error: fmt.Sprintf("file edit tool: inspect target: %v", err)}, nil
	}
	if info.IsDir() {
		return coretool.Result{Error: fmt.Sprintf("Path is a directory, not a file: %s", input.FilePath)}, nil
	}
	if err := validateExistingFileReadState(call.Context, filePath, info.ModTime()); err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	originalBytes, err := t.fs.ReadFile(filePath)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("file edit tool: read file: %v", err)}, nil
	}
	if !utf8.Valid(originalBytes) {
		return coretool.Result{Error: "This tool cannot edit binary files. The file appears to contain non-text content."}, nil
	}

	originalContent := string(originalBytes)
	matchCount := strings.Count(originalContent, input.OldString)
	if matchCount == 0 {
		return coretool.Result{
			Error: fmt.Sprintf("String to replace not found in file.\nString: %s", input.OldString),
		}, nil
	}
	if matchCount > 1 && !input.ReplaceAll {
		return coretool.Result{
			Error: fmt.Sprintf("Found %d matches of the string to replace, but replace_all is false. To replace all occurrences, set replace_all to true. To replace only one occurrence, please provide more context to uniquely identify the instance.\nString: %s", matchCount, input.OldString),
		}, nil
	}

	updatedContent := originalContent
	replacementCount := 1
	if input.ReplaceAll {
		updatedContent = strings.ReplaceAll(originalContent, input.OldString, input.NewString)
		replacementCount = matchCount
	} else {
		updatedContent = strings.Replace(originalContent, input.OldString, input.NewString, 1)
	}

	if err := t.fs.WriteFile(filePath, []byte(updatedContent), info.Mode().Perm()); err != nil {
		return coretool.Result{Error: fmt.Sprintf("file edit tool: write file: %v", err)}, nil
	}

	output := Output{
		FilePath:        platformfs.ToRelativePath(filePath, call.Context.WorkingDir),
		OldString:       input.OldString,
		NewString:       input.NewString,
		Replacements:    replacementCount,
		Content:         updatedContent,
		OriginalContent: originalContent,
		StructuredPatch: toolshared.BuildStructuredPatch(originalContent, updatedContent),
	}

	logger.DebugCF("file_edit_tool", "file edit finished", map[string]any{
		"file_path":    filePath,
		"replacements": replacementCount,
	})

	return coretool.Result{
		Output: renderOutput(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// inputSchema declares the FileEditTool input contract for the first migration pass.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"file_path": {
				Type:        coretool.ValueKindString,
				Description: "The file path to edit.",
				Required:    true,
			},
			"old_string": {
				Type:        coretool.ValueKindString,
				Description: "The exact string to replace.",
				Required:    true,
			},
			"new_string": {
				Type:        coretool.ValueKindString,
				Description: "The replacement string.",
				Required:    true,
			},
			"replace_all": {
				Type:        coretool.ValueKindBoolean,
				Description: "Whether to replace all occurrences of old_string.",
			},
		},
	}
}

// renderOutput converts the structured edit result into the minimal caller-facing text payload.
func renderOutput(output Output) string {
	return fmt.Sprintf("Updated file: %s (%d replacement)", output.FilePath, output.Replacements)
}

// validateExistingFileReadState enforces the minimal "read before edit" and drift-protection guards for existing files.
func validateExistingFileReadState(context coretool.UseContext, filePath string, currentModTime time.Time) error {
	state, ok := context.LookupReadState(filePath)
	if !ok || state.IsPartial {
		return fmt.Errorf(unreadBeforeEditError)
	}
	if !state.ObservedModTime.IsZero() && currentModTime.After(state.ObservedModTime) {
		return fmt.Errorf(modifiedSinceReadError)
	}

	return nil
}
