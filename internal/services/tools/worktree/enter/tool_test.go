package enter

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	worktreeshared "github.com/sheepzhao/claude-code-go/internal/services/tools/worktree/shared"
)

// mockWorktreeCreateHookDispatcher records hook calls for testing.
type mockWorktreeCreateHookDispatcher struct {
	calledCount int
	lastEvent   hook.HookEvent
	lastInput   hook.WorktreeCreateHookInput
	results     []hook.HookResult
}

func (m *mockWorktreeCreateHookDispatcher) RunHooks(_ context.Context, event hook.HookEvent, input any, cwd string) []hook.HookResult {
	m.calledCount++
	m.lastEvent = event
	if v, ok := input.(hook.WorktreeCreateHookInput); ok {
		m.lastInput = v
	} else if v, ok := input.(*hook.WorktreeCreateHookInput); ok {
		m.lastInput = *v
	}
	return m.results
}

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

// TestWorktreeCreateHookDispatched verifies that the WorktreeCreate hook is
// dispatched after a successful worktree creation with the correct event name
// and input fields.
func TestWorktreeCreateHookDispatched(t *testing.T) {
	dir := setupGitRepo(t)
	m := worktreeshared.NewManager()
	dispatcher := &mockWorktreeCreateHookDispatcher{
		results: []hook.HookResult{{ExitCode: 0}},
	}
	hookCfg := hook.HooksConfig{
		hook.EventWorktreeCreate: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo created"}`)},
		}},
	}
	tool := NewToolWithHooks(m, dispatcher, hookCfg, false)

	call := coretool.Call{
		Input:   map[string]any{"name": "feat-hook"},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	if dispatcher.calledCount != 1 {
		t.Fatalf("calledCount = %d, want 1", dispatcher.calledCount)
	}
	if dispatcher.lastEvent != hook.EventWorktreeCreate {
		t.Fatalf("event = %q, want %q", dispatcher.lastEvent, hook.EventWorktreeCreate)
	}
	if dispatcher.lastInput.Name != "feat-hook" {
		t.Fatalf("name = %q, want 'feat-hook'", dispatcher.lastInput.Name)
	}
	if dispatcher.lastInput.HookEventName != string(hook.EventWorktreeCreate) {
		t.Fatalf("hook_event_name = %q, want %q", dispatcher.lastInput.HookEventName, hook.EventWorktreeCreate)
	}
	if dispatcher.lastInput.CWD != dir {
		t.Fatalf("cwd = %q, want %q", dispatcher.lastInput.CWD, dir)
	}

	// Clean up.
	output, _ := result.Meta["data"].(Output)
	m.RemoveWorktree(output.WorktreePath, true)
}

// TestWorktreeCreateHookBlocking verifies that a blocking hook (exit code 2)
// returns an error result without panicking, and that the worktree creation
// result is still present (worktree was already created via git).
func TestWorktreeCreateHookBlocking(t *testing.T) {
	dir := setupGitRepo(t)
	m := worktreeshared.NewManager()
	dispatcher := &mockWorktreeCreateHookDispatcher{
		results: []hook.HookResult{{ExitCode: 2, Stderr: "worktree creation blocked by policy"}},
	}
	hookCfg := hook.HooksConfig{
		hook.EventWorktreeCreate: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo block"}`)},
		}},
	}
	tool := NewToolWithHooks(m, dispatcher, hookCfg, false)

	call := coretool.Call{
		Input:   map[string]any{"name": "feat-blocked"},
		Context: coretool.UseContext{WorkingDir: dir},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected blocking error in result")
	}
	if !strings.Contains(result.Error, "worktree creation blocked by policy") {
		t.Fatalf("result.Error = %q, want to contain blocking message", result.Error)
	}
	if dispatcher.calledCount != 1 {
		t.Fatalf("calledCount = %d, want 1", dispatcher.calledCount)
	}

	// Clean up the worktree that was created before the hook blocked.
	// Use a known path pattern: .claude/worktrees/<slug>
	worktreePath := filepath.Join(dir, ".claude", "worktrees", "feat-blocked")
	if _, statErr := os.Stat(worktreePath); statErr == nil {
		m.RemoveWorktree(worktreePath, true)
	}
}

// TestWorktreeCreateHookSkippedWhenNoConfig verifies that the hook is not
// dispatched when no WorktreeCreate hooks are configured.
func TestWorktreeCreateHookSkippedWhenNoConfig(t *testing.T) {
	dir := setupGitRepo(t)
	m := worktreeshared.NewManager()
	dispatcher := &mockWorktreeCreateHookDispatcher{}
	tool := NewToolWithHooks(m, dispatcher, nil, false)

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
	if dispatcher.calledCount != 0 {
		t.Fatalf("calledCount = %d, want 0 (no hooks configured)", dispatcher.calledCount)
	}

	// Clean up.
	output, _ := result.Meta["data"].(Output)
	m.RemoveWorktree(output.WorktreePath, true)
}

// TestWorktreeCreateHookSkippedWhenDisabled verifies that the global
// disableAllHooks flag prevents hook dispatch even when matchers exist.
func TestWorktreeCreateHookSkippedWhenDisabled(t *testing.T) {
	dir := setupGitRepo(t)
	m := worktreeshared.NewManager()
	dispatcher := &mockWorktreeCreateHookDispatcher{
		results: []hook.HookResult{{ExitCode: 0}},
	}
	hookCfg := hook.HooksConfig{
		hook.EventWorktreeCreate: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo hi"}`)},
		}},
	}
	tool := NewToolWithHooks(m, dispatcher, hookCfg, true)

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
	if dispatcher.calledCount != 0 {
		t.Fatalf("calledCount = %d, want 0 (hooks globally disabled)", dispatcher.calledCount)
	}

	// Clean up.
	output, _ := result.Meta["data"].(Output)
	m.RemoveWorktree(output.WorktreePath, true)
}
