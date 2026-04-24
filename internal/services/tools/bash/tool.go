package bash

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
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

// BackgroundShellExecutor describes the background shell execution dependency used by the Bash tool.
type BackgroundShellExecutor interface {
	// Start launches one normalized background shell request and returns a stoppable process handle.
	Start(req platformshell.Request) (platformshell.BackgroundProcess, error)
}

// CommandPermissionChecker describes the minimal Bash permission dependency used by the Bash tool.
type CommandPermissionChecker interface {
	// Check evaluates whether the provided Bash command is currently allowed to run.
	Check(command string) platformshell.PermissionEvaluation
}

// BackgroundTaskStore describes the shared lifecycle store used by background Bash tasks and task-control tools.
type BackgroundTaskStore interface {
	// Register inserts one new live background task snapshot into the shared store.
	Register(task coresession.BackgroundTaskSnapshot, stopper interface{ Stop() error })
	// Update replaces the stored snapshot for one existing task.
	Update(task coresession.BackgroundTaskSnapshot) bool
	// Remove deletes one task from the shared task list.
	Remove(id string)
}

// Tool implements the minimum migrated foreground Bash tool path.
type Tool struct {
	// executor runs the normalized shell request in the host environment.
	executor ShellExecutor
	// backgroundExecutor launches background shell commands when run_in_background is requested.
	backgroundExecutor BackgroundShellExecutor
	// permissions checks Bash(...) allow/deny/ask rules before execution starts.
	permissions CommandPermissionChecker
	// approvalMode stores the current default approval mode used to interpret ask-style Bash outcomes.
	approvalMode string
	// taskStore exposes the shared runtime task lifecycle store used by background Bash tasks.
	taskStore BackgroundTaskStore
	// notificationEmitter sends background-task completion notifications into the host event stream.
	notificationEmitter NotificationEmitter
	// securityScanner performs pre-execution security checks on allowed commands.
	securityScanner SecurityScanner
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
	// OutputFilePath stores the path to a captured output file when the command used stdout redirection.
	OutputFilePath string `json:"outputFilePath,omitempty"`
	// OutputFileSize stores the size in bytes of the captured output file.
	OutputFileSize int64 `json:"outputFileSize,omitempty"`
	// ElapsedSeconds stores the wall-clock seconds the command took to execute.
	ElapsedSeconds float64 `json:"elapsedSeconds,omitempty"`
}

// BackgroundOutput stores the structured result metadata returned when one Bash command starts in the background.
type BackgroundOutput struct {
	// TaskID stores the stable runtime task identifier created for the background command.
	TaskID string `json:"taskId"`
	// Command echoes the started command string.
	Command string `json:"command"`
	// Summary stores the minimum user-visible label registered with the shared task store.
	Summary string `json:"summary"`
}

// NewTool constructs a Bash tool with explicit host shell and permission dependencies.
func NewTool(executor ShellExecutor, permissions CommandPermissionChecker) *Tool {
	return NewToolWithRuntime(executor, permissions, "default", nil)
}

// NewToolWithMode constructs a Bash tool with explicit host shell, permission, and approval-mode dependencies.
func NewToolWithMode(executor ShellExecutor, permissions CommandPermissionChecker, approvalMode string) *Tool {
	return NewToolWithRuntime(executor, permissions, approvalMode, nil)
}

// NewToolWithRuntime constructs a Bash tool with explicit host shell, permission, approval-mode, and background-task dependencies.
func NewToolWithRuntime(executor ShellExecutor, permissions CommandPermissionChecker, approvalMode string, taskStore BackgroundTaskStore) *Tool {
	if strings.TrimSpace(approvalMode) == "" {
		approvalMode = "default"
	}
	backgroundExecutor, _ := executor.(BackgroundShellExecutor)
	return &Tool{
		executor:           executor,
		backgroundExecutor: backgroundExecutor,
		permissions:        permissions,
		approvalMode:       approvalMode,
		taskStore:          taskStore,
	}
}

