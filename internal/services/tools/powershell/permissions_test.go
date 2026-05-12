package powershell

import (
	"context"
	"testing"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
)

// newTestPolicy creates a minimal FilesystemPolicy with deny/ask rules for testing.
func newTestPolicy(t *testing.T) *corepermission.FilesystemPolicy {
	t.Helper()

	p, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{
		Read: []corepermission.Rule{
			{
				Source:   corepermission.RuleSourceSession,
				Decision: corepermission.DecisionDeny,
				Pattern:  "**/secrets/**",
			},
			{
				Source:   corepermission.RuleSourceSession,
				Decision: corepermission.DecisionAsk,
				Pattern:  "**/restricted/**",
			},
		},
		Write: []corepermission.Rule{
			{
				Source:   corepermission.RuleSourceSession,
				Decision: corepermission.DecisionDeny,
				Pattern:  "**/protected/**",
			},
			{
				Source:   corepermission.RuleSourceSession,
				Decision: corepermission.DecisionAsk,
				Pattern:  "**/sensitive/**",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// Direct path-level tests — verify extractPathsFromCommand + fsPolicy work
// together without requiring pwsh (uses mock ParsedCommandElement).
// ---------------------------------------------------------------------------

// TestFilesystemPolicyDenyWriteCmdlet verifies that a write cmdlet extracting
// a filesystem-deny path results in DecisionDeny.
func TestFilesystemPolicyDenyWriteCmdlet(t *testing.T) {
	policy := newTestPolicy(t)

	cmd := ParsedCommandElement{
		Name:         "Set-Content",
		NameType:     "cmdlet",
		ElementType:  "CommandAst",
		Args:         []string{"-Path", "/home/user/protected/config.txt", "-Value", "'data'"},
		ElementTypes: []string{"Parameter", "StringConstant", "Parameter", "StringConstant"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) == 0 {
		t.Fatal("expected paths to be extracted")
	}
	if result.OperationType != opWrite {
		t.Fatal("expected write operation type")
	}

	eval := policy.EvaluateFilesystem(context.Background(), corepermission.FilesystemRequest{
		ToolName:   Name,
		Path:       result.Paths[0],
		WorkingDir: "/home/user",
		Access:     corepermission.AccessWrite,
	})
	if eval.Decision != corepermission.DecisionDeny {
		t.Errorf("expected DecisionDeny for write to protected path, got %v", eval.Decision)
	}
}

// TestFilesystemPolicyDenyReadCmdlet verifies that a read cmdlet extracting
// a filesystem-deny read path results in DecisionDeny.
func TestFilesystemPolicyDenyReadCmdlet(t *testing.T) {
	policy := newTestPolicy(t)

	cmd := ParsedCommandElement{
		Name:         "Get-Content",
		NameType:     "cmdlet",
		ElementType:  "CommandAst",
		Args:         []string{"-Path", "/home/user/secrets/passwords.txt"},
		ElementTypes: []string{"Parameter", "StringConstant"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) == 0 {
		t.Fatal("expected paths to be extracted")
	}
	if result.OperationType != opRead {
		t.Fatal("expected read operation type")
	}

	eval := policy.EvaluateFilesystem(context.Background(), corepermission.FilesystemRequest{
		ToolName:   Name,
		Path:       result.Paths[0],
		WorkingDir: "/home/user",
		Access:     corepermission.AccessRead,
	})
	if eval.Decision != corepermission.DecisionDeny {
		t.Errorf("expected DecisionDeny for read from secrets path, got %v", eval.Decision)
	}
}

// TestFilesystemPolicyAskWriteCmdlet verifies that a write cmdlet extracting
// a filesystem-ask write path results in DecisionAsk.
func TestFilesystemPolicyAskWriteCmdlet(t *testing.T) {
	policy := newTestPolicy(t)

	cmd := ParsedCommandElement{
		Name:         "Set-Content",
		NameType:     "cmdlet",
		ElementType:  "CommandAst",
		Args:         []string{"-Path", "/home/user/sensitive/data.txt", "-Value", "'data'"},
		ElementTypes: []string{"Parameter", "StringConstant", "Parameter", "StringConstant"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) == 0 {
		t.Fatal("expected paths to be extracted")
	}

	eval := policy.EvaluateFilesystem(context.Background(), corepermission.FilesystemRequest{
		ToolName:   Name,
		Path:       result.Paths[0],
		WorkingDir: "/home/user",
		Access:     corepermission.AccessWrite,
	})
	if eval.Decision != corepermission.DecisionAsk {
		t.Errorf("expected DecisionAsk for write to sensitive path, got %v", eval.Decision)
	}
}

// TestFilesystemPolicyAllowPath verifies that a path outside deny/ask rules
// does not return DecisionDeny (DecisionAsk for writes without allow rule is expected).
func TestFilesystemPolicyAllowPath(t *testing.T) {
	policy := newTestPolicy(t)

	cmd := ParsedCommandElement{
		Name:         "Out-File",
		NameType:     "cmdlet",
		ElementType:  "CommandAst",
		Args:         []string{"-FilePath", "/home/user/output.txt", "-Encoding", "UTF8"},
		ElementTypes: []string{"Parameter", "StringConstant", "Parameter", "StringConstant"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) == 0 {
		t.Fatal("expected paths to be extracted")
	}

	eval := policy.EvaluateFilesystem(context.Background(), corepermission.FilesystemRequest{
		ToolName:   Name,
		Path:       result.Paths[0],
		WorkingDir: "/home/user",
		Access:     corepermission.AccessWrite,
	})
	if eval.Decision == corepermission.DecisionDeny {
		t.Errorf("unexpected DecisionDeny for allowed path (within working dir), got %v", eval.Decision)
	}
}

// ---------------------------------------------------------------------------
// CheckEnhanced-level tests — require pwsh for AST parsing
// ---------------------------------------------------------------------------

// TestCheckEnhancedFilesystemPolicyDenyWriteIntegration tests the full
// CheckEnhanced flow: a write cmdlet targeting a filesystem-deny path
// returns DecisionDeny. Requires pwsh for AST parsing.
func TestCheckEnhancedFilesystemPolicyDenyWriteIntegration(t *testing.T) {
	skipIfNoPwsh(t)

	policy := newTestPolicy(t)
	checker := NewPermissionChecker(coreconfig.PermissionConfig{
		DefaultMode: "default",
	}, policy)

	decision := checker.CheckEnhanced(
		`Set-Content -Path /home/user/protected/config.txt -Value 'data'`,
		ScanResult{Level: RiskLevelSafe},
		"default",
		context.Background(),
		"/home/user",
	)

	if decision.Evaluation.Decision != corepermission.DecisionDeny {
		t.Errorf("expected DecisionDeny for write to protected path, got %v (reason: %s)",
			decision.Evaluation.Decision, decision.Reason)
	}
}

// TestCheckEnhancedFilesystemPolicyDenyReadIntegration tests the full
// CheckEnhanced flow: a read cmdlet targeting a filesystem-deny read path
// returns DecisionDeny. Requires pwsh for AST parsing.
func TestCheckEnhancedFilesystemPolicyDenyReadIntegration(t *testing.T) {
	skipIfNoPwsh(t)

	policy := newTestPolicy(t)
	checker := NewPermissionChecker(coreconfig.PermissionConfig{
		DefaultMode: "default",
	}, policy)

	decision := checker.CheckEnhanced(
		`Get-Content -Path /home/user/secrets/passwords.txt`,
		ScanResult{Level: RiskLevelSafe},
		"default",
		context.Background(),
		"/home/user",
	)

	if decision.Evaluation.Decision != corepermission.DecisionDeny {
		t.Errorf("expected DecisionDeny for read from secrets path, got %v (reason: %s)",
			decision.Evaluation.Decision, decision.Reason)
	}
}

// TestCheckEnhancedFilesystemPolicyAskWriteIntegration tests the full
// CheckEnhanced flow: a write cmdlet targeting a filesystem-ask path
// returns DecisionAsk. Requires pwsh for AST parsing.
func TestCheckEnhancedFilesystemPolicyAskWriteIntegration(t *testing.T) {
	skipIfNoPwsh(t)

	policy := newTestPolicy(t)
	checker := NewPermissionChecker(coreconfig.PermissionConfig{
		DefaultMode: "default",
	}, policy)

	decision := checker.CheckEnhanced(
		`Set-Content -Path /home/user/sensitive/data.txt -Value 'data'`,
		ScanResult{Level: RiskLevelSafe},
		"default",
		context.Background(),
		"/home/user",
	)

	if decision.Evaluation.Decision != corepermission.DecisionAsk {
		t.Errorf("expected DecisionAsk for write to sensitive path, got %v (reason: %s)",
			decision.Evaluation.Decision, decision.Reason)
	}
}

// ---------------------------------------------------------------------------
// Edge cases — no pwsh required
// ---------------------------------------------------------------------------

// TestCheckEnhancedFilesystemPolicyWithNil verifies that nil fsPolicy does
// not interfere with normal permission flow.
func TestCheckEnhancedFilesystemPolicyWithNil(t *testing.T) {
	checker := NewPermissionChecker(coreconfig.PermissionConfig{
		Allow: []string{"PowerShell(Get-ChildItem:*)"},
	}, nil)

	decision := checker.CheckEnhanced(
		`Get-ChildItem C:\`,
		ScanResult{Level: RiskLevelSafe},
		"default",
		context.Background(),
		"/",
	)

	if decision.Evaluation.Decision != corepermission.DecisionAllow {
		t.Errorf("expected DecisionAllow via rule (policy=nil), got %v (reason: %s)",
			decision.Evaluation.Decision, decision.Reason)
	}
}

// TestCheckEnhancedFilesystemPolicyUnknownCmdlet verifies that a cmdlet
// not in CMDLET_PATH_CONFIG does not trigger filesystem checks.
func TestCheckEnhancedFilesystemPolicyUnknownCmdlet(t *testing.T) {
	policy := newTestPolicy(t)
	checker := NewPermissionChecker(coreconfig.PermissionConfig{
		DefaultMode: "default",
	}, policy)

	decision := checker.CheckEnhanced(
		`Invoke-Expression "echo hello"`,
		ScanResult{Level: RiskLevelDangerous, Message: "invoke expression"},
		"default",
		context.Background(),
		"/home/user",
	)

	// The security scanner flags this as dangerous → DecisionAsk (not fs-related)
	if decision.Evaluation.Decision != corepermission.DecisionAsk {
		t.Errorf("expected DecisionAsk for dangerous cmdlet, got %v (reason: %s)",
			decision.Evaluation.Decision, decision.Reason)
	}
}
