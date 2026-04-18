package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// exitCodeCanceled is the synthetic exit code used when one background process is explicitly stopped.
	exitCodeCanceled = -2
)

// BackgroundProcess exposes the minimum host-process handle needed by background task management.
type BackgroundProcess interface {
	// Stop requests termination of the running process.
	Stop() error
	// Result returns the eventual normalized execution result for the process.
	Result() <-chan Result
}

type runningBackgroundProcess struct {
	// cancel terminates the process context created for this background execution.
	cancel context.CancelFunc
	// result delivers the final normalized process outcome.
	result chan Result
	// stopOnce keeps repeated stop requests idempotent.
	stopOnce sync.Once
}

// Stop requests termination of the running process exactly once.
func (p *runningBackgroundProcess) Stop() error {
	if p == nil || p.cancel == nil {
		return fmt.Errorf("background process: stop is not available")
	}

	p.stopOnce.Do(func() {
		p.cancel()
	})
	return nil
}

// Result returns the channel that receives the final normalized process result.
func (p *runningBackgroundProcess) Result() <-chan Result {
	if p == nil {
		return nil
	}
	return p.result
}

// Start launches one shell command in the background and returns a process handle for later stop/result handling.
func (e *Executor) Start(req Request) (BackgroundProcess, error) {
	if e == nil {
		return nil, fmt.Errorf("shell executor: nil receiver")
	}
	if strings.TrimSpace(req.Command) == "" {
		return nil, fmt.Errorf("shell executor: command is required")
	}

	shellPath, prefixArgs := e.lookupShell()
	runCtx, cancel := context.WithCancel(context.Background())
	args := append(append([]string{}, prefixArgs...), req.Command)
	cmd := exec.CommandContext(runCtx, shellPath, args...)
	if strings.TrimSpace(req.WorkingDir) != "" {
		cmd.Dir = req.WorkingDir
	}
	cmd.Env = mergeEnvironment(e.environ(), req.Env)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("shell executor: start command: %w", err)
	}

	process := &runningBackgroundProcess{
		cancel: cancel,
		result: make(chan Result, 1),
	}

	var timedOut atomic.Bool
	var timeoutTimer *time.Timer
	if req.Timeout > 0 {
		timeoutTimer = time.AfterFunc(req.Timeout, func() {
			timedOut.Store(true)
			cancel()
		})
	}

	logger.DebugCF("shell_executor", "started background shell command", map[string]any{
		"shell_path":   shellPath,
		"working_dir":  req.WorkingDir,
		"timeout_ms":   req.Timeout.Milliseconds(),
		"command_size": len(req.Command),
	})

	go func() {
		defer close(process.result)
		defer cancel()
		if timeoutTimer != nil {
			defer timeoutTimer.Stop()
		}

		err := cmd.Wait()
		result := Result{
			Command:  req.Command,
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: exitCodeSuccess,
		}

		switch {
		case err == nil:
		case timedOut.Load():
			result.TimedOut = true
			result.ExitCode = exitCodeTimeout
		case errors.Is(runCtx.Err(), context.Canceled):
			result.ExitCode = exitCodeCanceled
			result.Canceled = true
		default:
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				result.ExitCode = exitErr.ExitCode()
			} else {
				result.ExitCode = exitCodeCanceled
				if result.Stderr == "" {
					result.Stderr = err.Error()
				}
			}
		}

		logger.DebugCF("shell_executor", "background shell command finished", map[string]any{
			"exit_code":  result.ExitCode,
			"timed_out":  result.TimedOut,
			"stdout_len": len(result.Stdout),
			"stderr_len": len(result.Stderr),
		})

		process.result <- result
	}()

	return process, nil
}
