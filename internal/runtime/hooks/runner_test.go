package hooks

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
)

func TestRunCommand_Success(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	runner := &Runner{Environ: os.Environ}
	cmdHook := hook.CommandHook{
		Type:    hook.TypeCommand,
		Command: `echo '{"status":"ok"}'`,
	}
	input := map[string]string{"session_id": "test-123"}

	result, err := runner.RunCommand(context.Background(), cmdHook, input, "")
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if !result.IsSuccess() {
		t.Errorf("expected success, got exit code %d", result.ExitCode)
	}
	if result.Stdout == "" {
		t.Error("expected stdout output")
	}
}

func TestRunCommand_StdinPipe(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	runner := &Runner{Environ: os.Environ}
	cmdHook := hook.CommandHook{
		Type:    hook.TypeCommand,
		Command: `cat`,
	}
	input := map[string]string{"hook_event_name": "Stop", "session_id": "s1"}

	result, err := runner.RunCommand(context.Background(), cmdHook, input, "")
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if !result.IsSuccess() {
		t.Fatalf("expected success, got exit code %d", result.ExitCode)
	}

	var parsed map[string]string
	if err := json.Unmarshal([]byte(result.Stdout), &parsed); err != nil {
		t.Fatalf("parse stdout JSON: %v", err)
	}
	if parsed["hook_event_name"] != "Stop" {
		t.Errorf("expected hook_event_name=Stop, got %v", parsed)
	}
}

func TestRunCommand_BlockingExitCode(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	runner := &Runner{Environ: os.Environ}
	cmdHook := hook.CommandHook{
		Type:    hook.TypeCommand,
		Command: `echo "blocked" >&2 && exit 2`,
	}

	result, err := runner.RunCommand(context.Background(), cmdHook, nil, "")
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if !result.IsBlocking() {
		t.Errorf("expected blocking (exit 2), got exit code %d", result.ExitCode)
	}
	if result.Stderr == "" {
		t.Error("expected stderr output")
	}
}

func TestRunCommand_Timeout(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	runner := &Runner{Environ: os.Environ}
	cmdHook := hook.CommandHook{
		Type:    hook.TypeCommand,
		Command: `sleep 30`,
		Timeout: 1,
	}

	start := time.Now()
	result, err := runner.RunCommand(context.Background(), cmdHook, nil, "")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if !result.TimedOut {
		t.Error("expected timeout")
	}
	if elapsed > 5*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestRunCommand_Cancel(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	runner := &Runner{Environ: os.Environ}
	cmdHook := hook.CommandHook{
		Type:    hook.TypeCommand,
		Command: `sleep 30`,
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result, err := runner.RunCommand(ctx, cmdHook, nil, "")
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code on cancel")
	}
}

func TestHasBlockingResult(t *testing.T) {
	tests := []struct {
		results []hook.HookResult
		want    bool
	}{
		{nil, false},
		{[]hook.HookResult{{ExitCode: 0}}, false},
		{[]hook.HookResult{{ExitCode: 2, Stderr: "blocked"}}, true},
		{[]hook.HookResult{{ExitCode: 0}, {ExitCode: 2, Stderr: "blocked"}}, true},
	}
	for _, tt := range tests {
		if got := HasBlockingResult(tt.results); got != tt.want {
			t.Errorf("HasBlockingResult(%v) = %v, want %v", tt.results, got, tt.want)
		}
	}
}

func TestBlockingErrors(t *testing.T) {
	results := []hook.HookResult{
		{ExitCode: 0},
		{ExitCode: 2, Stderr: "blocked error"},
		{ExitCode: 1, Stderr: "other error"},
	}
	errs := BlockingErrors(results)
	if len(errs) != 1 || errs[0] != "blocked error" {
		t.Errorf("BlockingErrors = %v, want [blocked error]", errs)
	}
}
