package bash

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformshell "github.com/sheepzhao/claude-code-go/internal/platform/shell"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registry identifier used by the migrated Bash tool.
	Name = "Bash"
	// defaultTimeoutMilliseconds mirrors the source Bash default timeout when no environment override is configured.
	defaultTimeoutMilliseconds = 120000
	// maxTimeoutMilliseconds mirrors the source Bash timeout cap when no environment override is configured.
	maxTimeoutMilliseconds = 600000
)

// ShellExecutor describes the foreground shell execution dependency used by the Bash tool.
type ShellExecutor interface {
	// Execute runs one normalized foreground shell request and returns the normalized result.
	Execute(ctx context.Context, req platformshell.Request) (platformshell.Result, error)
}

// CommandPermissionChecker describes the minimal Bash permission dependency used by the Bash tool.
type CommandPermissionChecker interface {
	// Check evaluates whether the provided Bash command is currently allowed to run.
	Check(command string) platformshell.PermissionEvaluation
}

// Tool implements the minimum migrated foreground Bash tool path.
type Tool struct {
	// executor runs the normalized shell request in the host environment.
	executor ShellExecutor
	// permissions checks Bash(...) allow/deny/ask rules before execution starts.
	permissions CommandPermissionChecker
}

// Input stores the typed request payload accepted by the migrated Bash tool.
type Input struct {
	// Command stores the shell command string that should execute in the current working directory.
	Command string `json:"command"`
	// Timeout stores the optional timeout override in milliseconds.
	Timeout int `json:"timeout,omitempty"`
	// Description stores the optional model-facing command summary accepted for source compatibility.
	Description string `json:"description,omitempty"`
	// RunInBackground preserves the source field while returning a stable not-yet-migrated error branch.
	RunInBackground bool `json:"run_in_background,omitempty"`
	// DangerouslyDisableSandbox preserves the source field while returning a stable not-yet-migrated error branch.
	DangerouslyDisableSandbox bool `json:"dangerouslyDisableSandbox,omitempty"`
}

// Output stores the structured result metadata returned by the migrated Bash tool.
type Output struct {
	// Command echoes the executed command string.
	Command string `json:"command"`
	// Stdout stores captured standard output.
	Stdout string `json:"stdout"`
	// Stderr stores captured standard error.
	Stderr string `json:"stderr"`
	// ExitCode stores the process exit code or the synthetic timeout code.
	ExitCode int `json:"exitCode"`
	// TimedOut reports whether the command exceeded its timeout.
	TimedOut bool `json:"timedOut"`
}

// NewTool constructs a Bash tool with explicit host shell and permission dependencies.
func NewTool(executor ShellExecutor, permissions CommandPermissionChecker) *Tool {
	return &Tool{
		executor:    executor,
		permissions: permissions,
	}
}

// Name returns the stable registration name for the migrated Bash tool.
func (t *Tool) Name() string {
	return Name
}

// Description returns the summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return "Execute one foreground shell command in the current working directory with an optional timeout."
}

// InputSchema returns the minimum source-aligned Bash tool input contract used by the Go host.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
}

// IsReadOnly reports that Bash commands may mutate external state.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that arbitrary shell commands should not be assumed safe to execute in parallel.
func (t *Tool) IsConcurrencySafe() bool {
	return false
}

// Invoke validates input, enforces the minimal Bash permission rules, runs the foreground command, and normalizes the result.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("bash tool: nil receiver")
	}
	if t.executor == nil {
		return coretool.Result{}, fmt.Errorf("bash tool: executor is not configured")
	}
	if t.permissions == nil {
		return coretool.Result{}, fmt.Errorf("bash tool: permission checker is not configured")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	command := strings.TrimSpace(input.Command)
	if command == "" {
		return coretool.Result{Error: "command is required"}, nil
	}
	if input.RunInBackground {
		return coretool.Result{Error: "Background Bash tasks are not available in Claude Code Go yet."}, nil
	}
	if input.DangerouslyDisableSandbox {
		return coretool.Result{Error: "Sandbox override is not available in Claude Code Go yet."}, nil
	}

	timeoutMilliseconds, err := resolveTimeoutMilliseconds(input.Timeout)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	evaluation := t.permissions.Check(command)
	switch evaluation.Decision {
	case corepermission.DecisionDeny, corepermission.DecisionAsk:
		return coretool.Result{
			Error: evaluation.Message,
			Meta: map[string]any{
				"permission_decision": string(evaluation.Decision),
				"permission_rule":     evaluation.Rule,
			},
		}, nil
	}

	logger.DebugCF("bash_tool", "starting foreground bash tool invocation", map[string]any{
		"working_dir": call.Context.WorkingDir,
		"timeout_ms":  timeoutMilliseconds,
		"command_len": len(command),
	})

	execResult, err := t.executor.Execute(ctx, platformshell.Request{
		Command:    command,
		WorkingDir: call.Context.WorkingDir,
		Timeout:    time.Duration(timeoutMilliseconds) * time.Millisecond,
	})
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("bash tool: %v", err)}, nil
	}

	output := Output{
		Command:  command,
		Stdout:   execResult.Stdout,
		Stderr:   execResult.Stderr,
		ExitCode: execResult.ExitCode,
		TimedOut: execResult.TimedOut,
	}

	if execResult.TimedOut {
		return coretool.Result{
			Error: renderTimeout(output, timeoutMilliseconds),
			Meta: map[string]any{
				"data": output,
			},
		}, nil
	}
	if execResult.ExitCode != 0 {
		return coretool.Result{
			Error: renderFailure(output),
			Meta: map[string]any{
				"data": output,
			},
		}, nil
	}

	logger.DebugCF("bash_tool", "foreground bash tool invocation finished", map[string]any{
		"exit_code":  output.ExitCode,
		"timed_out":  output.TimedOut,
		"stdout_len": len(output.Stdout),
		"stderr_len": len(output.Stderr),
	})

	return coretool.Result{
		Output: renderSuccess(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// inputSchema declares the minimum Bash tool input fields carried forward in the current batch.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"command": {
				Type:        coretool.ValueKindString,
				Description: "The foreground shell command to execute.",
				Required:    true,
			},
			"timeout": {
				Type:        coretool.ValueKindInteger,
				Description: fmt.Sprintf("Optional timeout in milliseconds (max %d).", effectiveMaxTimeoutMilliseconds()),
			},
			"description": {
				Type:        coretool.ValueKindString,
				Description: "Optional short description of what the command does.",
			},
			"run_in_background": {
				Type:        coretool.ValueKindBoolean,
				Description: "Set to true to run the command in the background. Not yet supported in Claude Code Go.",
			},
			"dangerouslyDisableSandbox": {
				Type:        coretool.ValueKindBoolean,
				Description: "Set to true to disable the sandbox. Not yet supported in Claude Code Go.",
			},
		},
	}
}

