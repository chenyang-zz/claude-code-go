package powershell

import (
	"context"
	"fmt"
	"os"
	"regexp"
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
	// Name is the stable registry identifier used by the PowerShell tool.
	Name = "PowerShell"

	// defaultTimeoutMilliseconds is the default PowerShell command timeout when no env override is configured.
	defaultTimeoutMilliseconds = 120000

	// maxTimeoutMilliseconds is the maximum PowerShell command timeout when no env override is configured.
	maxTimeoutMilliseconds = 600000
)

// ShellExecutor describes the foreground shell execution dependency for PowerShell commands.
type ShellExecutor interface {
	// Execute runs one normalized foreground shell request and returns the result.
	Execute(ctx context.Context, req platformshell.Request) (platformshell.Result, error)
}

// BackgroundShellExecutor describes the background execution dependency.
type BackgroundShellExecutor interface {
	// Start launches one normalized background shell request and returns a process handle.
	Start(req platformshell.Request) (platformshell.BackgroundProcess, error)
}

// BackgroundTaskStore describes the shared lifecycle store used by background tasks.
type BackgroundTaskStore interface {
	// Register inserts one new live background task snapshot into the shared store.
	Register(task coresession.BackgroundTaskSnapshot, stopper interface{ Stop() error })
	// Update replaces the stored snapshot for one existing task.
	Update(task coresession.BackgroundTaskSnapshot) bool
	// Remove deletes one task from the shared task list.
	Remove(id string)
}

// NotificationEmitter sends background-task completion notifications.
type NotificationEmitter interface {
	// EmitTaskNotification fires a notification for a completed background task.
	EmitTaskNotification(taskID string, status string, summary string, outputPath string)
}

// Tool implements the PowerShell tool that executes commands via pwsh/powershell.exe.
type Tool struct {
	// executor runs the normalized shell request in the host environment.
	executor ShellExecutor
	// backgroundExecutor launches background commands when run_in_background is requested.
	backgroundExecutor BackgroundShellExecutor
	// permissions checks PowerShell(...) allow/deny/ask rules before execution starts.
	permissions CommandPermissionChecker
	// approvalMode stores the current default approval mode.
	approvalMode string
	// securityScanner performs pre-execution security checks on allowed commands.
	securityScanner *SecurityScanner
	// taskStore exposes the shared runtime task lifecycle store used by background tasks.
	taskStore BackgroundTaskStore
	// notificationEmitter sends background-task completion notifications.
	notificationEmitter NotificationEmitter
}

// Input stores the typed request payload accepted by the PowerShell tool.
type Input struct {
	Command         string `json:"command"`
	Timeout         int    `json:"timeout,omitempty"`
	Description     string `json:"description,omitempty"`
	RunInBackground bool   `json:"run_in_background,omitempty"`
}

// Output stores the structured result metadata returned by the PowerShell tool.
type Output struct {
	Command                  string  `json:"command"`
	Stdout                   string  `json:"stdout"`
	Stderr                   string  `json:"stderr"`
	ExitCode                 int     `json:"exitCode"`
	TimedOut                 bool    `json:"timedOut"`
	ElapsedSecs              float64 `json:"elapsedSeconds,omitempty"`
	ReturnCodeInterpretation string  `json:"returnCodeInterpretation,omitempty"`
	BackgroundTaskID         string  `json:"backgroundTaskId,omitempty"`
}

// NewTool constructs a PowerShell tool with the given executor and permission checker.
func NewTool(executor ShellExecutor, permissions CommandPermissionChecker, approvalMode string) *Tool {
	return &Tool{
		executor:        executor,
		permissions:     permissions,
		approvalMode:    approvalMode,
		securityScanner: NewSecurityScanner(),
	}
}

// NewToolWithRuntime constructs a PowerShell tool with background task support.
func NewToolWithRuntime(executor ShellExecutor, permissions CommandPermissionChecker, approvalMode string, taskStore BackgroundTaskStore) *Tool {
	t := NewTool(executor, permissions, approvalMode)
	backgroundExecutor, _ := executor.(BackgroundShellExecutor)
	t.backgroundExecutor = backgroundExecutor
	t.taskStore = taskStore
	return t
}

// NewToolWithNotification constructs a PowerShell tool with full runtime support including notifications.
func NewToolWithNotification(executor ShellExecutor, permissions CommandPermissionChecker, approvalMode string, taskStore BackgroundTaskStore, emitter NotificationEmitter) *Tool {
	t := NewToolWithRuntime(executor, permissions, approvalMode, taskStore)
	t.notificationEmitter = emitter
	return t
}

// Name returns the stable registration name.
func (t *Tool) Name() string {
	return Name
}

// Description returns the summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return Description()
}

