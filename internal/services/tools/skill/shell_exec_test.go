package skill

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// stubShellExec is a test stub implementing ShellExecFunc for unit tests.
type stubShellExec struct {
	stdout string
	stderr string
	err    error
}

func (s *stubShellExec) execute(ctx context.Context, command string, workingDir string) (string, string, error) {
	if s.err != nil {
		return s.stdout, s.stderr, s.err
	}
	return s.stdout, s.stderr, nil
}

func TestHasShellCommands(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"no commands", "plain text", false},
		{"block pattern", "```!\necho hello\n```", true},
		{"inline pattern", "result: !`date`", true},
		{"both patterns", "```!\nls\n```\nand !`pwd`", true},
		{"backtick only no bang", "`code`", false},
		{"bang only no backtick", "just!text", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasShellCommands(tt.content)
			if got != tt.want {
				t.Errorf("hasShellCommands(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestProcessShellCommands_NoExecutor(t *testing.T) {
	content := "```!\necho hello\n```"
	result, err := processShellCommands(context.Background(), content, nil, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != content {
		t.Errorf("expected unchanged content when executor is nil, got %q", result)
	}
}

func TestProcessShellCommands_NoPatterns(t *testing.T) {
	content := "plain text without any shell commands"
	result, err := processShellCommands(context.Background(), content, stubExec, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != content {
		t.Errorf("expected unchanged content, got %q", result)
	}
}

func TestProcessShellCommands_BlockCommand(t *testing.T) {
	content := "Before\n```!\necho hello\n```\nAfter"
	result, err := processShellCommands(context.Background(), content, echoExec, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "echo hello") {
		t.Errorf("expected output to contain 'echo hello', got %q", result)
	}
	if !strings.Contains(result, "Before") || !strings.Contains(result, "After") {
		t.Errorf("expected surrounding content preserved, got %q", result)
	}
}

func TestProcessShellCommands_InlineCommand(t *testing.T) {
	content := "The date is !`date` today"
	want := "The date is 2026-01-01 today"
	result, err := processShellCommands(context.Background(), content, fixedOutputExec("2026-01-01"), "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != want {
		t.Errorf("expected %q, got %q", want, result)
	}
}

func TestProcessShellCommands_MultipleCommands(t *testing.T) {
	content := "```!\necho a\n```\nand !`echo b` end"
	result, err := processShellCommands(context.Background(), content, echoExec, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "a") || !strings.Contains(result, "b") {
		t.Errorf("expected output to contain both 'a' and 'b', got %q", result)
	}
}

func TestProcessShellCommands_BlockCommandMultiline(t *testing.T) {
	content := "```!\necho line1\necho line2\n```"
	result, err := processShellCommands(context.Background(), content, echoExec, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "line1") {
		t.Errorf("expected output to contain 'line1', got %q", result)
	}
}

func TestProcessShellCommands_EmptyCommand(t *testing.T) {
	content := "```!\n\n```"
	result, err := processShellCommands(context.Background(), content, echoExec, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != content {
		t.Errorf("expected unchanged content for empty command, got %q", result)
	}
}

func TestProcessShellCommands_BlockWithStderr(t *testing.T) {
	content := "```!\nls /nonexistent\n```"
	exec := &stubShellExec{stdout: "", stderr: "No such file"}
	result, err := processShellCommands(context.Background(), content, exec.execute, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "[stderr]") {
		t.Errorf("expected stderr section in output, got %q", result)
	}
	if !strings.Contains(result, "No such file") {
		t.Errorf("expected stderr content in output, got %q", result)
	}
}

func TestProcessShellCommands_InlineWithStderr(t *testing.T) {
	content := "cmd: !`ls /x` done"
	exec := &stubShellExec{stdout: "", stderr: "error msg"}
	result, err := processShellCommands(context.Background(), content, exec.execute, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "[stderr: error msg]") {
		t.Errorf("expected inline stderr format, got %q", result)
	}
}

func TestProcessShellCommands_ExecutionError(t *testing.T) {
	content := "```!\nfail\n```"
	exec := &stubShellExec{err: errors.New("command failed")}
	result, err := processShellCommands(context.Background(), content, exec.execute, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "[Error]") {
		t.Errorf("expected error section in output, got %q", result)
	}
	if !strings.Contains(result, "command failed") {
		t.Errorf("expected error message in output, got %q", result)
	}
}

func TestProcessShellCommands_InlineExecutionError(t *testing.T) {
	content := "run: !`fail` done"
	exec := &stubShellExec{err: errors.New("boom")}
	result, err := processShellCommands(context.Background(), content, exec.execute, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "[Error: boom]") {
		t.Errorf("expected inline error format, got %q", result)
	}
}

func TestProcessShellCommands_ContentWithDollarSign(t *testing.T) {
	// Shell output with $ should not be interpolated by Go string replacement
	content := "```!\necho '$PATH $HOME'\n```"
	result, err := processShellCommands(context.Background(), content, echoExec, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "$PATH $HOME") {
		t.Errorf("expected $PATH $HOME preserved in output, got %q", result)
	}
}

func TestFormatShellOutput(t *testing.T) {
	tests := []struct {
		name    string
		stdout  string
		stderr  string
		inline  bool
		contain string
	}{
		{"stdout only", "hello", "", false, "hello"},
		{"stderr only block", "", "error", false, "[stderr]\nerror"},
		{"stderr only inline", "", "error", true, "[stderr: error]"},
		{"stdout and stderr block", "out", "err", false, "out\n[stderr]\nerr"},
		{"stdout and stderr inline", "out", "err", true, "out [stderr: err]"},
		{"empty both", "", "", false, ""},
		{"whitespace only stdout", "  ", "", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatShellOutput(tt.stdout, tt.stderr, tt.inline)
			if got != tt.contain {
				t.Errorf("formatShellOutput(%q, %q, %v) = %q, want %q",
					tt.stdout, tt.stderr, tt.inline, got, tt.contain)
			}
		})
	}
}

func TestSkillExecute_WithoutShellExecutor(t *testing.T) {
	// Save and restore the global ShellExecutor
	orig := ShellExecutor
	ShellExecutor = nil
	defer func() { ShellExecutor = orig }()

	s := &Skill{
		name:    "test-skill",
		content: "```!\necho hello\n```",
		baseDir: "/tmp",
	}

	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Without ShellExecutor, content should be returned unchanged
	if !strings.Contains(result.Output, "```!") {
		t.Errorf("expected raw content preserved without ShellExecutor, got %q", result.Output)
	}
}

func TestSkillExecute_WithShellExecutor(t *testing.T) {
	orig := ShellExecutor
	ShellExecutor = func(ctx context.Context, command string, workingDir string) (string, string, error) {
		return fmt.Sprintf("output of: %s", command), "", nil
	}
	defer func() { ShellExecutor = orig }()

	s := &Skill{
		name:    "test-skill",
		content: "Run: !`date` now",
		baseDir: "/tmp",
	}

	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result.Output, "!`date`") {
		t.Errorf("expected shell command to be replaced, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "output of: date") {
		t.Errorf("expected shell command output in result, got %q", result.Output)
	}
}

func TestSkillExecute_BundledSkillWithShellExecutor(t *testing.T) {
	orig := ShellExecutor
	ShellExecutor = func(ctx context.Context, command string, workingDir string) (string, string, error) {
		return fmt.Sprintf("ran: %s", command), "", nil
	}
	defer func() { ShellExecutor = orig }()

	s := &Skill{
		name: "bundled-skill",
		content: "fallback content",
		bundledPrompt: func(args string) (string, error) {
			return "```!\necho bundled\n```", nil
		},
	}

	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "ran: echo bundled") {
		t.Errorf("expected shell command output for bundled skill, got %q", result.Output)
	}
}

// Helpers

// echoExec echoes the command back as stdout for testing.
func echoExec(ctx context.Context, command string, workingDir string) (string, string, error) {
	return command, "", nil
}

// fixedOutputExec returns a fixed stdout value for testing.
func fixedOutputExec(output string) ShellExecFunc {
	return func(ctx context.Context, command string, workingDir string) (string, string, error) {
		return output, "", nil
	}
}

// stubExec is a simple pass-through executor for tests.
func stubExec(ctx context.Context, command string, workingDir string) (string, string, error) {
	return command, "", nil
}