// resolveTimeoutMilliseconds normalizes the optional caller-supplied timeout against the current defaults and cap.
func resolveTimeoutMilliseconds(value int) (int, error) {
	timeoutMilliseconds := value
	if timeoutMilliseconds == 0 {
		timeoutMilliseconds = effectiveDefaultTimeoutMilliseconds()
	}
	if timeoutMilliseconds < 0 {
		return 0, fmt.Errorf("timeout must be greater than or equal to 0")
	}
	if timeoutMilliseconds > effectiveMaxTimeoutMilliseconds() {
		return 0, fmt.Errorf("timeout must be less than or equal to %d", effectiveMaxTimeoutMilliseconds())
	}
	return timeoutMilliseconds, nil
}

// effectiveDefaultTimeoutMilliseconds mirrors the source env override behavior for the default timeout.
func effectiveDefaultTimeoutMilliseconds() int {
	return parsePositiveIntegerEnv("BASH_DEFAULT_TIMEOUT_MS", defaultTimeoutMilliseconds)
}

// effectiveMaxTimeoutMilliseconds mirrors the source env override behavior for the maximum timeout.
func effectiveMaxTimeoutMilliseconds() int {
	maximum := parsePositiveIntegerEnv("BASH_MAX_TIMEOUT_MS", maxTimeoutMilliseconds)
	defaultTimeout := effectiveDefaultTimeoutMilliseconds()
	if maximum < defaultTimeout {
		return defaultTimeout
	}
	return maximum
}

// parsePositiveIntegerEnv reads one optional positive integer environment value and falls back when absent or invalid.
func parsePositiveIntegerEnv(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// renderSuccess converts a successful foreground Bash execution into one stable caller-facing text payload.
func renderSuccess(output Output) string {
	stdout := strings.TrimRight(output.Stdout, "\n")
	stderr := strings.TrimRight(output.Stderr, "\n")

	switch {
	case stdout == "" && stderr == "":
		return "Done"
	case stdout != "" && stderr == "":
		return stdout
	case stdout == "" && stderr != "":
		return fmt.Sprintf("stderr:\n%s", stderr)
	default:
		return fmt.Sprintf("stdout:\n%s\n\nstderr:\n%s", stdout, stderr)
	}
}

// renderFailure converts one non-zero exit result into a stable caller-facing error payload.
func renderFailure(output Output) string {
	details := renderStreamDetails(output.Stdout, output.Stderr)
	if details == "" {
		return fmt.Sprintf("Command failed with exit code %d.", output.ExitCode)
	}
	return fmt.Sprintf("Command failed with exit code %d.\n%s", output.ExitCode, details)
}

// renderTimeout converts one timed-out foreground result into a stable caller-facing error payload.
func renderTimeout(output Output, timeoutMilliseconds int) string {
	details := renderStreamDetails(output.Stdout, output.Stderr)
	if details == "" {
		return fmt.Sprintf("Command timed out after %dms.", timeoutMilliseconds)
	}
	return fmt.Sprintf("Command timed out after %dms.\n%s", timeoutMilliseconds, details)
}

// renderStreamDetails normalizes stdout and stderr into one shared multi-section payload.
func renderStreamDetails(stdout string, stderr string) string {
	trimmedStdout := strings.TrimRight(stdout, "\n")
	trimmedStderr := strings.TrimRight(stderr, "\n")

	switch {
	case trimmedStdout == "" && trimmedStderr == "":
		return ""
	case trimmedStdout != "" && trimmedStderr == "":
		return fmt.Sprintf("stdout:\n%s", trimmedStdout)
	case trimmedStdout == "" && trimmedStderr != "":
		return fmt.Sprintf("stderr:\n%s", trimmedStderr)
	default:
		return fmt.Sprintf("stdout:\n%s\n\nstderr:\n%s", trimmedStdout, trimmedStderr)
	}
}
