package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// exitCodeSuccess is the stable success code returned by completed shell commands.
	exitCodeSuccess = 0
	// exitCodeTimeout is the synthetic exit code used when the host kills a command after timeout.
	exitCodeTimeout = -1
)

// Request stores one normalized shell execution request issued by the Bash tool.
type Request struct {
	// Command stores the shell source that should run in the selected interpreter.
	Command string
	// WorkingDir stores the process working directory used to resolve relative paths.
	WorkingDir string
	// Timeout bounds the foreground command duration when greater than zero.
	Timeout time.Duration
	// Env stores optional process-level environment overrides merged on top of the host environment.
	Env map[string]string
	// OnStdoutLine is an optional callback invoked for each stdout line produced by the command.
	// When set, the executor reads stdout through a pipe instead of buffering it all at once,
	// but still collects the full output in the returned Result.
	OnStdoutLine func(line string)
}

// Result stores the normalized foreground shell execution outcome returned to callers.
type Result struct {
	// Command echoes the command string that was executed for tracing and tests.
	Command string
	// Stdout stores the captured standard output stream.
	Stdout string
	// Stderr stores the captured standard error stream.
	Stderr string
	// ExitCode records the process exit code or a synthetic timeout code.
	ExitCode int
	// TimedOut reports whether the host terminated the process after exceeding Timeout.
	TimedOut bool
	// Canceled reports whether the host explicitly canceled the process.
	Canceled bool
}

// Executor runs foreground shell commands through the host shell implementation.
type Executor struct {
	// ShellLookup resolves the executable and argument prefix used for one request.
	ShellLookup func() (string, []string)
	// Environ returns the base environment inherited by child processes.
	Environ func() []string
}

// NewExecutor constructs the default foreground shell executor used by the migrated Bash tool.
func NewExecutor() *Executor {
	return &Executor{
		ShellLookup: defaultShellCommand,
		Environ:     os.Environ,
	}
}

// Execute runs one foreground shell command and normalizes stdout, stderr, exit code, and timeout state.
func (e *Executor) Execute(ctx context.Context, req Request) (Result, error) {
	if e == nil {
		return Result{}, fmt.Errorf("shell executor: nil receiver")
	}
	if strings.TrimSpace(req.Command) == "" {
		return Result{}, fmt.Errorf("shell executor: command is required")
	}

	shellPath, prefixArgs := e.lookupShell()
	runCtx := ctx
	cancel := func() {}
	if req.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, req.Timeout)
	}
	defer cancel()

	args := append(append([]string{}, prefixArgs...), req.Command)
	cmd := exec.CommandContext(runCtx, shellPath, args...)
	if strings.TrimSpace(req.WorkingDir) != "" {
		cmd.Dir = req.WorkingDir
	}
	cmd.Env = mergeEnvironment(e.environ(), req.Env)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	logger.DebugCF("shell_executor", "starting foreground shell command", map[string]any{
		"shell_path":   shellPath,
		"working_dir":  req.WorkingDir,
		"timeout_ms":   req.Timeout.Milliseconds(),
		"command_size": len(req.Command),
	})

	var stdoutStr string
	var runErr error
	if req.OnStdoutLine != nil {
		// Streaming mode: pipe stdout through a reader so callbacks receive line-oriented
		// updates without changing the exact stdout bytes returned to the caller.
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			return Result{}, fmt.Errorf("shell executor: stdout pipe: %w", err)
		}
		if err := cmd.Start(); err != nil {
			return Result{}, fmt.Errorf("shell executor: start command: %w", err)
		}

		var stdoutBuf bytes.Buffer
		readErrCh := make(chan error, 1)
		go func() {
			readErrCh <- streamStdout(stdoutPipe, &stdoutBuf, req.OnStdoutLine)
		}()

		runErr = cmd.Wait()
		readErr := <-readErrCh
		if readErr != nil {
			return Result{}, fmt.Errorf("shell executor: read stdout: %w", readErr)
		}
		stdoutStr = stdoutBuf.String()
	} else {
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		runErr = cmd.Run()
		stdoutStr = stdout.String()
	}

	result := Result{
		Command:  req.Command,
		Stdout:   stdoutStr,
		Stderr:   stderr.String(),
		ExitCode: exitCodeSuccess,
	}

	if runErr == nil {
		logger.DebugCF("shell_executor", "foreground shell command finished", map[string]any{
			"exit_code":  result.ExitCode,
			"timed_out":  result.TimedOut,
			"stdout_len": len(result.Stdout),
			"stderr_len": len(result.Stderr),
		})
		return result, nil
	}

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) && runErr != nil {
		result.TimedOut = true
		result.ExitCode = exitCodeTimeout
		logger.DebugCF("shell_executor", "foreground shell command timed out", map[string]any{
			"timeout_ms": req.Timeout.Milliseconds(),
			"stdout_len": len(result.Stdout),
			"stderr_len": len(result.Stderr),
		})
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		logger.DebugCF("shell_executor", "foreground shell command exited with failure", map[string]any{
			"exit_code":  result.ExitCode,
			"stdout_len": len(result.Stdout),
			"stderr_len": len(result.Stderr),
		})
		return result, nil
	}

	return Result{}, fmt.Errorf("shell executor: run command: %w", runErr)
}

func streamStdout(stdout io.ReadCloser, dst *bytes.Buffer, onLine func(line string)) error {
	defer stdout.Close()

	var pending bytes.Buffer
	buf := make([]byte, 32*1024)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if _, writeErr := dst.Write(chunk); writeErr != nil {
				return writeErr
			}
			if onLine != nil {
				pending.Write(chunk)
				for {
					data := pending.Bytes()
					idx := bytes.IndexByte(data, '\n')
					if idx < 0 {
						break
					}
					line := string(data[:idx])
					if strings.HasSuffix(line, "\r") {
						line = strings.TrimSuffix(line, "\r")
					}
					onLine(line)
					pending.Next(idx + 1)
				}
			}
		}

		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			if onLine != nil && pending.Len() > 0 {
				onLine(strings.TrimSuffix(pending.String(), "\r"))
			}
			return nil
		}
		return err
	}
}

// lookupShell resolves the concrete shell executable and argument prefix used for one request.
func (e *Executor) lookupShell() (string, []string) {
	if e != nil && e.ShellLookup != nil {
		return e.ShellLookup()
	}
	return defaultShellCommand()
}

// environ returns the base child-process environment used for one request.
func (e *Executor) environ() []string {
	if e != nil && e.Environ != nil {
		return e.Environ()
	}
	return os.Environ()
}

// defaultShellCommand selects the minimum cross-platform shell entrypoint used by the migrated Bash tool.
func defaultShellCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell", []string{"-NoProfile", "-Command"}
	}
	return "bash", []string{"-lc"}
}

// mergeEnvironment overlays request-scoped environment values on top of one base environment slice.
func mergeEnvironment(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return append([]string{}, base...)
	}

	envMap := make(map[string]string, len(base)+len(overrides))
	order := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, exists := envMap[key]; !exists {
			order = append(order, key)
		}
		envMap[key] = value
	}
	for key, value := range overrides {
		if _, exists := envMap[key]; !exists {
			order = append(order, key)
		}
		envMap[key] = value
	}

	merged := make([]string, 0, len(order))
	for _, key := range order {
		merged = append(merged, fmt.Sprintf("%s=%s", key, envMap[key]))
	}
	return merged
}