// NewToolWithNotification constructs a Bash tool with notification emitter support for background task completion events.
func NewToolWithNotification(executor ShellExecutor, permissions CommandPermissionChecker, approvalMode string, taskStore BackgroundTaskStore, emitter NotificationEmitter) *Tool {
	t := NewToolWithRuntime(executor, permissions, approvalMode, taskStore)
	t.notificationEmitter = emitter
	return t
}

// NewToolWithSecurityScanner constructs a Bash tool with a pre-execution security scanner.
func NewToolWithSecurityScanner(executor ShellExecutor, permissions CommandPermissionChecker, approvalMode string, taskStore BackgroundTaskStore, emitter NotificationEmitter, scanner SecurityScanner) *Tool {
	t := NewToolWithNotification(executor, permissions, approvalMode, taskStore, emitter)
	t.securityScanner = scanner
	return t
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
	if input.DangerouslyDisableSandbox {
		return coretool.Result{Error: "Sandbox override is not available in Claude Code Go yet."}, nil
	}

	timeoutMilliseconds, err := resolveTimeoutMilliseconds(input.Timeout)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	if t.approvalMode == "bypassPermissions" {
		return t.executeAllowedCommand(ctx, call, input, command, timeoutMilliseconds)
	}

	evaluation := t.permissions.Check(command)
	normalizedCommand := strings.TrimSpace(evaluation.NormalizedCommand)
	if normalizedCommand == "" {
		normalizedCommand = command
	}

	if corepermission.HasBashGrant(ctx, corepermission.BashRequest{
		ToolName:   call.Name,
		Command:    normalizedCommand,
		WorkingDir: call.Context.WorkingDir,
	}) {
		logger.DebugCF("bash_tool", "bash command allowed by runtime grant", map[string]any{
			"working_dir": call.Context.WorkingDir,
			"command":     normalizedCommand,
		})
		return t.executeAllowedCommand(ctx, call, input, command, timeoutMilliseconds)
	}

	switch evaluation.Decision {
	case corepermission.DecisionDeny:
		return coretool.Result{
			Error: evaluation.Message,
			Meta: map[string]any{
				"permission_decision": string(evaluation.Decision),
				"permission_rule":     evaluation.Rule,
			},
		}, nil
	case corepermission.DecisionAsk:
		if t.approvalMode == "acceptEdits" && isAcceptEditsAutoAllowedCommand(normalizedCommand) {
			return t.executeAllowedCommand(ctx, call, input, command, timeoutMilliseconds)
		}
		if t.approvalMode == "dontAsk" {
			return coretool.Result{
				Error: fmt.Sprintf("Permission to execute %q was not granted.", normalizedCommand),
				Meta: map[string]any{
					"permission_decision": string(corepermission.DecisionDeny),
					"permission_rule":     evaluation.Rule,
				},
			}, nil
		}
		return coretool.Result{}, &corepermission.BashPermissionError{
			ToolName:   call.Name,
			Command:    normalizedCommand,
			WorkingDir: call.Context.WorkingDir,
			Decision:   corepermission.DecisionAsk,
			Rule:       evaluation.Rule,
			Message:    evaluation.Message,
		}
	default:
		return t.executeAllowedCommand(ctx, call, input, command, timeoutMilliseconds)
	}
}

