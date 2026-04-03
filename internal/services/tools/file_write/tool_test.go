package file_write

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

// TestToolInvokeCreatesFile verifies the first-batch FileWriteTool can create a new file and its parent directories.
func TestToolInvokeCreatesFile(t *testing.T) {
	projectDir := t.TempDir()

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path": "docs/output.txt",
			"content":   "hello\nworld\n",
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
	if result.Output != "Created file: docs/output.txt" {
		t.Fatalf("Invoke() output = %q", result.Output)
	}

	writtenContent, readErr := os.ReadFile(filepath.Join(projectDir, "docs", "output.txt"))
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(writtenContent) != "hello\nworld\n" {
		t.Fatalf("written content = %q", string(writtenContent))
	}

	data, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatalf("Invoke() meta data type = %T", result.Meta["data"])
	}
	if data.Type != "create" || data.FilePath != "docs/output.txt" {
		t.Fatalf("Invoke() metadata = %#v", data)
	}
	if data.OriginalContent != nil {
		t.Fatalf("Invoke() original content = %v, want nil", *data.OriginalContent)
	}
	if len(data.StructuredPatch) != 1 {
		t.Fatalf("Invoke() structured patch = %#v, want one hunk", data.StructuredPatch)
	}
	if data.StructuredPatch[0].NewLines != 2 {
		t.Fatalf("Invoke() structured patch = %#v", data.StructuredPatch)
	}
}

// TestToolInvokeUpdatesExistingFile verifies existing file content is replaced and surfaced in metadata.
func TestToolInvokeUpdatesExistingFile(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "before\n")

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path": "notes.txt",
			"content":   "after\n",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: time.Unix(90, 0),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}
	if result.Output != "Updated file: notes.txt" {
		t.Fatalf("Invoke() output = %q", result.Output)
	}

	data := result.Meta["data"].(Output)
	if data.Type != "update" || data.OriginalContent == nil || *data.OriginalContent != "before\n" {
		t.Fatalf("Invoke() metadata = %#v", data)
	}
	if len(data.StructuredPatch) != 1 {
		t.Fatalf("Invoke() structured patch = %#v, want one hunk", data.StructuredPatch)
	}
	if got := data.StructuredPatch[0].Lines; len(got) != 2 || got[0] != "-before" || got[1] != "+after" {
		t.Fatalf("Invoke() structured patch lines = %#v", got)
	}

	writtenContent, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(writtenContent) != "after\n" {
		t.Fatalf("written content = %q", string(writtenContent))
	}
}

// TestToolInvokeRejectsOverwriteWithoutReadState verifies existing files must be read before full overwrite.
func TestToolInvokeRejectsOverwriteWithoutReadState(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "before\n")

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path": "notes.txt",
			"content":   "after\n",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != unreadBeforeWriteError {
		t.Fatalf("Invoke() result.Error = %q, want %q", result.Error, unreadBeforeWriteError)
	}

	writtenContent, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(writtenContent) != "before\n" {
		t.Fatalf("written content = %q, want %q", string(writtenContent), "before\n")
	}
}

// TestToolInvokeRejectsOverwriteAfterPartialRead verifies partial reads do not satisfy the overwrite safety guard.
func TestToolInvokeRejectsOverwriteAfterPartialRead(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "before\n")

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path": "notes.txt",
			"content":   "after\n",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: time.Unix(90, 0),
						IsPartial:       true,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != unreadBeforeWriteError {
		t.Fatalf("Invoke() result.Error = %q, want %q", result.Error, unreadBeforeWriteError)
	}
}

// TestToolInvokeRejectsWriteWithoutPermission verifies the migrated tool reuses the minimal write-permission gate.
func TestToolInvokeRejectsWriteWithoutPermission(t *testing.T) {
	projectDir := t.TempDir()

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	_, err = tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path": "secret.txt",
			"content":   "secret\n",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err == nil {
		t.Fatalf("Invoke() error = nil, want permission error")
	}

	var permissionErr *corepermission.PermissionError
	if !errors.As(err, &permissionErr) {
		t.Fatalf("Invoke() error = %T, want *PermissionError", err)
	}
	if permissionErr.Decision != corepermission.DecisionAsk {
		t.Fatalf("Invoke() decision = %q, want %q", permissionErr.Decision, corepermission.DecisionAsk)
	}
}

// TestToolInvokeRejectsDirectory verifies handled directory validation failures are surfaced in Result.Error.
func TestToolInvokeRejectsDirectory(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectDir, "docs"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path": "docs",
			"content":   "ignored",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "Path is a directory, not a file: docs" {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}
}

// newAllowWritePolicy constructs a minimal policy that allows writes inside one workspace.
func newAllowWritePolicy(workspace string) (*corepermission.FilesystemPolicy, error) {
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

// mustWriteFile creates a file and fails the test immediately on errors to keep setup terse.
func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