// InputSchema returns the PowerShell tool input contract.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
}

// IsReadOnly reports that PowerShell commands may mutate external state.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that arbitrary PowerShell commands should not be assumed safe to run in parallel.
func (t *Tool) IsConcurrencySafe() bool {
	return false
}

// Invoke validates input, enforces permission rules, runs the PowerShell command, and normalizes the result.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("powershell tool: nil receiver")
	}
	if t.executor == nil {
		return coretool.Result{}, fmt.Errorf("powershell tool: executor is not configured")
	}
	if t.permissions == nil {
		return coretool.Result{}, fmt.Errorf("powershell tool: permission checker is not configured")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	command := strings.TrimSpace(input.Command)
	if command == "" {
		return coretool.Result{Error: "command is required"}, nil
	}

	timeoutMilliseconds, err := resolveTimeout(input.Timeout)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}


	if t.approvalMode == "bypassPermissions" {
		return t.executeOrBackground(ctx, call, input, command, timeoutMilliseconds)
	}

	// Enhanced permission check: rules + sub-commands + allowlist + provider paths + mode
	scanResult := t.securityScanner.Scan(command)
	permDecision := checkPermission(t.permissions, command, scanResult, t.approvalMode)

	normalizedCommand := strings.TrimSpace(permDecision.Evaluation.NormalizedCommand)
	if normalizedCommand == "" {
		normalizedCommand = command
	}

	if corepermission.HasBashGrant(ctx, corepermission.BashRequest{
		ToolName:   call.Name,
		Command:    normalizedCommand,
		WorkingDir: call.Context.WorkingDir,
	}) {
		logger.DebugCF("powershell_tool", "command allowed by runtime grant", map[string]any{
			"command": normalizedCommand,
		})
		return t.executeOrBackground(ctx, call, input, command, timeoutMilliseconds)
	}

	switch permDecision.Evaluation.Decision {
	case corepermission.DecisionDeny:
		return coretool.Result{
			Error: permDecision.Evaluation.Message,
			Meta: map[string]any{
				"permission_decision": string(permDecision.Evaluation.Decision),
				"permission_rule":     permDecision.Evaluation.Rule,
			},
		}, nil
	case corepermission.DecisionAsk:
		return coretool.Result{}, &corepermission.BashPermissionError{
			ToolName:   call.Name,
			Command:    normalizedCommand,
			WorkingDir: call.Context.WorkingDir,
			Decision:   corepermission.DecisionAsk,
			Rule:       permDecision.Evaluation.Rule,
			Message:    permDecision.Evaluation.Message,
		}
	default:
		return t.executeCommand(ctx, call, command, timeoutMilliseconds)
	}
}

// executeCommand runs the PowerShell command and returns the result.
// executeOrBackground routes to foreground or background based on RunInBackground flag.
func (t *Tool) executeOrBackground(ctx context.Context, call coretool.Call, input Input, command string, timeoutMilliseconds int) (coretool.Result, error) {
	if input.RunInBackground {
		if t.backgroundExecutor == nil {
			return coretool.Result{Error: "Background execution is not supported (no background executor configured)"}, nil
		}
		return t.startBackgroundCommand(ctx, call, input, command, timeoutMilliseconds)
	}
	return t.executeCommand(ctx, call, command, timeoutMilliseconds)
}

// PowerShellProgressData carries one incremental stdout update emitted by a running command.
type PowerShellProgressData struct {
	Output         string  `json:"output"`
	FullOutput     string  `json:"fullOutput"`
	ElapsedSeconds float64 `json:"elapsedSeconds"`
}

