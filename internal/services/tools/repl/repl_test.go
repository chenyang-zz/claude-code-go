package repl

import (
	"os"
	"strings"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func TestIsReplModeEnabled_DefaultAntCLI(t *testing.T) {
	// Default: USER_TYPE=ant + CLAUDE_CODE_ENTRYPOINT=cli
	restore := setEnv("USER_TYPE", "ant")
	defer restore()
	restore2 := setEnv("CLAUDE_CODE_ENTRYPOINT", "cli")
	defer restore2()

	if !IsReplModeEnabled() {
		t.Error("IsReplModeEnabled() = false for default ant CLI, want true")
	}
}

func TestIsReplModeEnabled_CLAUDE_CODE_REPL_Disabled(t *testing.T) {
	restore := setEnv("CLAUDE_CODE_REPL", "0")
	defer restore()
	restore2 := setEnv("USER_TYPE", "ant")
	defer restore2()

	if IsReplModeEnabled() {
		t.Error("IsReplModeEnabled() = true with CLAUDE_CODE_REPL=0, want false")
	}
}

func TestIsReplModeEnabled_CLAUDE_REPL_MODE_Enabled(t *testing.T) {
	restore := setEnv("CLAUDE_REPL_MODE", "1")
	defer restore()
	restore2 := setEnv("USER_TYPE", "sdk")
	defer restore2()

	if !IsReplModeEnabled() {
		t.Error("IsReplModeEnabled() = false with CLAUDE_REPL_MODE=1, want true")
	}
}

func TestIsReplModeEnabled_NonAntCLI(t *testing.T) {
	restore := setEnv("USER_TYPE", "sdk")
	defer restore()
	restore2 := setEnv("CLAUDE_CODE_ENTRYPOINT", "cli")
	defer restore2()

	if IsReplModeEnabled() {
		t.Error("IsReplModeEnabled() = true for SDK USER_TYPE, want false")
	}
}

func TestIsReplModeEnabled_CLAUDE_CODE_REPL_FalseLiteral(t *testing.T) {
	restore := setEnv("CLAUDE_CODE_REPL", "false")
	defer restore()
	restore2 := setEnv("USER_TYPE", "ant")
	defer restore2()

	if IsReplModeEnabled() {
		t.Error("IsReplModeEnabled() = true with CLAUDE_CODE_REPL=false, want false")
	}
}

func TestREPL_ONLY_TOOLS_ContainsExpectedTools(t *testing.T) {
	expected := []string{
		"FileReadTool",
		"FileWriteTool",
		"FileEditTool",
		"GlobTool",
		"GrepTool",
		"BashTool",
		"NotebookEditTool",
		"AgentTool",
	}
	for _, name := range expected {
		if _, ok := REPL_ONLY_TOOLS[name]; !ok {
			t.Errorf("REPL_ONLY_TOOLS missing expected tool: %s", name)
		}
	}
	if len(REPL_ONLY_TOOLS) != len(expected) {
		t.Errorf("REPL_ONLY_TOOLS has %d entries, want %d", len(REPL_ONLY_TOOLS), len(expected))
	}
}

func TestToolName(t *testing.T) {
	tool := NewTool()
	if got := tool.Name(); got != "REPL" {
		t.Errorf("Name() = %q, want %q", got, "REPL")
	}
}

func TestToolDescription_NonEmpty(t *testing.T) {
	tool := NewTool()
	if got := tool.Description(); got == "" {
		t.Error("Description() = empty")
	}
}

func TestToolIsReadOnly(t *testing.T) {
	tool := NewTool()
	if !tool.IsReadOnly() {
		t.Error("IsReadOnly() = false, want true")
	}
}

func TestToolIsConcurrencySafe(t *testing.T) {
	tool := NewTool()
	if !tool.IsConcurrencySafe() {
		t.Error("IsConcurrencySafe() = false, want true")
	}
}

func TestToolInputSchema(t *testing.T) {
	tool := NewTool()
	schema := tool.InputSchema()
	if _, ok := schema.Properties["show_tools"]; !ok {
		t.Error("InputSchema missing 'show_tools' property")
	}
}

func TestToolInvoke_Default(t *testing.T) {
	restore := setEnv("USER_TYPE", "ant")
	defer restore()
	restore2 := setEnv("CLAUDE_CODE_ENTRYPOINT", "cli")
	defer restore2()

	tool := NewTool()
	result, err := tool.Invoke(nil, coretool.Call{Input: map[string]any{}})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}

	if result.Output == "" {
		t.Error("Invoke() returned empty Output")
	}
	if !strings.Contains(result.Output, "REPL mode is enabled") {
		t.Errorf("Output missing 'REPL mode is enabled', got: %s", result.Output)
	}
}

func TestToolInvoke_WithShowTools(t *testing.T) {
	tool := NewTool()
	result, err := tool.Invoke(nil, coretool.Call{Input: map[string]any{"show_tools": true}})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}

	for _, name := range []string{"BashTool", "FileReadTool", "AgentTool"} {
		if !strings.Contains(result.Output, name) {
			t.Errorf("Output missing tool name %q when ShowTools=true", name)
		}
	}
}

func TestToolInvoke_NotEnabled(t *testing.T) {
	restore := setEnv("USER_TYPE", "sdk")
	defer restore()

	tool := NewTool()
	result, err := tool.Invoke(nil, coretool.Call{Input: map[string]any{}})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}

	if !strings.Contains(result.Output, "REPL mode is disabled") {
		t.Errorf("Output missing 'REPL mode is disabled', got: %s", result.Output)
	}
}

func TestToolInvoke_NilReceiver(t *testing.T) {
	var nilTool *Tool
	_, err := nilTool.Invoke(nil, coretool.Call{})
	if err == nil {
		t.Error("Invoke() with nil receiver: expected error, got nil")
	}
}

// setEnv sets an env var and returns a restore function.
func setEnv(key, value string) func() {
	old, had := os.LookupEnv(key)
	os.Setenv(key, value)
	return func() {
		if had {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	}
}
