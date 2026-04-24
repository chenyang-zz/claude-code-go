package bash

import (
	"context"
	"fmt"
	"os"
	"time"

	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformshell "github.com/sheepzhao/claude-code-go/internal/platform/shell"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// assistantBlockingBudgetMilliseconds is the maximum time the main agent
	// should block on a foreground Bash command before auto-backgrounding it.
	// Mirrors the TypeScript ASSISTANT_BLOCKING_BUDGET_MS constant.
	assistantBlockingBudgetMilliseconds = 15000
)

// switchableAutoBackgroundBudget controls the auto-background timer in
// executeSwitchableCommand. It defaults to the production budget but can be
// overridden in tests to avoid long sleeps.
var switchableAutoBackgroundBudget = assistantBlockingBudgetMilliseconds * time.Millisecond

// executeSwitchableCommand runs one Bash command that may be moved from
// foreground to background while it is still running. It uses the background
// executor to start the process so that the foreground goroutine can release
// without terminating the underlying OS process.
//
// This is a simplified migration of the TypeScript runShellCommand backgrounding
// flow. Streamed stdout line-by-line is not supported in switchable mode because
// the background executor buffers output; callers that need streaming should use
// the standard executeForegroundCommand path.
func (t *Tool) executeSwitchableCommand(ctx context.Context, call coretool.Call, input Input, command string, timeoutMilliseconds int) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("bash tool: nil receiver")
	}
	if t.backgroundExecutor == nil {
		return coretool.Result{Error: "Switchable Bash execution is not available in Claude Code Go yet."}, nil
	}
	if t.taskStore == nil {
		return coretool.Result{Error: "Switchable Bash execution is not available in Claude Code Go yet."}, nil
	}

	// Parse output redirection so the captured file path is preserved even when
	// the command is backgrounded before it finishes.
	redirect := parseOutputRedirection(command)
	execCommand := redirect.Command
	if execCommand == "" {
		execCommand = command
	}

	logger.DebugCF("bash_tool", "starting switchable bash tool invocation", map[string]any{
		"working_dir":  call.Context.WorkingDir,
		"timeout_ms":   timeoutMilliseconds,
		"command_len":  len(execCommand),
		"has_redirect": redirect.StdoutFile != "" || redirect.StderrFile != "",
	})

	process, err := t.backgroundExecutor.Start(platformshell.Request{
		Command:    execCommand,
		WorkingDir: call.Context.WorkingDir,
		Timeout:    time.Duration(timeoutMilliseconds) * time.Millisecond,
	})
	if err != nil {
		return coretool.Result{
			Error: fmt.Sprintf("bash tool: %v", err),
		}, nil
	}

	// Register as a foreground-visible task so it can be listed and stopped.
	taskID := newBackgroundTaskID()
	summary := summarizeBackgroundCommand(input.Description, command)
	snapshot := coresession.BackgroundTaskSnapshot{
		ID:                taskID,
		Type:              "bash",
		Status:            coresession.BackgroundTaskStatusRunning,
		Summary:           summary,
		ControlsAvailable: true,
	}
	t.taskStore.Register(snapshot, process)

	resultCh := process.Result()

	// Assistant-mode auto-backgrounding: if the command is still running after
	// the blocking budget, return a background task ID so the agent stays
	// responsive. The process keeps running and a completion notification will
	// be fired later.
	autoBackgroundTimer := time.NewTimer(switchableAutoBackgroundBudget)
	defer autoBackgroundTimer.Stop()

	select {
	case result, ok := <-resultCh:
		if !ok {
			t.taskStore.Remove(taskID)
			return coretool.Result{Error: "bash tool: background result channel closed unexpectedly"}, nil
		}
		t.taskStore.Remove(taskID)
		output := Output{
			Command:        execCommand,
			Stdout:         result.Stdout,
			Stderr:         result.Stderr,
			ExitCode:       result.ExitCode,
			TimedOut:       result.TimedOut,
			ElapsedSeconds: float64(assistantBlockingBudgetMilliseconds) / 1000,
		}
		if redirect.StdoutFile != "" {
			output.OutputFilePath = redirect.StdoutFile
			if info, err := platformStat(redirect.StdoutFile); err == nil {
				output.OutputFileSize = info.Size()
			}
		}
		return t.renderResult(output, timeoutMilliseconds), nil

	case <-autoBackgroundTimer.C:
		// Auto-background: the process continues running. Transition the store
		// snapshot to background-visible and start the result consumer.
		go t.consumeBackgroundResult(taskID, snapshot, process)

		logger.DebugCF("bash_tool", "switchable bash command auto-backgrounded", map[string]any{
			"task_id":      taskID,
			"budget_ms":    assistantBlockingBudgetMilliseconds,
			"working_dir":  call.Context.WorkingDir,
		})

		output := BackgroundOutput{
			TaskID:  taskID,
			Command: command,
			Summary: summary,
		}
		return coretool.Result{
			Output: renderAutoBackgroundStart(output),
			Meta: map[string]any{
				"data": output,
			},
		}, nil

	case <-ctx.Done():
		// Context cancelled (e.g. user interrupt). Stop the process and clean up.
		_ = process.Stop()
		t.taskStore.Remove(taskID)
		return coretool.Result{
			Error: "Command was aborted before completion",
		}, nil
	}
}

// renderResult maps one shell Result into the stable tool result shape,
// handling success, failure, and timeout branches.
func (t *Tool) renderResult(output Output, timeoutMilliseconds int) coretool.Result {
	if output.TimedOut {
		return coretool.Result{
			Error: renderTimeout(output, timeoutMilliseconds),
			Meta: map[string]any{
				"data": output,
			},
		}
	}
	if output.ExitCode != 0 {
		return coretool.Result{
			Error: renderFailure(output),
			Meta: map[string]any{
				"data": output,
			},
		}
	}
	return coretool.Result{
		Output: renderSuccess(output),
		Meta: map[string]any{
			"data": output,
		},
	}
}

// renderAutoBackgroundStart converts one auto-backgrounded Bash task into a
// stable caller-facing text payload that explains the command is still running.
func renderAutoBackgroundStart(output BackgroundOutput) string {
	return fmt.Sprintf(
		"Command exceeded the assistant-mode blocking budget (%ds) and was moved to the background with ID: %s. It is still running — you will be notified when it completes.",
		assistantBlockingBudgetMilliseconds/1000,
		output.TaskID,
	)
}

// platformStat is a test-friendly abstraction for os.Stat.
var platformStat = func(name string) (os.FileInfo, error) {
	return os.Stat(name)
}
