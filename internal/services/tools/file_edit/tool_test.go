package file_edit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

// TestToolInvokeReplacesSingleOccurrence verifies the first-batch FileEditTool replaces one unique match in place.
func TestToolInvokeReplacesSingleOccurrence(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "main.go")
	mustWriteFile(t, filePath, "package main\n\nfunc main() {}\n")

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "main.go",
			"old_string": "main()",
			"new_string": "run()",
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
	if result.Output != "Updated file: main.go (1 replacement)" {
		t.Fatalf("Invoke() output = %q", result.Output)
	}

	updatedContent, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(updatedContent) != "package main\n\nfunc run() {}\n" {
		t.Fatalf("updated content = %q", string(updatedContent))
	}

	data := result.Meta["data"].(Output)
	if data.Replacements != 1 || data.Content != string(updatedContent) {
		t.Fatalf("Invoke() metadata = %#v", data)
	}
}

// TestToolInvokeRequiresReplaceAllForAmbiguousMatch verifies repeated matches are rejected unless replace_all is enabled.
func TestToolInvokeRequiresReplaceAllForAmbiguousMatch(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "foo\nfoo\n")

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "notes.txt",
			"old_string": "foo",
			"new_string": "bar",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("Invoke() result.Error = empty, want ambiguity error")
	}
}

// TestToolInvokeSupportsReplaceAll verifies callers can replace all exact matches when requested.
func TestToolInvokeSupportsReplaceAll(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "foo\nfoo\n")

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":   "notes.txt",
			"old_string":  "foo",
			"new_string":  "bar",
			"replace_all": true,
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

	data := result.Meta["data"].(Output)
	if data.Replacements != 2 {
		t.Fatalf("Invoke() replacements = %d, want 2", data.Replacements)
	}
}

// TestToolInvokeRejectsWriteWithoutPermission verifies the migrated tool reuses the minimal write-permission gate.
func TestToolInvokeRejectsWriteWithoutPermission(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "before\n")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	_, err = tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "notes.txt",
			"old_string": "before",
			"new_string": "after",
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

// TestToolInvokeRejectsMissingString verifies file drift or mismatched expectations are rejected without writing.
func TestToolInvokeRejectsMissingString(t *testing.T) {
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
			"file_path":  "notes.txt",
			"old_string": "missing",
			"new_string": "after",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("Invoke() result.Error = empty, want missing string error")
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