// executeAllowedCommand dispatches one already-authorized Bash invocation to the foreground or background execution path.
func (t *Tool) executeAllowedCommand(ctx context.Context, call coretool.Call, input Input, command string, timeoutMilliseconds int) (coretool.Result, error) {
	// Sed safety validation: dangerous sed operations are blocked even when the
	// command is otherwise authorized. This mirrors the TypeScript
	// checkSedConstraints defense-in-depth layer.
	trimmed := strings.TrimSpace(command)
	if strings.HasPrefix(trimmed, "sed") {
		rest := trimmed[len("sed"):]
		if len(rest) > 0 && (rest[0] == ' ' || rest[0] == '\t') {
			allowFileWrites := t.approvalMode == "acceptEdits"
			if !sedCommandIsAllowed(command, allowFileWrites) {
				return coretool.Result{}, &corepermission.BashPermissionError{
					ToolName:   call.Name,
					Command:    command,
					WorkingDir: call.Context.WorkingDir,
					Decision:   corepermission.DecisionAsk,
					Message:    "sed command requires approval (contains potentially dangerous operations)",
				}
			}
		}
	}

	// Security scan: defense-in-depth layer that checks for dangerous command
	// patterns even when the command is otherwise authorized by permission rules.
	if t.securityScanner != nil {
		scanResult := t.securityScanner.Scan(command)
		switch scanResult.RiskLevel {
		case RiskLevelDangerous:
			logger.DebugCF("bash_tool", "bash command blocked by security scanner", map[string]any{
				"command":         command,
				"matched_pattern": scanResult.MatchedPattern,
			})
			return coretool.Result{}, &corepermission.BashPermissionError{
				ToolName:   call.Name,
				Command:    command,
				WorkingDir: call.Context.WorkingDir,
				Decision:   corepermission.DecisionDeny,
				Message:    scanResult.Message,
			}
		case RiskLevelWarning:
			logger.DebugCF("bash_tool", "bash command requires approval by security scanner", map[string]any{
				"command":         command,
				"matched_pattern": scanResult.MatchedPattern,
			})
			return coretool.Result{}, &corepermission.BashPermissionError{
				ToolName:   call.Name,
				Command:    command,
				WorkingDir: call.Context.WorkingDir,
				Decision:   corepermission.DecisionAsk,
				Message:    scanResult.Message,
			}
		}
	}

	if input.RunInBackground {
		return t.startBackgroundCommand(call, input, command, timeoutMilliseconds)
	}
	// When switchable execution is available and the command is eligible for
	// auto-backgrounding, use the switchable path so long-running commands
	// can be moved to the background automatically.
	if t.backgroundExecutor != nil && t.taskStore != nil && isAutobackgroundingAllowed(command) {
		return t.executeSwitchableCommand(ctx, call, input, command, timeoutMilliseconds)
	}
	return t.executeForegroundCommand(ctx, call, command, timeoutMilliseconds)
}

// isAutobackgroundingAllowed reports whether the command is eligible for
// automatic backgrounding. It mirrors the TypeScript DISALLOWED_AUTO_BACKGROUND
// and COMMON_BACKGROUND_COMMANDS heuristics.
func isAutobackgroundingAllowed(command string) bool {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return false
	}
	// Disallow simple sleep commands — they should run in foreground unless
	// explicitly backgrounded by the user.
	fields := strings.Fields(trimmed)
	if len(fields) > 0 && fields[0] == "sleep" {
		return false
	}
	return true
}

