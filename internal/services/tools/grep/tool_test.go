package grep

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

// TestToolInvokeMatchesFiles verifies the first-batch GrepTool content search, glob filtering, and ordering.
func TestToolInvokeMatchesFiles(t *testing.T) {
	projectDir := t.TempDir()
	mustWriteFile(t, filepath.Join(projectDir, "skip.txt"), "target text\n")
	mustWriteFile(t, filepath.Join(projectDir, "first.go"), "package main\nconst target = true\n")
	time.Sleep(10 * time.Millisecond)
	mustMkdirAll(t, filepath.Join(projectDir, "nested"))
	mustWriteFile(t, filepath.Join(projectDir, "nested", "second.go"), "package nested\nconst target = true\n")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"pattern": "target",
			"glob":    "*.go",
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

	if result.Output != filepath.Join("nested", "second.go")+"\nfirst.go" {
		t.Fatalf("Invoke() output = %q", result.Output)
	}

	data, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatalf("Invoke() meta data type = %T", result.Meta["data"])
	}

	if data.NumFiles != 2 {
		t.Fatalf("Invoke() numFiles = %d, want 2", data.NumFiles)
	}
	if data.Mode != outputModeFilesWithMatches {
		t.Fatalf("Invoke() mode = %q, want %q", data.Mode, outputModeFilesWithMatches)
	}
	if len(data.Filenames) != 2 || data.Filenames[0] != filepath.Join("nested", "second.go") || data.Filenames[1] != "first.go" {
		t.Fatalf("Invoke() filenames = %#v", data.Filenames)
	}
}

// TestToolInvokeContentMode verifies content output mode returns relativized matching lines.
func TestToolInvokeContentMode(t *testing.T) {
	projectDir := t.TempDir()
	mustMkdirAll(t, filepath.Join(projectDir, "nested"))
	mustWriteFile(t, filepath.Join(projectDir, "main.go"), "package main\nconst target = true\n")
	mustWriteFile(t, filepath.Join(projectDir, "nested", "helper.go"), "package nested\nconst target = true\n")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"pattern":     "\\btarget\\b",
			"glob":        "*.go",
			"output_mode": "content",
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

	data, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatalf("Invoke() meta data type = %T", result.Meta["data"])
	}

	if data.Mode != outputModeContent {
		t.Fatalf("Invoke() mode = %q, want %q", data.Mode, outputModeContent)
	}
	if data.NumLines != 2 {
		t.Fatalf("Invoke() numLines = %d, want 2", data.NumLines)
	}
	if data.Content != "main.go:const target = true\nnested/helper.go:const target = true" {
		t.Fatalf("Invoke() content = %q", data.Content)
	}
	if result.Output != data.Content {
		t.Fatalf("Invoke() output = %q, want content output", result.Output)
	}
}

// TestToolInvokeCountMode verifies count output mode returns relative per-file counts and summary totals.
func TestToolInvokeCountMode(t *testing.T) {
	projectDir := t.TempDir()
	mustMkdirAll(t, filepath.Join(projectDir, "nested"))
	mustWriteFile(t, filepath.Join(projectDir, "main.go"), "package main\nconst target = true\nconst targetAgain = true\n")
	mustWriteFile(t, filepath.Join(projectDir, "nested", "helper.go"), "package nested\nconst target = true\n")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"pattern":     "\\btarget\\b",
			"glob":        "*.go",
			"output_mode": "count",
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

	data, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatalf("Invoke() meta data type = %T", result.Meta["data"])
	}

	if data.Mode != outputModeCount {
		t.Fatalf("Invoke() mode = %q, want %q", data.Mode, outputModeCount)
	}
	if data.NumFiles != 2 {
		t.Fatalf("Invoke() numFiles = %d, want 2", data.NumFiles)
	}
	if data.NumMatches != 2 {
		t.Fatalf("Invoke() numMatches = %d, want 2", data.NumMatches)
	}
	if data.Content != "main.go:1\nnested/helper.go:1" {
		t.Fatalf("Invoke() content = %q", data.Content)
	}
	wantOutput := "main.go:1\nnested/helper.go:1\n\nFound 2 total occurrences across 2 files."
	if result.Output != wantOutput {
		t.Fatalf("Invoke() output = %q, want %q", result.Output, wantOutput)
	}
}

// TestToolInvokeSupportsPathOverride verifies explicit search paths are honored and relativized against the caller cwd.
func TestToolInvokeSupportsPathOverride(t *testing.T) {
	projectDir := t.TempDir()
	searchDir := filepath.Join(projectDir, "src")
	mustMkdirAll(t, searchDir)
	mustWriteFile(t, filepath.Join(searchDir, "main.ts"), "export const target = true\n")
	mustWriteFile(t, filepath.Join(projectDir, "root.ts"), "export const target = true\n")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"pattern": "target",
			"path":    "src",
			"glob":    "*.ts",
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
	mustWriteFile(t, filepath.Join(outsideDir, "secret.txt"), "target text\n")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	_, err = tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"pattern": "target",
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

// TestToolInvokeRejectsInvalidPath verifies handled path validation failures are surfaced in Result.Error.
func TestToolInvokeRejectsInvalidPath(t *testing.T) {
	projectDir := t.TempDir()

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"pattern": "target",
			"path":    "missing",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "Path does not exist: missing" {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}
}

// TestToolInvokeReturnsNoMatches verifies ripgrep exit code 1 is mapped to an empty successful result.
func TestToolInvokeReturnsNoMatches(t *testing.T) {
	projectDir := t.TempDir()
	mustWriteFile(t, filepath.Join(projectDir, "main.go"), "package main\n")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"pattern": "target",
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
	if result.Output != "No files found" {
		t.Fatalf("Invoke() output = %q", result.Output)
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
