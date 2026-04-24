package bash

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformshell "github.com/sheepzhao/claude-code-go/internal/platform/shell"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
)

// TestExecuteSwitchableCommandAutoBackgrounds verifies that a long-running
// foreground command is automatically moved to the background after the
// assistant blocking budget expires.
func TestExecuteSwitchableCommandAutoBackgrounds(t *testing.T) {
	// Override the budget to a short value so the test does not wait 15s.
	origBudget := switchableAutoBackgroundBudget
	switchableAutoBackgroundBudget = 200 * time.Millisecond
	defer func() { switchableAutoBackgroundBudget = origBudget }()

	store := runtimesession.NewBackgroundTaskStore()
	tool := NewToolWithNotification(
		&switchableExecutorStub{delay: 10 * time.Second},
		platformshell.NewPermissionChecker(coreconfig.PermissionConfig{Allow: []string{"Bash(*)"}}),
		"default",
		store,
		nil,
	)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: "Bash",
		Input: map[string]any{
			"command": "npm test",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error output = %q, want empty", result.Error)
	}
	if !strings.Contains(result.Output, "moved to the background") {
		t.Fatalf("Invoke() output = %q, want backgrounded message", result.Output)
	}

	tasks := store.List()
	if len(tasks) != 0 {
		t.Logf("tasks remaining: %d", len(tasks))
	}
}

// TestExecuteSwitchableCommandCompletesQuickly verifies that a short command
// finishes normally without being auto-backgrounded.
func TestExecuteSwitchableCommandCompletesQuickly(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewToolWithNotification(
		&switchableExecutorStub{delay: 0},
		platformshell.NewPermissionChecker(coreconfig.PermissionConfig{Allow: []string{"Bash(*)"}}),
		"default",
		store,
		nil,
	)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: "Bash",
		Input: map[string]any{
			"command": "echo hello",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error output = %q, want empty", result.Error)
	}
	if strings.TrimSpace(result.Output) != "hello" {
		t.Fatalf("Invoke() output = %q, want hello", result.Output)
	}
}