// executeForegroundCommand runs one normalized foreground Bash request and maps the process result into the stable tool result shape.
func (t *Tool) executeForegroundCommand(ctx context.Context, call coretool.Call, command string, timeoutMilliseconds int) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("bash tool: nil receiver")
	}
	if t.executor == nil {
		return coretool.Result{}, fmt.Errorf("bash tool: executor is not configured")
	}

	// Parse output redirection before execution so the captured file path is
	// recorded even when the command fails or times out.
	redirect := parseOutputRedirection(command)
	execCommand := redirect.Command
	if execCommand == "" {
		execCommand = command
	}

	logger.DebugCF("bash_tool", "starting foreground bash tool invocation", map[string]any{
		"working_dir":    call.Context.WorkingDir,
		"timeout_ms":     timeoutMilliseconds,
		"command_len":    len(execCommand),
		"has_redirect":   redirect.StdoutFile != "" || redirect.StderrFile != "",
	})

	startTime := time.Now()
	req := platformshell.Request{
		Command:    execCommand,
		WorkingDir: call.Context.WorkingDir,
		Timeout:    time.Duration(timeoutMilliseconds) * time.Millisecond,
	}

	// When a progress callback is available in context, stream stdout line-by-line.
	if progressFn := coretool.GetProgress(ctx); progressFn != nil {
		req.OnStdoutLine = func(line string) {
			progressFn(BashProgressData{
				Output:         line,
				FullOutput:     "", // FullOutput not tracked per-line in minimal impl
				ElapsedSeconds: time.Since(startTime).Seconds(),
			})
		}
	}

	execResult, err := t.executor.Execute(ctx, req)
	elapsed := time.Since(startTime).Seconds()
	if err != nil {
		return coretool.Result{
			Error: fmt.Sprintf("bash tool: %v", err),
		}, nil
	}

	output := Output{
		Command:        execCommand,
		Stdout:         execResult.Stdout,
		Stderr:         execResult.Stderr,
		ExitCode:       execResult.ExitCode,
		TimedOut:       execResult.TimedOut,
		ElapsedSeconds: elapsed,
	}

	// Record captured output file metadata when the command used redirection.
	if redirect.StdoutFile != "" {
		output.OutputFilePath = redirect.StdoutFile
		// Best-effort file size; the file may not be flushed yet.
		if info, err := os.Stat(redirect.StdoutFile); err == nil {
			output.OutputFileSize = info.Size()
		}
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
		"exit_code":      output.ExitCode,
		"timed_out":      output.TimedOut,
		"stdout_len":     len(output.Stdout),
		"stderr_len":     len(output.Stderr),
		"elapsed_sec":    output.ElapsedSeconds,
		"output_file":    output.OutputFilePath,
	})

	return coretool.Result{
		Output: renderSuccess(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// startBackgroundCommand launches one authorized Bash command in the background and registers it in the shared runtime task store.
func (t *Tool) startBackgroundCommand(call coretool.Call, input Input, command string, timeoutMilliseconds int) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("bash tool: nil receiver")
	}
	if t.backgroundExecutor == nil {
		return coretool.Result{Error: "Background Bash tasks are not available in Claude Code Go yet."}, nil
	}
	if t.taskStore == nil {
		return coretool.Result{Error: "Background Bash tasks are not available in Claude Code Go yet."}, nil
	}

	process, err := t.backgroundExecutor.Start(platformshell.Request{
		Command:    command,
		WorkingDir: call.Context.WorkingDir,
		Timeout:    time.Duration(timeoutMilliseconds) * time.Millisecond,
	})
	if err != nil {
		return coretool.Result{
			Error: fmt.Sprintf("bash tool: %v", err),
		}, nil
	}

	output := BackgroundOutput{
		TaskID:  newBackgroundTaskID(),
		Command: command,
		Summary: summarizeBackgroundCommand(input.Description, command),
	}
	snapshot := coresession.BackgroundTaskSnapshot{
		ID:                output.TaskID,
		Type:              "bash",
		Status:            coresession.BackgroundTaskStatusRunning,
		Summary:           output.Summary,
		ControlsAvailable: true,
	}
	t.taskStore.Register(snapshot, process)

	logger.DebugCF("bash_tool", "started background bash task", map[string]any{
		"task_id":     output.TaskID,
		"working_dir": call.Context.WorkingDir,
		"timeout_ms":  timeoutMilliseconds,
	})

	go t.consumeBackgroundResult(output.TaskID, snapshot, process)

	return coretool.Result{
		Output: renderBackgroundStart(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// consumeBackgroundResult observes the background process completion, removes the task from the shared active-task store, and emits a completion notification.
func (t *Tool) consumeBackgroundResult(taskID string, snapshot coresession.BackgroundTaskSnapshot, process platformshell.BackgroundProcess) {
	if t == nil || t.taskStore == nil || process == nil {
		return
	}

	resultCh := process.Result()
	if resultCh == nil {
		return
	}

	result, ok := <-resultCh
	if !ok {
		return
	}

	var notifyStatus string
	switch {
	case result.TimedOut:
		snapshot.Status = coresession.BackgroundTaskStatusFailed
		notifyStatus = "failed"
	case result.ExitCode == 0:
		snapshot.Status = coresession.BackgroundTaskStatusCompleted
		notifyStatus = "completed"
	case result.Canceled:
		snapshot.Status = coresession.BackgroundTaskStatusStopped
		notifyStatus = "killed"
	default:
		snapshot.Status = coresession.BackgroundTaskStatusFailed
		notifyStatus = "failed"
	}

	t.taskStore.Update(snapshot)
	t.taskStore.Remove(taskID)

	// Emit notification if an emitter is configured.
	if t.notificationEmitter != nil {
		emitBackgroundCompletionNotification(
			t.notificationEmitter,
			taskID,
			snapshot.Summary,
			notifyStatus,
			result.ExitCode,
			"", // outputPath not yet tracked per-task in current store
		)
	}

	logger.DebugCF("bash_tool", "background bash task finished", map[string]any{
		"task_id":     taskID,
		"exit_code":   result.ExitCode,
		"timed_out":   result.TimedOut,
		"final_state": snapshot.Status,
	})
}

// isAcceptEditsAutoAllowedCommand reports whether the normalized Bash command falls inside the minimal acceptEdits auto-allow subset.
func isAcceptEditsAutoAllowedCommand(command string) bool {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "&&") || strings.Contains(trimmed, "||") || strings.Contains(trimmed, ";") || strings.Contains(trimmed, "|") {
		return false
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return false
	}

	switch fields[0] {
	case "mkdir", "touch", "rm", "rmdir", "mv", "cp", "sed":
		return true
	default:
		return false
	}
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
				Description: "Set to true to run the command in the background and return a task identifier.",
			},
			"dangerouslyDisableSandbox": {
				Type:        coretool.ValueKindBoolean,
				Description: "Set to true to disable the sandbox. Not yet supported in Claude Code Go.",
			},
		},
	}
}

