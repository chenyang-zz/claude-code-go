package powershell

import (
	"context"
	"errors"
	"testing"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformshell "github.com/sheepzhao/claude-code-go/internal/platform/shell"
)

// mockExecutor simulates PowerShell command execution for tests.
type mockExecutor struct {
	result platformshell.Result
	err    error
}

func (m *mockExecutor) Execute(_ context.Context, req platformshell.Request) (platformshell.Result, error) {
	if m.err != nil {
		return platformshell.Result{}, m.err
	}
	res := m.result
	res.Stdout = "output from: " + req.Command
	return res, nil
}

// mockPermissionChecker simulates permission decisions for tests.
type mockPermissionChecker struct {
	decision corepermission.Decision
}

func (m *mockPermissionChecker) Check(command string) platformshell.PermissionEvaluation {
	return platformshell.PermissionEvaluation{
		Decision:          m.decision,
		NormalizedCommand: command,
	}
}

func TestToolName(t *testing.T) {
	tool := NewTool(&mockExecutor{}, &mockPermissionChecker{decision: corepermission.DecisionAllow}, "default")
	if tool.Name() != "PowerShell" {
		t.Errorf("expected 'PowerShell', got %q", tool.Name())
	}
}

func TestToolInputSchema(t *testing.T) {
	tool := NewTool(&mockExecutor{}, &mockPermissionChecker{decision: corepermission.DecisionAllow}, "default")
	schema := tool.InputSchema()
	if schema.Properties == nil {
		t.Fatal("expected non-nil Properties")
	}
	if _, ok := schema.Properties["command"]; !ok {
		t.Error("expected 'command' property in schema")
	}
	if _, ok := schema.Properties["timeout"]; !ok {
		t.Error("expected 'timeout' property in schema")
	}
	if _, ok := schema.Properties["description"]; !ok {
		t.Error("expected 'description' property in schema")
	}
	cmdField := schema.Properties["command"]
	if !cmdField.Required {
		t.Error("expected 'command' to be required")
	}
}

func TestToolIsReadOnly(t *testing.T) {
	tool := NewTool(&mockExecutor{}, &mockPermissionChecker{decision: corepermission.DecisionAllow}, "default")
	if tool.IsReadOnly() {
		t.Error("expected IsReadOnly to be false")
	}
}

func TestToolIsConcurrencySafe(t *testing.T) {
	tool := NewTool(&mockExecutor{}, &mockPermissionChecker{decision: corepermission.DecisionAllow}, "default")
	if tool.IsConcurrencySafe() {
		t.Error("expected IsConcurrencySafe to be false")
	}
}

func TestToolInvokeEmptyCommand(t *testing.T) {
	tool := NewTool(&mockExecutor{}, &mockPermissionChecker{decision: corepermission.DecisionAllow}, "default")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"command": ""},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "command is required" {
		t.Errorf("expected 'command is required', got %q", result.Error)
	}
}

func TestToolInvokeDenied(t *testing.T) {
	tool := NewTool(&mockExecutor{}, &mockPermissionChecker{decision: corepermission.DecisionDeny}, "default")
	_, err := tool.Invoke(context.Background(), coretool.Call{
		Name: "PowerShell",
		Input: map[string]any{
			"command": "Remove-Item / -Recurse",
		},
		Context: coretool.UseContext{WorkingDir: "/test"},
	})
	if err != nil {
		t.Fatalf("expected no error for deny (result error), got: %v", err)
	}
}

func TestToolInvokeAsk(t *testing.T) {
	tool := NewTool(&mockExecutor{}, &mockPermissionChecker{decision: corepermission.DecisionAsk}, "default")
	_, err := tool.Invoke(context.Background(), coretool.Call{
		Name: "PowerShell",
		Input: map[string]any{
			"command": "Get-Process",
		},
		Context: coretool.UseContext{WorkingDir: "/test"},
	})
	var permErr *corepermission.BashPermissionError
	if !errors.As(err, &permErr) {
		t.Fatalf("expected BashPermissionError, got: %v (type: %T)", err, err)
	}
	if permErr.ToolName != "PowerShell" {
		t.Errorf("expected ToolName 'PowerShell', got %q", permErr.ToolName)
	}
}

func TestToolInvokeAllow(t *testing.T) {
	tool := NewTool(&mockExecutor{}, &mockPermissionChecker{decision: corepermission.DecisionAllow}, "default")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"command": "Get-Process",
		},
		Context: coretool.UseContext{WorkingDir: "/test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("expected no error, got: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
}

func TestToolBypassPermissions(t *testing.T) {
	tool := NewTool(&mockExecutor{}, &mockPermissionChecker{decision: corepermission.DecisionAsk}, "bypassPermissions")
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"command": "Get-ChildItem",
		},
		Context: coretool.UseContext{WorkingDir: "/test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("expected no error with bypassPermissions, got: %s", result.Error)
	}
}

func TestToolNilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{})
	if err == nil {
		t.Fatal("expected error for nil receiver")
	}
}

func TestToolNilExecutor(t *testing.T) {
	tool := &Tool{permissions: &mockPermissionChecker{decision: corepermission.DecisionAllow}}
	_, err := tool.Invoke(context.Background(), coretool.Call{})
	if err == nil {
		t.Fatal("expected error for nil executor")
	}
}

