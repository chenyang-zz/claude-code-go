package exit

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

func TestExitName(t *testing.T) {
	m := worktreeshared.NewManager()
	tool := NewTool(m)
	if tool.Name() != Name {
		t.Errorf("Name() = %q, want %q", tool.Name(), Name)
	}
}

func TestExitDescription(t *testing.T) {
	m := worktreeshared.NewManager()
	tool := NewTool(m)
	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestExitInputSchema(t *testing.T) {
	m := worktreeshared.NewManager()
	tool := NewTool(m)
	schema := tool.InputSchema()
	if _, ok := schema.Properties["action"]; !ok {
		t.Error("InputSchema should have action property")
	}
	if !schema.Properties["action"].Required {
		t.Error("action should be required")
	}
	if _, ok := schema.Properties["discard_changes"]; !ok {
		t.Error("InputSchema should have discard_changes property")
	}
}

func TestExitIsReadOnly(t *testing.T) {
	m := worktreeshared.NewManager()
	tool := NewTool(m)
	if tool.IsReadOnly() {
		t.Error("ExitWorktreeTool should not be read-only")
	}
}

func TestExitRequiresUserInteraction(t *testing.T) {
	m := worktreeshared.NewManager()
	tool := NewTool(m)
	if !tool.RequiresUserInteraction() {
		t.Error("ExitWorktreeTool should require user interaction")
	}
}

func TestExitInvokeMissingAction(t *testing.T) {
	m := worktreeshared.NewManager()
	tool := NewTool(m)

	call := coretool.Call{
		Input:   map[string]any{},
		Context: coretool.UseContext{WorkingDir: "/tmp"},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error == "" {
		t.Error("expected error for missing action")
	}
}

func TestExitInvokeInvalidAction(t *testing.T) {
	m := worktreeshared.NewManager()
	tool := NewTool(m)

	call := coretool.Call{
		Input: map[string]any{
			"action": "invalid",
		},
		Context: coretool.UseContext{WorkingDir: "/tmp"},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error == "" {
		t.Error("expected error for invalid action")
	}
	if !strings.Contains(result.Error, "keep") && !strings.Contains(result.Error, "remove") {
		t.Errorf("error should mention keep/remove: %q", result.Error)
	}
}

func TestExitInvokeKeep(t *testing.T) {
	m := worktreeshared.NewManager()
	tool := NewTool(m)

	call := coretool.Call{
		Input: map[string]any{
			"action": "keep",
		},
		Context: coretool.UseContext{WorkingDir: "/tmp"},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	output, _ := result.Meta["data"].(Output)
	if output.Action != "keep" {
		t.Errorf("expected action 'keep', got %q", output.Action)
	}
	if !strings.Contains(output.Message, "preserved") {
		t.Errorf("expected 'preserved' in message: %q", output.Message)
	}
}

func TestExitInvokeRemoveWithUncommittedChanges(t *testing.T) {
	dir := setupGitRepo(t)
	m := worktreeshared.NewManager()

	// Create a worktree.
	wt, err := m.CreateWorktree(dir, "test-exit")
	if err != nil {
		t.Fatalf("CreateWorktree error: %v", err)
	}
	defer m.RemoveWorktree(wt.Path, true)

	// Add an uncommitted file to the worktree.
	newFile := filepath.Join(wt.Path, "uncommitted.txt")
	if err := os.WriteFile(newFile, []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write uncommitted file: %v", err)
	}

	tool := NewTool(m)
	call := coretool.Call{
		Input: map[string]any{
			"action": "remove",
		},
		Context: coretool.UseContext{WorkingDir: wt.Path},
	}
	result, _ := tool.Invoke(context.Background(), call)
	if result.Error == "" {
		t.Error("expected error for uncommitted changes without discard_changes")
	}
	if !strings.Contains(result.Error, "uncommitted") {
		t.Errorf("expected uncommitted mention: %q", result.Error)
	}
}

func TestExitInvokeRemoveWithForce(t *testing.T) {
	dir := setupGitRepo(t)
	m := worktreeshared.NewManager()

	wt, err := m.CreateWorktree(dir, "test-force")
	if err != nil {
		t.Fatalf("CreateWorktree error: %v", err)
	}

	tool := NewTool(m)
	call := coretool.Call{
		Input: map[string]any{
			"action":          "remove",
			"discard_changes": true,
		},
		Context: coretool.UseContext{WorkingDir: wt.Path},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	output, _ := result.Meta["data"].(Output)
	if output.Action != "remove" {
		t.Errorf("expected action 'remove', got %q", output.Action)
	}

	// Verify worktree is gone.
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Error("worktree should not exist after force remove")
	}
}

func TestExitInvokeNilReceiver(t *testing.T) {
	var tool *Tool
	call := coretool.Call{Input: map[string]any{}}
	_, err := tool.Invoke(context.Background(), call)
	if err == nil {
		t.Error("expected error for nil receiver")
	}
}

func TestExitInvokeNilManager(t *testing.T) {
	tool := &Tool{manager: nil}
	call := coretool.Call{Input: map[string]any{}}
	_, err := tool.Invoke(context.Background(), call)
	if err == nil {
		t.Error("expected error for nil manager")
	}
}