// newBackgroundTaskID returns one stable task identifier for a newly started background Bash task.
func newBackgroundTaskID() string {
	return fmt.Sprintf("task_%s", strings.ReplaceAll(uuid.NewString(), "-", ""))
}

// summarizeBackgroundCommand returns the minimum user-visible label used for one background Bash task.
func summarizeBackgroundCommand(description string, command string) string {
	if trimmed := strings.TrimSpace(description); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(command)
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

	var body string
	switch {
	case stdout == "" && stderr == "":
		body = "Done"
	case stdout != "" && stderr == "":
		body = stdout
	case stdout == "" && stderr != "":
		body = fmt.Sprintf("stderr:\n%s", stderr)
	default:
		body = fmt.Sprintf("stdout:\n%s\n\nstderr:\n%s", stdout, stderr)
	}

	return appendOutputFileInfo(body, output)
}

// appendOutputFileInfo appends captured output-file metadata to a rendered payload when present.
func appendOutputFileInfo(body string, output Output) string {
	if output.OutputFilePath == "" {
		return body
	}
	var info string
	if output.OutputFileSize > 0 {
		info = fmt.Sprintf("\n\nOutput written to: %s (%d bytes)", output.OutputFilePath, output.OutputFileSize)
	} else {
		info = fmt.Sprintf("\n\nOutput written to: %s", output.OutputFilePath)
	}
	return body + info
}

// renderBackgroundStart converts one background Bash task creation into a stable caller-facing text payload.
func renderBackgroundStart(output BackgroundOutput) string {
	return fmt.Sprintf("Started background task: %s\nCommand: %s", output.TaskID, output.Command)
}

// renderFailure converts one non-zero exit result into a stable caller-facing error payload.
func renderFailure(output Output) string {
	details := renderStreamDetails(output.Stdout, output.Stderr)
	var body string
	if details == "" {
		body = fmt.Sprintf("Command failed with exit code %d.", output.ExitCode)
	} else {
		body = fmt.Sprintf("Command failed with exit code %d.\n%s", output.ExitCode, details)
	}
	return appendOutputFileInfo(body, output)
}

// renderTimeout converts one timed-out foreground result into a stable caller-facing error payload.
func renderTimeout(output Output, timeoutMilliseconds int) string {
	details := renderStreamDetails(output.Stdout, output.Stderr)
	var body string
	if details == "" {
		body = fmt.Sprintf("Command timed out after %dms.", timeoutMilliseconds)
	} else {
		body = fmt.Sprintf("Command timed out after %dms.\n%s", timeoutMilliseconds, details)
	}
	return appendOutputFileInfo(body, output)
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

// BashProgressData carries one incremental stdout update emitted by a running Bash command.
type BashProgressData struct {
	// Output stores the most recently received stdout lines.
	Output string `json:"output"`
	// FullOutput stores the complete stdout accumulated so far.
	FullOutput string `json:"fullOutput"`
	// ElapsedSeconds stores the wall-clock seconds since the command started.
	ElapsedSeconds float64 `json:"elapsedSeconds"`
}
