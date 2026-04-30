package enter

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	worktreeshared "github.com/sheepzhao/claude-code-go/internal/services/tools/worktree/shared"
)

func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	dummy := filepath.Join(dir, "dummy.txt")
	os.WriteFile(dummy, []byte("dummy"), 0o644)
	runGit(t, dir, "add", "dummy.txt")
	runGit(t, dir, "commit", "-m", "initial")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %s: %v", args, string(out), err)
	}
}

func TestEnterName(t *testing.T) {
	m := worktreeshared.NewManager()
	tool := NewTool(m)
	if tool.Name() != Name {
		t.Errorf("Name() = %q, want %q", tool.Name(), Name)
	}
}

func TestEnterDescription(t *testing.T) {
	m := worktreeshared.NewManager()
	tool := NewTool(m)
	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestEnterInputSchema(t *testing.T) {
	m := worktreeshared.NewManager()
	tool := NewTool(m)
	schema := tool.InputSchema()
	if _, ok := schema.Properties["name"]; !ok {
		t.Error("InputSchema should have name property")
	}
}

func TestEnterIsReadOnly(t *testing.T) {
	m := worktreeshared.NewManager()
	tool := NewTool(m)
	if tool.IsReadOnly() {
		t.Error("EnterWorktreeTool should not be read-only")
	}
}

func TestEnterRequiresUserInteraction(t *testing.T) {
	m := worktreeshared.NewManager()
	tool := NewTool(m)
	if !tool.RequiresUserInteraction() {
		t.Error("EnterWorktreeTool should require user interaction")
	}
}

func TestEnterInvokeNilReceiver(t *testing.T) {
	var tool *Tool
	call := coretool.Call{Input: map[string]any{}}
	_, err := tool.Invoke(context.Background(), call)
	if err == nil {
		t.Error("expected error for nil receiver")
	}
}

func TestEnterInvokeNilManager(t *testing.T) {
	tool := &Tool{manager: nil}
	call := coretool.Call{Input: map[string]any{}}
	_, err := tool.Invoke(context.Background(), call)
	if err == nil {
		t.Error("expected error for nil manager")
	}
}

func TestEnterInvokeCreatesWorktree(t *testing.T) {
	dir := setupGitRepo(t)
	m := worktreeshared.NewManager()
	tool := NewTool(m)

	call := coretool.Call{
		Input: map[string]any{
			"name": "my-feature",
		},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	output, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatal("expected Output in metadata")
	}
	if output.WorktreePath == "" {
		t.Error("expected non-empty worktree path")
	}
	if output.WorktreeBranch == "" {
		t.Error("expected non-empty branch name")
	}
	if !strings.Contains(output.WorktreePath, "my-feature") {
		t.Errorf("expected worktree path to contain slug, got %q", output.WorktreePath)
	}

	// Clean up.
	m.RemoveWorktree(output.WorktreePath, true)
}

func TestEnterInvokeWithoutName(t *testing.T) {
	dir := setupGitRepo(t)
	m := worktreeshared.NewManager()
	tool := NewTool(m)

	call := coretool.Call{
		Input:   map[string]any{},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	output, _ := result.Meta["data"].(Output)
	if output.WorktreePath == "" {
		t.Error("expected worktree path even without name")
	}

	m.RemoveWorktree(output.WorktreePath, true)
}

func TestEnterInvokeInvalidSlug(t *testing.T) {
	dir := setupGitRepo(t)
	m := worktreeshared.NewManager()
	tool := NewTool(m)

	call := coretool.Call{
		Input: map[string]any{
			"name": "..",
		},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error == "" {
		t.Error("expected error for invalid slug")
	}
}

func TestEnterInvokeNotGitRepo(t *testing.T) {
	dir := t.TempDir()
	m := worktreeshared.NewManager()
	tool := NewTool(m)

	call := coretool.Call{
		Input:   map[string]any{},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error == "" {
		t.Error("expected error for non-git directory")
	}
}
