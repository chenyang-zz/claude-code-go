package autodream

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// stubRunner is a SubagentRunner that records invocations.
type stubRunner struct {
	invocations int
	messages    [][]message.Message
}

func (r *stubRunner) Run(ctx context.Context, messages []message.Message) error {
	r.invocations++
	r.messages = append(r.messages, messages)
	return nil
}

func TestNewSystem(t *testing.T) {
	sys := NewSystem(nil, "/tmp/test")
	if sys == nil {
		t.Fatal("expected non-nil system")
	}
	if sys.projectRoot != "/tmp/test" {
		t.Errorf("expected projectRoot=/tmp/test, got %s", sys.projectRoot)
	}
}

func TestIsGateOpen_DisabledByDefault(t *testing.T) {
	sys := NewSystem(nil, "/tmp/test")
	if sys.isGateOpen() {
		t.Error("expected gate to be closed when feature flag is not set")
	}
}

func TestIsGateOpen_Enabled(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_AUTO_DREAM", "1")
	defer os.Unsetenv("CLAUDE_FEATURE_AUTO_DREAM")

	sys := NewSystem(nil, "/tmp/test")
	if !sys.isGateOpen() {
		t.Error("expected gate to be open when feature flag is set")
	}
}

func TestRunAutoDream_TimeGateNotPassed(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory-base")
	os.Setenv("CLAUDE_CODE_REMOTE_MEMORY_DIR", memDir)
	os.Setenv("CLAUDE_FEATURE_AUTO_DREAM", "1")
	defer os.Unsetenv("CLAUDE_CODE_REMOTE_MEMORY_DIR")
	defer os.Unsetenv("CLAUDE_FEATURE_AUTO_DREAM")

	// Record a consolidation "just now" so time gate doesn't pass.
	recordConsolidation(dir)

	runner := &stubRunner{}
	sys := NewSystem(runner, dir)
	err := sys.RunAutoDream(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner.invocations > 0 {
		t.Errorf("expected no subagent invocation when time gate not passed, got %d", runner.invocations)
	}
}

func TestRunAutoDream_GateDisabled(t *testing.T) {
	sys := NewSystem(nil, "/tmp/test")
	err := sys.RunAutoDream(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should no-op silently when gate is closed.
}

func TestRunAutoDream_FirstRunWithRunner(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory-base")
	os.Setenv("CLAUDE_CODE_REMOTE_MEMORY_DIR", memDir)
	os.Setenv("CLAUDE_FEATURE_AUTO_DREAM", "1")
	defer os.Unsetenv("CLAUDE_CODE_REMOTE_MEMORY_DIR")
	defer os.Unsetenv("CLAUDE_FEATURE_AUTO_DREAM")

	runner := &stubRunner{}
	sys := NewSystem(runner, dir)
	err := sys.RunAutoDream(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First run: no prior consolidation, time gate should pass.
	// Session gate: no sessions in temp dir, so should NOT fire.
	if runner.invocations > 0 {
		t.Errorf("expected no subagent invocation (no sessions in dir), got %d", runner.invocations)
	}
}

func TestRunAutoDream_LockHeld(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory-base")
	os.Setenv("CLAUDE_CODE_REMOTE_MEMORY_DIR", memDir)
	os.Setenv("CLAUDE_FEATURE_AUTO_DREAM", "1")
	defer os.Unsetenv("CLAUDE_CODE_REMOTE_MEMORY_DIR")
	defer os.Unsetenv("CLAUDE_FEATURE_AUTO_DREAM")

	// Acquire lock first to simulate another process holding it.
	_, err := tryAcquireConsolidationLock(dir)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	runner := &stubRunner{}
	sys := NewSystem(runner, dir)
	err = sys.RunAutoDream(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner.invocations > 0 {
		t.Errorf("expected no subagent invocation when lock is held, got %d", runner.invocations)
	}
}
