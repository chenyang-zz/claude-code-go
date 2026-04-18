package bash

import (
	"context"
	"runtime"
	"strings"
	"testing"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformshell "github.com/sheepzhao/claude-code-go/internal/platform/shell"
)

// TestToolInvokeSuccess verifies the Bash tool executes one allowed foreground command and returns stable text output.
func TestToolInvokeSuccess(t *testing.T) {
	tool := NewTool(platformshell.NewExecutor(), platformshell.NewPermissionChecker(coreconfig.PermissionConfig{
		Allow: []string{"Bash(*)"},
	}))

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"command": successCommandForToolTest(),
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error output = %q, want empty", result.Error)
	}
	if got := strings.TrimSpace(result.Output); got != "hello-from-bash-tool" {
		t.Fatalf("Invoke() output = %q, want hello-from-bash-tool", got)
	}
}

// TestToolInvokeRejectsBackgroundExecution verifies the current batch keeps background Bash out of scope with a stable message.
func TestToolInvokeRejectsBackgroundExecution(t *testing.T) {
	tool := NewTool(platformshell.NewExecutor(), platformshell.NewPermissionChecker(coreconfig.PermissionConfig{
		Allow: []string{"Bash(*)"},
	}))

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"command":           successCommandForToolTest(),
			"run_in_background": true,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "Background Bash tasks are not available in Claude Code Go yet." {
		t.Fatalf("Invoke() error = %q, want stable background boundary", result.Error)
	}
}

// TestToolInvokeRejectsPermissionDenied verifies deny rules short-circuit execution with a stable permission message.
func TestToolInvokeRejectsPermissionDenied(t *testing.T) {
	tool := NewTool(platformshell.NewExecutor(), platformshell.NewPermissionChecker(coreconfig.PermissionConfig{
		Deny: []string{"Bash(*)"},
	}))

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"command": "pwd",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, `Permission to execute "pwd" has been denied.`) {
		t.Fatalf("Invoke() error = %q, want deny message", result.Error)
	}
}

// TestToolInvokeTimeout verifies timed-out commands return a stable timeout error payload.
func TestToolInvokeTimeout(t *testing.T) {
	tool := NewTool(platformshell.NewExecutor(), platformshell.NewPermissionChecker(coreconfig.PermissionConfig{
		Allow: []string{"Bash(*)"},
	}))

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"command": timeoutCommandForToolTest(),
			"timeout": 100,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "Command timed out after 100ms.") {
		t.Fatalf("Invoke() error = %q, want timeout message", result.Error)
	}
}

// TestToolInvokeFailure verifies non-zero shell exits are surfaced through the normalized error branch.
func TestToolInvokeFailure(t *testing.T) {
	tool := NewTool(platformshell.NewExecutor(), platformshell.NewPermissionChecker(coreconfig.PermissionConfig{
		Allow: []string{"Bash(*)"},
	}))

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"command": failureCommandForToolTest(),
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "Command failed with exit code 7.") {
		t.Fatalf("Invoke() error = %q, want exit-code message", result.Error)
	}
}

// successCommandForToolTest returns a cross-platform shell command that prints one deterministic line.
func successCommandForToolTest() string {
	if runtime.GOOS == "windows" {
		return "Write-Output 'hello-from-bash-tool'"
	}
	return "printf 'hello-from-bash-tool\\n'"
}

// timeoutCommandForToolTest returns a cross-platform shell command that exceeds the short unit-test timeout.
func timeoutCommandForToolTest() string {
	if runtime.GOOS == "windows" {
		return "Start-Sleep -Seconds 2"
	}
	return "sleep 2"
}

// failureCommandForToolTest returns a cross-platform shell command that exits with a deterministic non-zero status.
func failureCommandForToolTest() string {
	if runtime.GOOS == "windows" {
		return "exit 7"
	}
	return "exit 7"
}