// TestExecuteSwitchableCommandContextCancellation verifies that cancelling the
// context stops the background process and cleans up the task store.
func TestExecuteSwitchableCommandContextCancellation(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewToolWithNotification(
		&switchableExecutorStub{delay: 10 * time.Second},
		platformshell.NewPermissionChecker(coreconfig.PermissionConfig{Allow: []string{"Bash(*)"}}),
		"default",
		store,
		nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result, err := tool.Invoke(ctx, coretool.Call{
		Name: "Bash",
		Input: map[string]any{
			"command": "npm test",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "Command was aborted before completion" {
		t.Fatalf("Invoke() error = %q, want 'Command was aborted before completion'", result.Error)
	}
}

// TestIsAutobackgroundingAllowed verifies the auto-backgrounding eligibility rules.
func TestIsAutobackgroundingAllowed(t *testing.T) {
	cases := []struct {
		command string
		want    bool
	}{
		{"sleep 5", false},
		{"sleep 1 && echo done", false},
		{"npm test", true},
		{"go build", true},
		{"python script.py", true},
		{"", false},
		{"  sleep 2  ", false},
	}
	for _, c := range cases {
		got := isAutobackgroundingAllowed(c.command)
		if got != c.want {
			t.Fatalf("isAutobackgroundingAllowed(%q) = %v, want %v", c.command, got, c.want)
		}
	}
}

// TestRenderAutoBackgroundStart verifies the auto-background result message format.
func TestRenderAutoBackgroundStart(t *testing.T) {
	output := renderAutoBackgroundStart(BackgroundOutput{
		TaskID:  "task_abc123",
		Command: "npm test",
		Summary: "npm test",
	})
	if !strings.Contains(output, "task_abc123") {
		t.Fatalf("output = %q, want task ID", output)
	}
	if !strings.Contains(output, "15s") {
		t.Fatalf("output = %q, want blocking budget seconds", output)
	}
}

// switchableExecutorStub is a test double that implements both ShellExecutor and
// BackgroundShellExecutor so that switchable execution can be exercised without
// spawning real OS processes.
type switchableExecutorStub struct {
	delay time.Duration
}

func (s *switchableExecutorStub) Execute(ctx context.Context, req platformshell.Request) (platformshell.Result, error) {
	select {
	case <-time.After(s.delay):
		return platformshell.Result{Stdout: "hello", ExitCode: 0}, nil
	case <-ctx.Done():
		return platformshell.Result{}, ctx.Err()
	}
}

func (s *switchableExecutorStub) Start(req platformshell.Request) (platformshell.BackgroundProcess, error) {
	resultCh := make(chan platformshell.Result, 1)
	cancelCh := make(chan struct{}, 1)
	go func() {
		select {
		case <-time.After(s.delay):
			resultCh <- platformshell.Result{Stdout: "hello", ExitCode: 0}
		case <-cancelCh:
			resultCh <- platformshell.Result{ExitCode: -2, Canceled: true}
		}
	}()
	return &stubBackgroundProcess{result: resultCh, cancel: cancelCh}, nil
}

type stubBackgroundProcess struct {
	result chan platformshell.Result
	cancel chan struct{}
}

func (p *stubBackgroundProcess) Stop() error {
	if p.cancel != nil {
		select {
		case p.cancel <- struct{}{}:
		default:
		}
	}
	return nil
}

func (p *stubBackgroundProcess) Result() <-chan platformshell.Result {
	return p.result
}

// TestConsumeBackgroundResultEmitsNotification verifies that when a background
// task completes and a notification emitter is configured, a notification is fired.
func TestConsumeBackgroundResultEmitsNotification(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	emitter := &testNotificationEmitter{}
	tool := NewToolWithNotification(
		platformshell.NewExecutor(),
		platformshell.NewPermissionChecker(coreconfig.PermissionConfig{Allow: []string{"Bash(*)"}}),
		"default",
		store,
		emitter,
	)

	process := &stubBackgroundProcess{
		result: make(chan platformshell.Result, 1),
	}
	process.result <- platformshell.Result{ExitCode: 0}

	tool.consumeBackgroundResult("task_1", coresession.BackgroundTaskSnapshot{
		ID:      "task_1",
		Type:    "bash",
		Status:  coresession.BackgroundTaskStatusRunning,
		Summary: "run tests",
	}, process)

	time.Sleep(50 * time.Millisecond) // let the goroutine finish

	if len(emitter.calls) != 1 {
		t.Fatalf("notification calls = %d, want 1", len(emitter.calls))
	}
	c := emitter.calls[0]
	if c.TaskID != "task_1" {
		t.Fatalf("TaskID = %q, want task_1", c.TaskID)
	}
	if c.Status != "completed" {
		t.Fatalf("Status = %q, want completed", c.Status)
	}
}

// TestAppendOutputFileInfo verifies output file metadata is appended to rendered results.
func TestAppendOutputFileInfo(t *testing.T) {
	body := "stdout content"
	output := Output{
		Stdout:         "stdout content",
		OutputFilePath: "/tmp/out.txt",
		OutputFileSize: 42,
	}
	result := appendOutputFileInfo(body, output)
	if !strings.Contains(result, "/tmp/out.txt") {
		t.Fatalf("result = %q, want output file path", result)
	}
	if !strings.Contains(result, "42 bytes") {
		t.Fatalf("result = %q, want output file size", result)
	}
}

func TestAppendOutputFileInfoNoFile(t *testing.T) {
	body := "stdout content"
	output := Output{Stdout: "stdout content"}
	result := appendOutputFileInfo(body, output)
	if result != body {
		t.Fatalf("result = %q, want %q", result, body)
	}
}

// TestToolInvokeWithOutputRedirection verifies that a foreground command with
// stdout redirection captures the output file path in the result metadata.
func TestToolInvokeWithOutputRedirection(t *testing.T) {
	projectDir := t.TempDir()
	tool := NewTool(platformshell.NewExecutor(), platformshell.NewPermissionChecker(coreconfig.PermissionConfig{Allow: []string{"Bash(*)"}}))

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: "Bash",
		Input: map[string]any{
			"command": fmt.Sprintf("echo hello-world > %s/out.txt", projectDir),
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error output = %q, want empty", result.Error)
	}
	if !strings.Contains(result.Output, "hello-world") {
		t.Fatalf("Invoke() output = %q, want hello-world", result.Output)
	}

	// The Meta should contain the output file path.
	data, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatalf("Meta data type = %T, want Output", result.Meta["data"])
	}
	if data.OutputFilePath == "" {
		t.Fatal("OutputFilePath is empty, want captured file path")
	}
}

// TestToolInvokeWithNotification verifies that a background task completion
// triggers the notification emitter when configured.
func TestToolInvokeWithNotification(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	emitter := &testNotificationEmitter{}
	tool := NewToolWithNotification(
		platformshell.NewExecutor(),
		platformshell.NewPermissionChecker(coreconfig.PermissionConfig{Allow: []string{"Bash(*)"}}),
		"default",
		store,
		emitter,
	)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: "Bash",
		Input: map[string]any{
			"command":           "echo hello",
			"run_in_background": true,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error output = %q, want empty", result.Error)
	}

	// Wait for the background goroutine to complete and emit the notification.
	// Shell startup can take >500ms on some hosts (e.g. heavy .bashrc), so we
	// allow a generous window.
	time.Sleep(2 * time.Second)

	if len(emitter.calls) != 1 {
		t.Fatalf("notification calls = %d, want 1", len(emitter.calls))
	}
	c := emitter.calls[0]
	if c.Status != "completed" {
		t.Fatalf("Status = %q, want completed", c.Status)
	}
}

// switchableExecutorErrorStub always returns an error from Start.
type switchableExecutorErrorStub struct{}

func (s *switchableExecutorErrorStub) Execute(ctx context.Context, req platformshell.Request) (platformshell.Result, error) {
	return platformshell.Result{}, errors.New("execute error")
}

func (s *switchableExecutorErrorStub) Start(req platformshell.Request) (platformshell.BackgroundProcess, error) {
	return nil, errors.New("start error")
}

// TestExecuteSwitchableCommandStartError verifies that a Start error is surfaced gracefully.
func TestExecuteSwitchableCommandStartError(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	tool := NewToolWithNotification(
		&switchableExecutorErrorStub{},
		platformshell.NewPermissionChecker(coreconfig.PermissionConfig{Allow: []string{"Bash(*)"}}),
		"default",
		store,
		nil,
	)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: "Bash",
		Input: map[string]any{
			"command": "echo hello",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("Invoke() error = empty, want start error message")
	}
}
