package shell

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestExecutorExecuteSuccess verifies the foreground executor captures stdout from one successful shell command.
func TestExecutorExecuteSuccess(t *testing.T) {
	executor := NewExecutor()
	result, err := executor.Execute(context.Background(), Request{
		Command: successCommandForTest(),
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("Execute() exit code = %d, want 0", result.ExitCode)
	}
	if result.TimedOut {
		t.Fatal("Execute() TimedOut = true, want false")
	}
	if got := strings.TrimSpace(result.Stdout); got != "hello-from-bash" {
		t.Fatalf("Execute() stdout = %q, want hello-from-bash", got)
	}
}

// TestExecutorExecuteFailure verifies non-zero shell exits are returned as normalized results instead of transport errors.
func TestExecutorExecuteFailure(t *testing.T) {
	executor := NewExecutor()
	result, err := executor.Execute(context.Background(), Request{
		Command: failureCommandForTest(),
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ExitCode == 0 {
		t.Fatalf("Execute() exit code = %d, want non-zero", result.ExitCode)
	}
}

// TestExecutorExecuteTimeout verifies the executor marks commands that exceed the configured timeout.
func TestExecutorExecuteTimeout(t *testing.T) {
	executor := NewExecutor()
	result, err := executor.Execute(context.Background(), Request{
		Command: timeoutCommandForTest(),
		Timeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.TimedOut {
		t.Fatal("Execute() TimedOut = false, want true")
	}
	if result.ExitCode != exitCodeTimeout {
		t.Fatalf("Execute() exit code = %d, want %d", result.ExitCode, exitCodeTimeout)
	}
}

// TestExecutorExecuteStreamingPreservesStdoutBytes verifies streaming mode does not
// synthesize trailing newlines when stdout does not contain them.
func TestExecutorExecuteStreamingPreservesStdoutBytes(t *testing.T) {
	executor := NewExecutor()

	var lines []string
	result, err := executor.Execute(context.Background(), Request{
		Command:      successCommandWithoutTrailingNewlineForTest(),
		Timeout:      2 * time.Second,
		OnStdoutLine: func(line string) { lines = append(lines, line) },
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("Execute() exit code = %d, want 0", result.ExitCode)
	}
	if result.Stdout != "hello-without-newline" {
		t.Fatalf("Execute() stdout = %q, want exact stdout bytes", result.Stdout)
	}
	if len(lines) != 1 || lines[0] != "hello-without-newline" {
		t.Fatalf("Execute() streamed lines = %#v, want one callback line", lines)
	}
}

// TestExecutorExecuteStreamingPreservesLongLines verifies streaming mode handles
// single stdout lines that exceed bufio.Scanner's default token limit.
func TestExecutorExecuteStreamingPreservesLongLines(t *testing.T) {
	executor := NewExecutor()
	want := strings.Repeat("x", 70*1024)

	result, err := executor.Execute(context.Background(), Request{
		Command:      printLiteralForTest(want, false),
		Timeout:      2 * time.Second,
		OnStdoutLine: func(string) {},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("Execute() exit code = %d, want 0", result.ExitCode)
	}
	if result.Stdout != want {
		t.Fatalf("Execute() stdout len = %d, want %d", len(result.Stdout), len(want))
	}
}

// TestExecutorStartSuccess verifies the background executor returns one successful result for a completed command.
func TestExecutorStartSuccess(t *testing.T) {
	executor := NewExecutor()
	process, err := executor.Start(Request{
		Command: successCommandForTest(),
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	result, ok := <-process.Result()
	if !ok {
		t.Fatal("Start() result channel closed without a result")
	}
	if result.ExitCode != 0 {
		t.Fatalf("Start() exit code = %d, want 0", result.ExitCode)
	}
	if result.TimedOut {
		t.Fatal("Start() TimedOut = true, want false")
	}
	if result.Canceled {
		t.Fatal("Start() Canceled = true, want false")
	}
}

// TestExecutorStartStop verifies explicit stop requests surface as canceled background results.
func TestExecutorStartStop(t *testing.T) {
	executor := NewExecutor()
	process, err := executor.Start(Request{
		Command: timeoutCommandForTest(),
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := process.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	result, ok := <-process.Result()
	if !ok {
		t.Fatal("Stop() result channel closed without a result")
	}
	if !result.Canceled {
		t.Fatal("Stop() Canceled = false, want true")
	}
	if result.ExitCode != exitCodeCanceled {
		t.Fatalf("Stop() exit code = %d, want %d", result.ExitCode, exitCodeCanceled)
	}
}

// successCommandForTest returns a cross-platform shell command that prints one deterministic line.
func successCommandForTest() string {
	if runtime.GOOS == "windows" {
		return "Write-Output 'hello-from-bash'"
	}
	return "printf 'hello-from-bash\\n'"
}

func successCommandWithoutTrailingNewlineForTest() string {
	if runtime.GOOS == "windows" {
		return "Write-Host -NoNewline 'hello-without-newline'"
	}
	return "printf 'hello-without-newline'"
}

func printLiteralForTest(text string, trailingNewline bool) string {
	if runtime.GOOS == "windows" {
		if trailingNewline {
			return "Write-Output '" + text + "'"
		}
		return "Write-Host -NoNewline '" + text + "'"
	}
	if trailingNewline {
		return "printf '%s\\n' '" + text + "'"
	}
	return "printf '%s' '" + text + "'"
}

// failureCommandForTest returns a cross-platform shell command that exits with a non-zero status code.
func failureCommandForTest() string {
	if runtime.GOOS == "windows" {
		return "exit 3"
	}
	return "exit 3"
}

// timeoutCommandForTest returns a cross-platform shell command that exceeds the short unit-test timeout.
func timeoutCommandForTest() string {
	if runtime.GOOS == "windows" {
		return "Start-Sleep -Seconds 2"
	}
	return "sleep 2"
}
