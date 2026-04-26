package bash

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformshell "github.com/sheepzhao/claude-code-go/internal/platform/shell"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
)

// TestToolDescription verifies the BashTool description contains the key guidance migrated from the TypeScript prompt.
func TestToolDescription(t *testing.T) {
	tool := NewTool(platformshell.NewExecutor(), platformshell.NewPermissionChecker(coreconfig.PermissionConfig{}))
	desc := tool.Description()
	if desc == "" {
		t.Fatal("Description() is empty")
	}
	mustContain := []string{
		"bash command",
		"timeout",
		"run_in_background",
		"git commands",
		"sleep",
	}
	for _, substr := range mustContain {
		if !strings.Contains(desc, substr) {
			t.Errorf("Description() missing %q", substr)
		}
	}
}

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

// TestToolInvokeStartsBackgroundExecution verifies run_in_background creates one live background task in the shared runtime store.
func TestToolInvokeStartsBackgroundExecution(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewToolWithRuntime(platformshell.NewExecutor(), platformshell.NewPermissionChecker(coreconfig.PermissionConfig{
		Allow: []string{"Bash(*)"},
	}), "default", store)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: "Bash",
		Input: map[string]any{
			"command":           timeoutCommandForToolTest(),
			"run_in_background": true,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error output = %q, want empty", result.Error)
	}
	if !strings.Contains(result.Output, "Started background task: task_") {
		t.Fatalf("Invoke() output = %q, want background task start message", result.Output)
	}

	tasks := store.List()
	if len(tasks) != 1 {
		t.Fatalf("List() len = %d, want 1", len(tasks))
	}
	if tasks[0].Type != "bash" {
		t.Fatalf("List()[0].Type = %q, want bash", tasks[0].Type)
	}
	if !tasks[0].ControlsAvailable {
		t.Fatal("List()[0].ControlsAvailable = false, want true")
	}
	if _, err := store.Stop(tasks[0].ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
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

// TestToolInvokeRequestsApproval verifies ask-style Bash permission outcomes return a retryable Bash permission error.
func TestToolInvokeRequestsApproval(t *testing.T) {
	tool := NewTool(platformshell.NewExecutor(), platformshell.NewPermissionChecker(coreconfig.PermissionConfig{}))

	_, err := tool.Invoke(context.Background(), coretool.Call{
		Name: "Bash",
		Input: map[string]any{
			"command": "pwd",
		},
	})
	if err == nil {
		t.Fatal("Invoke() error = nil, want Bash permission error")
	}

	var permissionErr *corepermission.BashPermissionError
	if !errors.As(err, &permissionErr) {
		t.Fatalf("Invoke() error = %T, want *BashPermissionError", err)
	}
	if permissionErr.Decision != corepermission.DecisionAsk {
		t.Fatalf("Invoke() decision = %q, want %q", permissionErr.Decision, corepermission.DecisionAsk)
	}
	if permissionErr.Command != "pwd" {
		t.Fatalf("Invoke() command = %q, want pwd", permissionErr.Command)
	}
}

// TestToolInvokeAcceptEditsAutoAllowsFilesystemCommands verifies acceptEdits auto-allows the migrated filesystem-style Bash subset.
func TestToolInvokeAcceptEditsAutoAllowsFilesystemCommands(t *testing.T) {
	projectDir := t.TempDir()
	tool := NewToolWithMode(platformshell.NewExecutor(), platformshell.NewPermissionChecker(coreconfig.PermissionConfig{}), "acceptEdits")

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: "Bash",
		Input: map[string]any{
			"command": "mkdir accept-edits-dir",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v, want nil", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() result.Error = %q, want empty", result.Error)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "accept-edits-dir")); err != nil {
		t.Fatalf("Stat() error = %v, want created directory", err)
	}
}

// TestToolInvokeAcceptEditsStillAsksForNonFilesystemCommands verifies acceptEdits does not auto-approve arbitrary Bash commands.
func TestToolInvokeAcceptEditsStillAsksForNonFilesystemCommands(t *testing.T) {
	tool := NewToolWithMode(platformshell.NewExecutor(), platformshell.NewPermissionChecker(coreconfig.PermissionConfig{}), "acceptEdits")

	_, err := tool.Invoke(context.Background(), coretool.Call{
		Name: "Bash",
		Input: map[string]any{
			"command": successCommandForToolTest(),
		},
	})
	if err == nil {
		t.Fatal("Invoke() error = nil, want Bash permission error")
	}

	var permissionErr *corepermission.BashPermissionError
	if !errors.As(err, &permissionErr) {
		t.Fatalf("Invoke() error = %T, want *BashPermissionError", err)
	}
}

// TestToolInvokeDontAskConvertsAskIntoStableDenial verifies dontAsk short-circuits ask-style Bash requests without entering approval flow.
func TestToolInvokeDontAskConvertsAskIntoStableDenial(t *testing.T) {
	tool := NewToolWithMode(platformshell.NewExecutor(), platformshell.NewPermissionChecker(coreconfig.PermissionConfig{}), "dontAsk")

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: "Bash",
		Input: map[string]any{
			"command": "pwd",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v, want nil", err)
	}
	if result.Error != `Permission to execute "pwd" was not granted.` {
		t.Fatalf("Invoke() result.Error = %q, want stable dontAsk denial", result.Error)
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
