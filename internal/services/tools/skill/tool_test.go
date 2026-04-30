package skill

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// stubCommand is a minimal command.Command implementation for tests.
type stubCommand struct {
	name        string
	description string
	output      string
	execErr     error
}

func (c stubCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        c.name,
		Description: c.description,
	}
}

func (c stubCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	return command.Result{Output: c.output}, c.execErr
}

// setupTestRegistry creates an InMemoryRegistry with a test command registered.
func setupTestRegistry() command.Registry {
	r := command.NewInMemoryRegistry()
	r.Register(stubCommand{
		name:        "commit",
		description: "Create a git commit",
		output:      "Commit created successfully.",
	})
	r.Register(stubCommand{
		name:        "help",
		description: "Show help",
		output:      "Available commands: help, commit",
	})
	return r
}

func TestName(t *testing.T) {
	tool := NewTool()
	if tool.Name() != Name {
		t.Errorf("expected Name %q, got %q", Name, tool.Name())
	}
	if tool.Name() != "Skill" {
		t.Errorf("expected Name to be \"Skill\", got %q", tool.Name())
	}
}

func TestDescription(t *testing.T) {
	tool := NewTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("expected non-empty Description")
	}
	if !strings.Contains(desc, "skill") && !strings.Contains(desc, "Skill") {
		t.Error("expected Description to mention skill")
	}
}

func TestInputSchema(t *testing.T) {
	tool := NewTool()
	schema := tool.InputSchema()

	if _, ok := schema.Properties["skill"]; !ok {
		t.Error("expected schema to have 'skill' property")
	}
	if schema.Properties["skill"].Type != coretool.ValueKindString {
		t.Errorf("expected skill type string, got %v", schema.Properties["skill"].Type)
	}
	if !schema.Properties["skill"].Required {
		t.Error("expected skill to be required")
	}

	if _, ok := schema.Properties["args"]; !ok {
		t.Error("expected schema to have 'args' property")
	}
	if schema.Properties["args"].Type != coretool.ValueKindString {
		t.Errorf("expected args type string, got %v", schema.Properties["args"].Type)
	}
	if schema.Properties["args"].Required {
		t.Error("expected args to be optional")
	}
}

func TestIsReadOnly(t *testing.T) {
	tool := NewTool()
	if tool.IsReadOnly() {
		t.Error("expected SkillTool to NOT be read-only")
	}
}

func TestIsConcurrencySafe(t *testing.T) {
	tool := NewTool()
	if tool.IsConcurrencySafe() {
		t.Error("expected SkillTool to NOT be concurrency-safe")
	}
}

func TestRequiresUserInteraction(t *testing.T) {
	tool := NewTool()
	if !tool.RequiresUserInteraction() {
		t.Error("expected SkillTool to require user interaction")
	}
}

func TestNilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{})
	if err == nil {
		t.Error("expected error from nil receiver")
	}
	if err.Error() != "skill tool: nil receiver" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInvokeNilRegistry(t *testing.T) {
	// Save and restore SharedRegistry
	orig := SharedRegistry
	SharedRegistry = nil
	defer func() { SharedRegistry = orig }()

	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"skill": "help",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error result when registry is nil")
	}
	if !strings.Contains(result.Error, "not initialized") {
		t.Errorf("expected 'not initialized' error, got: %s", result.Error)
	}
}

func TestInvokeSkillNotFound(t *testing.T) {
	orig := SharedRegistry
	SharedRegistry = setupTestRegistry()
	defer func() { SharedRegistry = orig }()

	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"skill": "nonexistent",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error result for unknown skill")
	}
	if !strings.Contains(result.Error, "Unknown skill") {
		t.Errorf("expected 'Unknown skill' error, got: %s", result.Error)
	}
}

func TestInvokeSkillFound(t *testing.T) {
	orig := SharedRegistry
	SharedRegistry = setupTestRegistry()
	defer func() { SharedRegistry = orig }()

	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"skill": "help",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "help") {
		t.Errorf("expected output to contain 'help', got: %s", result.Output)
	}

	// Check Meta data
	data, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatal("expected Meta.data to be of type Output")
	}
	if !data.Success {
		t.Error("expected success=true")
	}
	if data.CommandName != "help" {
		t.Errorf("expected commandName=help, got %q", data.CommandName)
	}
	if !strings.Contains(data.Output, "Available commands") {
		t.Errorf("expected output to contain command output, got: %s", data.Output)
	}
}

func TestInvokeSkillWithLeadingSlash(t *testing.T) {
	orig := SharedRegistry
	SharedRegistry = setupTestRegistry()
	defer func() { SharedRegistry = orig }()

	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"skill": "/help",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	if data.CommandName != "help" {
		t.Errorf("expected commandName=help after stripping slash, got %q", data.CommandName)
	}
}

func TestInvokeSkillWithArgs(t *testing.T) {
	orig := SharedRegistry
	SharedRegistry = setupTestRegistry()
	defer func() { SharedRegistry = orig }()

	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"skill": "commit",
			"args":  "-m 'test commit'",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	if !data.Success {
		t.Error("expected success=true")
	}
}

func TestInvokeEmptySkill(t *testing.T) {
	orig := SharedRegistry
	SharedRegistry = setupTestRegistry()
	defer func() { SharedRegistry = orig }()

	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"skill": "",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for empty skill")
	}
	if !strings.Contains(result.Error, "Invalid skill format") {
		t.Errorf("expected 'Invalid skill format' error, got: %s", result.Error)
	}
}

func TestInvokeSkillWhitespaceOnly(t *testing.T) {
	orig := SharedRegistry
	SharedRegistry = setupTestRegistry()
	defer func() { SharedRegistry = orig }()

	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"skill": "   ",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for whitespace-only skill")
	}
}

func TestInvokeSchemaValidationWrongType(t *testing.T) {
	orig := SharedRegistry
	SharedRegistry = setupTestRegistry()
	defer func() { SharedRegistry = orig }()

	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"skill": 123,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected schema validation error for non-string skill")
	}
}

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"empty", "", nil},
		{"single", "hello", []string{"hello"}},
		{"multiple", "hello world", []string{"hello", "world"}},
		{"with unclosed single quote", "it's a test", []string{"its a test"}},
		{"with double quotes", `arg1 "arg with spaces" arg3`, []string{"arg1", "arg with spaces", "arg3"}},
		{"with single quote wrapping", "arg1 'arg with spaces' arg3", []string{"arg1", "arg with spaces", "arg3"}},
		{"multiple spaces", "a  b   c", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitArgs(tt.raw)
			if len(got) != len(tt.want) {
				t.Errorf("splitArgs(%q) = %v (len=%d), want %v (len=%d)",
					tt.raw, got, len(got), tt.want, len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitArgs(%q)[%d] = %q, want %q", tt.raw, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFormatOutputText(t *testing.T) {
	tests := []struct {
		name   string
		output Output
		want   string
	}{
		{
			name:   "failure",
			output: Output{Success: false},
			want:   "Skill execution failed.",
		},
		{
			name:   "success without output",
			output: Output{Success: true, CommandName: "test"},
			want:   `Skill "test" executed successfully.`,
		},
		{
			name:   "success with output",
			output: Output{Success: true, CommandName: "test", Output: "Hello world"},
			want:   "Skill \"test\" executed:\n\nHello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatOutputText(tt.output)
			if got != tt.want {
				t.Errorf("formatOutputText() = %q, want %q", got, tt.want)
			}
		})
	}
}
