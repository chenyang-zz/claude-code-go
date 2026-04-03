package glob

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

// TestToolInvokeMatchesFiles verifies the first-batch GlobTool path search, ordering, and truncation shape.
func TestToolInvokeMatchesFiles(t *testing.T) {
	projectDir := t.TempDir()
	mustWriteFile(t, filepath.Join(projectDir, "a.go"), "package main\n")
	time.Sleep(10 * time.Millisecond)
	mustMkdirAll(t, filepath.Join(projectDir, "nested"))
	mustWriteFile(t, filepath.Join(projectDir, "nested", "b.go"), "package nested\n")
	mustWriteFile(t, filepath.Join(projectDir, "nested", "c.txt"), "text\n")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)
	tool.maxResults = 1

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"pattern": "**/*.go",
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

	if result.Output != "a.go\n(Results are truncated. Consider using a more specific path or pattern.)" {
		t.Fatalf("Invoke() output = %q", result.Output)
	}

	data, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatalf("Invoke() meta data type = %T", result.Meta["data"])
	}

	if data.NumFiles != 1 {
		t.Fatalf("Invoke() numFiles = %d, want 1", data.NumFiles)
	}
	if !data.Truncated {
		t.Fatalf("Invoke() truncated = false, want true")
	}
	if len(data.Filenames) != 1 || data.Filenames[0] != "a.go" {
		t.Fatalf("Invoke() filenames = %#v", data.Filenames)
	}
}

// TestToolInvokeSupportsPathOverride verifies explicit search roots are honored and relativized against the caller cwd.
func TestToolInvokeSupportsPathOverride(t *testing.T) {
	projectDir := t.TempDir()
	searchDir := filepath.Join(projectDir, "src")
	mustMkdirAll(t, searchDir)
	mustWriteFile(t, filepath.Join(searchDir, "main.ts"), "export {}\n")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"pattern": "*.ts",
			"path":    "src",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}

	data := result.Meta["data"].(Output)
	if len(data.Filenames) != 1 || data.Filenames[0] != filepath.Join("src", "main.ts") {
		t.Fatalf("Invoke() filenames = %#v", data.Filenames)
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
			"pattern": "*.txt",
			"path":    outsideDir,
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

// TestToolInvokeRejectsInvalidDirectory verifies handled directory validation failures are surfaced in Result.Error.
func TestToolInvokeRejectsInvalidDirectory(t *testing.T) {
	projectDir := t.TempDir()

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"pattern": "*.go",
			"path":    "missing",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "Directory does not exist: missing" {
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
