package glob

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

// TestToolDescription verifies the GlobTool description contains the key guidance migrated from the TypeScript prompt.
func TestToolDescription(t *testing.T) {
	tool := NewTool(platformfs.NewLocalFS(), nil)
	desc := tool.Description()
	if desc == "" {
		t.Fatal("Description() is empty")
	}
	mustContain := []string{
		"glob patterns",
		"**/*.js",
		"modification time",
		"Agent tool",
	}
	for _, substr := range mustContain {
		if !strings.Contains(desc, substr) {
			t.Errorf("Description() missing %q", substr)
		}
	}
}

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
	want := "Directory does not exist: missing. Note: your current working directory is " + projectDir + "."
	if result.Error != want {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}
}

// TestToolInvokeSuggestsPathUnderWorkingDir verifies missing sibling paths receive the source-aligned cwd suggestion.
func TestToolInvokeSuggestsPathUnderWorkingDir(t *testing.T) {
	parentDir := t.TempDir()
	projectDir := filepath.Join(parentDir, "repo")
	mustMkdirAll(t, projectDir)
	mustMkdirAll(t, filepath.Join(projectDir, "docs"))
	missingPath := filepath.Join(parentDir, "docs")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"pattern": "*.md",
			"path":    missingPath,
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}

	want := "Directory does not exist: " + missingPath + ". Note: your current working directory is " + projectDir + ". Did you mean " + filepath.Join(projectDir, "docs") + "?"
	if result.Error != want {
		t.Fatalf("Invoke() result.Error = %q, want %q", result.Error, want)
	}
}

// TestToolInvokeSkipsDirectoryPrecheckForUNCPath verifies UNC-style paths do not trigger the extra validation stat call.
func TestToolInvokeSkipsDirectoryPrecheckForUNCPath(t *testing.T) {
	workingDir := t.TempDir()

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	trackingFS := &trackingFileSystem{}
	tool := NewTool(trackingFS, policy)

	_, err = tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"pattern": "*.txt",
			"path":    "//server/share",
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
	if trackingFS.statCalls != 0 {
		t.Fatalf("Stat() calls = %d, want 0", trackingFS.statCalls)
	}
}

// TestToolInvokeAllowsOutsideWorkingDirWhenGlobRuleMatches verifies glob-specific wildcard rules can pre-authorize one query.
func TestToolInvokeAllowsOutsideWorkingDirWhenGlobRuleMatches(t *testing.T) {
	workingDir := t.TempDir()
	searchDir := t.TempDir()
	mustWriteFile(t, filepath.Join(searchDir, "guide.md"), "# guide\n")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{
		Read: []corepermission.Rule{
			{
				Source:   corepermission.RuleSourceSession,
				Decision: corepermission.DecisionAllow,
				BaseDir:  searchDir,
				Pattern:  "**/*.md",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"pattern": "**/*.md",
			"path":    searchDir,
		},
		Context: coretool.UseContext{
			WorkingDir: workingDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}

	data := result.Meta["data"].(Output)
	if len(data.Filenames) != 1 || data.Filenames[0] != filepath.Join(searchDir, "guide.md") {
		t.Fatalf("Invoke() filenames = %#v", data.Filenames)
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

// trackingFileSystem records Stat invocations while satisfying the FileSystem contract for focused validation tests.
type trackingFileSystem struct {
	statCalls int
}

// Stat records the validation precheck call and returns a missing-path error if reached unexpectedly.
func (f *trackingFileSystem) Stat(path string) (os.FileInfo, error) {
	f.statCalls++
	return nil, os.ErrNotExist
}

// Lstat panics because the UNC-precheck test should not reach it.
func (f *trackingFileSystem) Lstat(path string) (os.FileInfo, error) {
	panic("unexpected Lstat call")
}

// ReadDir panics because the UNC-precheck test should fail at permissions first.
func (f *trackingFileSystem) ReadDir(path string) ([]os.DirEntry, error) {
	panic("unexpected ReadDir call")
}

// ReadFile panics because the UNC-precheck test does not read files.
func (f *trackingFileSystem) ReadFile(path string) ([]byte, error) {
	panic("unexpected ReadFile call")
}

// OpenRead panics because the UNC-precheck test does not open files.
func (f *trackingFileSystem) OpenRead(path string) (io.ReadCloser, error) {
	panic("unexpected OpenRead call")
}

// WriteFile panics because GlobTool is read-only.
func (f *trackingFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	panic("unexpected WriteFile call")
}

// MkdirAll panics because GlobTool never creates directories.
func (f *trackingFileSystem) MkdirAll(path string, perm os.FileMode) error {
	panic("unexpected MkdirAll call")
}

// Rename panics because GlobTool never renames files.
func (f *trackingFileSystem) Rename(oldPath, newPath string) error {
	panic("unexpected Rename call")
}

// Remove panics because GlobTool never removes files.
func (f *trackingFileSystem) Remove(path string) error {
	panic("unexpected Remove call")
}

// RemoveAll panics because GlobTool never removes trees.
func (f *trackingFileSystem) RemoveAll(path string) error {
	panic("unexpected RemoveAll call")
}

// Readlink panics because the UNC-precheck test does not resolve symlink entries.
func (f *trackingFileSystem) Readlink(path string) (string, error) {
	panic("unexpected Readlink call")
}

// EvalSymlinks panics because the UNC-precheck test does not reach path suggestions.
func (f *trackingFileSystem) EvalSymlinks(path string) (string, error) {
	panic("unexpected EvalSymlinks call")
}