func (t *Tool) executeCommand(ctx context.Context, call coretool.Call, command string, timeoutMilliseconds int) (coretool.Result, error) {
	startTime := time.Now()

	// Wire up progress reporting via context-based ProgressFunc.
	// This enables the runtime to stream incremental output to the user.
	req := platformshell.Request{
		Command:    command,
		WorkingDir: call.Context.WorkingDir,
		Timeout:    time.Duration(timeoutMilliseconds) * time.Millisecond,
	}
	if progressFn := coretool.GetProgress(ctx); progressFn != nil {
		var fullOutput strings.Builder
		req.OnStdoutLine = func(line string) {
			fullOutput.WriteString(line)
			progressFn(PowerShellProgressData{
				Output:         line,
				FullOutput:     fullOutput.String(),
				ElapsedSeconds: time.Since(startTime).Seconds(),
			})
		}
	}

	shellResult, err := t.executor.Execute(ctx, req)
	elapsed := time.Since(startTime).Seconds()

	if err != nil {
		if ctx.Err() != nil {
			return coretool.Result{Error: "Command was aborted"}, nil
		}
		return coretool.Result{Error: fmt.Sprintf("Failed to execute command: %v", err)}, nil
	}

	// Interpret exit code using semantic rules
	semantic := interpretCommandResult(command, shellResult.ExitCode, shellResult.Stdout, shellResult.Stderr)

	var resultErr string
	if semantic.isError {
		if semantic.message != "" {
			resultErr = semantic.message
		} else if shellResult.ExitCode != 0 {
			resultErr = fmt.Sprintf("Command failed with exit code %d", shellResult.ExitCode)
		}
	}

	output := Output{
		Command:                  command,
		Stdout:                   shellResult.Stdout,
		Stderr:                   shellResult.Stderr,
		ExitCode:                 shellResult.ExitCode,
		TimedOut:                 shellResult.TimedOut,
		ElapsedSecs:              elapsed,
		ReturnCodeInterpretation: semantic.message,
	}

	if resultErr != "" {
		return coretool.Result{Error: resultErr}, nil
	}

	rendered := renderSuccess(output)

	// Check for image output (data URI) and set Meta["image"] for runtime
	// content block conversion.
	result := coretool.Result{
		Output: rendered,
		Meta:   imageMetaFromOutput(shellResult.Stdout),
	}

	return result, nil
}

// dataURIPattern matches a base64-encoded image data URI.
var dataURIPattern = regexp.MustCompile(`^data:([^;]+);base64,(.+)$`)

// imageMetaFromOutput checks if stdout is a data URI and returns Meta["image"].
func imageMetaFromOutput(stdout string) map[string]any {
	trimmed := strings.TrimSpace(stdout)
	matches := dataURIPattern.FindStringSubmatch(trimmed)
	if len(matches) < 3 {
		return nil
	}
	return map[string]any{
		"image": coretool.ImageData{
			MediaType: matches[1],
			Base64:    matches[2],
		},
	}
}

// startBackgroundCommand launches a PowerShell command in the background and returns a task ID.
func (t *Tool) startBackgroundCommand(_ context.Context, call coretool.Call, input Input, command string, timeoutMilliseconds int) (coretool.Result, error) {
	bgProcess, err := t.backgroundExecutor.Start(platformshell.Request{
		Command:    command,
		WorkingDir: call.Context.WorkingDir,
		Timeout:    time.Duration(timeoutMilliseconds) * time.Millisecond,
	})
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("Failed to start background command: %v", err)}, nil
	}

	taskID := newBackgroundTaskID()
	summary := summarizeBackgroundCommand(input.Description, command)

	if t.taskStore != nil {
		t.taskStore.Register(coresession.BackgroundTaskSnapshot{
			ID:                taskID,
			Type:              "shell",
			Summary:           summary,
			Status:            coresession.BackgroundTaskStatusRunning,
			ControlsAvailable: true,
		}, bgProcess)
	}

	// Monitor background completion and emit notification
	go func() {
		result := <-bgProcess.Result()

		var status coresession.BackgroundTaskStatus
		switch {
		case result.TimedOut:
			status = coresession.BackgroundTaskStatusFailed
		case result.Canceled:
			status = coresession.BackgroundTaskStatusStopped
		case result.ExitCode != 0:
			status = coresession.BackgroundTaskStatusFailed
		default:
			status = coresession.BackgroundTaskStatusCompleted
		}

		if t.taskStore != nil {
			t.taskStore.Update(coresession.BackgroundTaskSnapshot{
				ID:                taskID,
				Type:              "shell",
				Summary:           summary,
				Status:            status,
				ControlsAvailable: false,
			})
		}

		emitStatus := string(status)
		emitNotification := t.notificationEmitter != nil
		if emitNotification {
			interpret := interpretCommandResult(command, result.ExitCode, result.Stdout, result.Stderr)
			outputPath := ""
			outputSummary := summary
			if interpret.message != "" {
				outputSummary = summary + ": " + interpret.message
			}
			t.notificationEmitter.EmitTaskNotification(taskID, emitStatus, outputSummary, outputPath)
		}
		// Remove from store after notification
		if t.taskStore != nil {
			t.taskStore.Remove(taskID)
		}
	}()

	return coretool.Result{
		Output: fmt.Sprintf("Command running in background with ID: %s. You will be notified when it completes.", taskID),
		Meta: map[string]any{
			"backgroundTaskId": taskID,
		},
	}, nil
}

// inputSchema returns the PowerShell tool input schema.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"command": {
				Type:        coretool.ValueKindString,
				Description: "The PowerShell command to execute.",
				Required:    true,
			},
			"timeout": {
				Type:        coretool.ValueKindInteger,
				Description: fmt.Sprintf("Optional timeout in milliseconds (max %d).", effectiveMaxTimeout()),
			},
			"description": {
				Type:        coretool.ValueKindString,
				Description: "Optional short description of what the command does.",
			},
			"run_in_background": {
				Type:        coretool.ValueKindBoolean,
				Description: "Set to true to run the command in the background and return a task identifier.",
			},
		},
	}
}

