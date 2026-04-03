package file_read

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

// TestToolInvokeReadsTextFile verifies the first-batch FileReadTool returns numbered text lines and structured metadata.
func TestToolInvokeReadsTextFile(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "main.go")
	mustWriteFile(t, filePath, "package main\n\nfunc main() {}\n")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path": "main.go",
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

	wantOutput := "     1\tpackage main\n     2\t\n     3\tfunc main() {}"
	if result.Output != wantOutput {
		t.Fatalf("Invoke() output = %q, want %q", result.Output, wantOutput)
	}

	data, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatalf("Invoke() meta data type = %T", result.Meta["data"])
	}
	if data.FilePath != "main.go" {
		t.Fatalf("Invoke() filePath = %q, want %q", data.FilePath, "main.go")
	}
	if data.NumLines != 3 || data.StartLine != 1 || data.TotalLines != 3 {
		t.Fatalf("Invoke() metadata = %#v", data)
	}
}

// TestToolInvokeSupportsOffsetAndLimit verifies callers can read a targeted line window.
func TestToolInvokeSupportsOffsetAndLimit(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "a\nb\nc\nd\n")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path": "notes.txt",
			"offset":    2,
			"limit":     2,
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}

	wantOutput := "     2\tb\n     3\tc"
	if result.Output != wantOutput {
		t.Fatalf("Invoke() output = %q, want %q", result.Output, wantOutput)
	}

	data := result.Meta["data"].(Output)
	if data.Content != "b\nc" || data.NumLines != 2 || data.StartLine != 2 || data.TotalLines != 4 {
		t.Fatalf("Invoke() metadata = %#v", data)
	}
}

// TestToolInvokeRejectsLargeFullFile verifies full reads are blocked when the file exceeds the first-batch size cap.
func TestToolInvokeRejectsLargeFullFile(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "large.txt")
	mustWriteFile(t, filePath, strings.Repeat("abcdef\n", 8))

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)
	tool.maxFileSizeBytes = 16

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path": "large.txt",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "exceeds maximum allowed size") {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}
}

// TestToolInvokeRejectsReadOutsideWorkingDir verifies the migrated tool reuses the minimal permission gate.
func TestToolInvokeRejectsReadOutsideWorkingDir(t *testing.T) {
	workingDir := t.TempDir()
	outsideDir := t.TempDir()
	mustWriteFile(t, filepath.Join(outsideDir, "secret.txt"), "secret\n")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	_, err = tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path": filepath.Join(outsideDir, "secret.txt"),
		},
		Context: coretool.UseContext{
			WorkingDir: workingDir,
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
	mustMkdirAll(t, filepath.Join(projectDir, "docs"))

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path": "docs",
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

// TestToolInvokeRejectsBinaryContent verifies the first-batch text-only scope rejects non-UTF-8 content.
func TestToolInvokeRejectsBinaryContent(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "image.bin")
	if err := os.WriteFile(filePath, []byte{0xff, 0xfe, 0xfd}, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", filePath, err)
	}

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path": "image.bin",
			"limit":     1,
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "This tool cannot read binary files. The file appears to contain non-text content." {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}
}

// mustWriteFile creates a file and fails the test immediately on errors to keep setup terse.
func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

// mustMkdirAll creates one directory tree for test fixtures.
func mustMkdirAll(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}