func TestToolNilPermissions(t *testing.T) {
	tool := &Tool{executor: &mockExecutor{}}
	_, err := tool.Invoke(context.Background(), coretool.Call{})
	if err == nil {
		t.Fatal("expected error for nil permissions")
	}
}

func TestResolveTimeout(t *testing.T) {
	tests := []struct {
		name    string
		value   int
		wantErr bool
	}{
		{"zero uses default", 0, false},
		{"valid custom", 5000, false},
		{"negative", -1, true},
		{"too large", maxTimeoutMilliseconds + 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveTimeout(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.value == 0 && got != effectiveDefaultTimeout() {
				t.Errorf("expected default timeout %d, got %d", effectiveDefaultTimeout(), got)
			}
			if tt.value > 0 && got != tt.value {
				t.Errorf("expected %d, got %d", tt.value, got)
			}
		})
	}
}

func TestNormalizePSCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ls", "get-childitem"},
		{"Get-ChildItem", "get-childitem"},
		{"cat file.txt", "get-content file.txt"},
		{"cd /tmp", "set-location /tmp"},
		{"rm -r foo", "remove-item -r foo"},
		{"iex payload", "invoke-expression payload"},
		{"echo hello world", "write-output hello world"},
		{"", ""},
		{"  ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizePSCommand(tt.input)
			if got != tt.expected {
				t.Errorf("normalizePSCommand(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestResolvePSCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ls", "get-childitem"},
		{"Get-ChildItem", "get-childitem"},
		{"get-childitem", "get-childitem"},
		{"rm", "remove-item"},
		{"iex", "invoke-expression"},
		{"unknown-cmd", "unknown-cmd"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := resolvePSCommand(tt.input)
			if got != tt.expected {
				t.Errorf("resolvePSCommand(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsPSDangerousCmdlet(t *testing.T) {
	tests := []struct {
		name     string
		cmdlet   string
		dangerous bool
	}{
		{"Invoke-Expression", "Invoke-Expression", true},
		{"iex alias", "iex", true},
		{"Invoke-WebRequest", "Invoke-WebRequest", true},
		{"Add-Type", "Add-Type", true},
		{"Get-ChildItem", "Get-ChildItem", false},
		{"Get-Content", "Get-Content", false},
		{"Write-Output", "Write-Output", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPSDangerousCmdlet(tt.cmdlet)
			if got != tt.dangerous {
				t.Errorf("isPSDangerousCmdlet(%q) = %v, want %v", tt.cmdlet, got, tt.dangerous)
			}
		})
	}
}

func TestPermissionChecker(t *testing.T) {
	cfg := coreconfig.PermissionConfig{
		Allow: []string{"PowerShell(Get-ChildItem:*)"},
		Deny:  []string{"PowerShell(Remove-Item:*)"},
		Ask:   []string{"PowerShell(Set-Content:*)"},
	}

	checker := NewPermissionChecker(cfg)

	// Deny rule
	eval := checker.Check("Remove-Item / -Recurse")
	if eval.Decision != corepermission.DecisionDeny {
		t.Errorf("expected deny, got %v", eval.Decision)
	}

	// Allow rule
	eval = checker.Check("Get-ChildItem C:\\")
	if eval.Decision != corepermission.DecisionAllow {
		t.Errorf("expected allow, got %v", eval.Decision)
	}

	// Ask rule
	eval = checker.Check("Set-Content file.txt 'hello'")
	if eval.Decision != corepermission.DecisionAsk {
		t.Errorf("expected ask, got %v", eval.Decision)
	}

	// Default (no matching rule)
	eval = checker.Check("Get-Process")
	if eval.Decision != corepermission.DecisionAsk {
		t.Errorf("expected ask (default), got %v", eval.Decision)
	}
}

func TestPermissionCheckerAliasResolution(t *testing.T) {
	cfg := coreconfig.PermissionConfig{
		Deny: []string{"PowerShell(iex:*)"},
	}

	checker := NewPermissionChecker(cfg)

	// Alias should be resolved to canonical cmdlet
	eval := checker.Check("iex 'malicious code'")
	if eval.Decision != corepermission.DecisionDeny {
		t.Errorf("expected deny for iex via alias resolution, got %v", eval.Decision)
	}

	// Full cmdlet name should also match
	eval = checker.Check("Invoke-Expression 'malicious code'")
	if eval.Decision != corepermission.DecisionDeny {
		t.Errorf("expected deny for Invoke-Expression, got %v", eval.Decision)
	}
}

func TestPermissionCheckerExactMatch(t *testing.T) {
	cfg := coreconfig.PermissionConfig{
		Allow: []string{"PowerShell(Get-Process)"},
	}

	checker := NewPermissionChecker(cfg)

	// Exact match
	eval := checker.Check("Get-Process")
	if eval.Decision != corepermission.DecisionAllow {
		t.Errorf("expected allow for exact match, got %v", eval.Decision)
	}

	// Should not match with args
	eval = checker.Check("Get-Process -Id 1")
	if eval.Decision != corepermission.DecisionAsk {
		t.Errorf("expected ask (not exact match with args), got %v", eval.Decision)
	}
}