// resolveTimeout normalizes the optional caller-supplied timeout against the current defaults and cap.
func resolveTimeout(value int) (int, error) {
	timeoutMs := value
	if timeoutMs == 0 {
		timeoutMs = effectiveDefaultTimeout()
	}
	if timeoutMs < 0 {
		return 0, fmt.Errorf("timeout must be greater than or equal to 0")
	}
	if timeoutMs > effectiveMaxTimeout() {
		return 0, fmt.Errorf("timeout must be less than or equal to %d", effectiveMaxTimeout())
	}
	return timeoutMs, nil
}

// effectiveDefaultTimeout returns the default timeout, overridable via POWERSHELL_DEFAULT_TIMEOUT_MS env.
func effectiveDefaultTimeout() int {
	return parsePositiveIntEnv("POWERSHELL_DEFAULT_TIMEOUT_MS", defaultTimeoutMilliseconds)
}

// effectiveMaxTimeout returns the maximum timeout, overridable via POWERSHELL_MAX_TIMEOUT_MS env.
func effectiveMaxTimeout() int {
	max := parsePositiveIntEnv("POWERSHELL_MAX_TIMEOUT_MS", maxTimeoutMilliseconds)
	defaultT := effectiveDefaultTimeout()
	if max < defaultT {
		return defaultT
	}
	return max
}

// parsePositiveIntEnv reads one optional positive integer environment value and falls back when absent.
func parsePositiveIntEnv(name string, fallback int) int {
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

// newBackgroundTaskID returns one stable task identifier for a newly started background task.
func newBackgroundTaskID() string {
	return fmt.Sprintf("task_%s", strings.ReplaceAll(uuid.NewString(), "-", ""))
}

// summarizeBackgroundCommand returns the minimum user-visible label used for one background task.
func summarizeBackgroundCommand(description string, command string) string {
	if trimmed := strings.TrimSpace(description); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(command)
}

// renderSuccess converts a successful PowerShell execution result to text.
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

	if output.TimedOut {
		body += "\n\n<error>Command timed out</error>"
	}

	return body
}


// isSearchOrReadPowerShellCommand checks if a command is search/read for collapsible display.
func isSearchOrReadPowerShellCommand(command string) (isSearch, isRead bool) {
    trimmed := strings.TrimSpace(command)
    if trimmed == "" {
        return false, false
    }

    psSearch := map[string]bool{"select-string": true, "get-childitem": true, "findstr": true}
    psRead := map[string]bool{
        "get-content": true, "get-item": true, "test-path": true,
        "resolve-path": true, "get-process": true, "get-service": true,
        "get-childitem": true, "get-location": true, "get-filehash": true,
        "get-acl": true, "format-hex": true,
    }
    psNeutral := map[string]bool{"write-output": true, "write-host": true}

    parts := strings.FieldsFunc(trimmed, func(r rune) bool {
        return r == ';' || r == '|'
    })
    if len(parts) == 0 {
        return false, false
    }

    var hasSearch, hasRead, hasNonNeutral bool
    for _, part := range parts {
        part = strings.TrimSpace(part)
        if part == "" {
            continue
        }
        tokens := strings.Fields(part)
        if len(tokens) == 0 {
            continue
        }
        canonical := resolvePSCommand(tokens[0])
        if psNeutral[canonical] {
            continue
        }
        hasNonNeutral = true
        if psSearch[canonical] {
            hasSearch = true
        } else if psRead[canonical] {
            hasRead = true
        } else {
            return false, false
        }
    }
    return hasSearch && hasNonNeutral, hasRead && hasNonNeutral
}

// detectBlockedSleepPattern detects blocking Start-Sleep patterns.
func detectBlockedSleepPattern(command string) string {
    first := command
    if idx := strings.IndexAny(first, ";|&"); idx >= 0 {
        first = first[:idx]
    }
    first = strings.TrimSpace(first)
    if first == "" {
        return ""
    }

    re := regexp.MustCompile(`(?i)^(?:start-sleep|sleep)(?:\s+-s(?:econds)?)?\s+(\d+)\s*$`)
    m := re.FindStringSubmatch(first)
    if m == nil {
        return ""
    }
    secs, _ := strconv.Atoi(m[1])
    if secs < 2 {
        return ""
    }

    rest := strings.TrimSpace(command[len(first):])
    if rest != "" {
        return fmt.Sprintf("Start-Sleep %d followed by: %s", secs, rest)
    }
    return fmt.Sprintf("standalone Start-Sleep %d", secs)
}
