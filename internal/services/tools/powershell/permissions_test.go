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


// ---------------------------------------------------------------------------
// Mixed-IO cmdlet tests — verify per-path access for cmdlets with both
// read and write path parameters (Copy-Item, Invoke-WebRequest, etc.).
// ---------------------------------------------------------------------------

// TestFilesystemPolicyCopyItemMixedIO verifies that Copy-Item's source path
// (-Path) is treated as read and destination (-Destination) as write.
func TestFilesystemPolicyCopyItemMixedIO(t *testing.T) {
	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{
		Read: []corepermission.Rule{
			{
				Source:   corepermission.RuleSourceSession,
				Decision: corepermission.DecisionDeny,
				Pattern:  "**/readonly/**",
			},
		},
		Write: []corepermission.Rule{
			{
				Source:   corepermission.RuleSourceSession,
				Decision: corepermission.DecisionDeny,
				Pattern:  "**/nowrite/**",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy: %v", err)
	}

	cmd := ParsedCommandElement{
		Name:         "Copy-Item",
		NameType:     "cmdlet",
		ElementType:  "CommandAst",
		Args:         []string{"-Path", "/home/user/readonly/source.txt", "-Destination", "/home/user/nowrite/backup.txt"},
		ElementTypes: []string{"Parameter", "StringConstant", "Parameter", "StringConstant"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(result.Paths), result.Paths)
	}
	if len(result.PathAccess) != 2 {
		t.Fatalf("expected 2 path access entries, got %d", len(result.PathAccess))
	}

	// -Path should be read (source), -Destination should be write (target)
	if result.PathAccess[0] != opRead {
		t.Errorf("expected -Path (source) to be opRead, got %v", result.PathAccess[0])
	}
	if result.PathAccess[1] != opWrite {
		t.Errorf("expected -Destination to be opWrite, got %v", result.PathAccess[1])
	}

	// Verify read-deny path does get denied for read access
	eval1 := policy.EvaluateFilesystem(context.Background(), corepermission.FilesystemRequest{
		ToolName:   Name,
		Path:       result.Paths[0],
		WorkingDir: "/home/user",
		Access:     corepermission.AccessRead,
	})
	if eval1.Decision != corepermission.DecisionDeny {
		t.Errorf("expected source (%s) to be denied for read (readonly rule), got %v", result.Paths[0], eval1.Decision)
	}

	// Verify write-deny path does get denied for write access
	eval2 := policy.EvaluateFilesystem(context.Background(), corepermission.FilesystemRequest{
		ToolName:   Name,
		Path:       result.Paths[1],
		WorkingDir: "/home/user",
		Access:     corepermission.AccessWrite,
	})
	if eval2.Decision != corepermission.DecisionDeny {
		t.Errorf("expected dest (%s) to be denied for write (nowrite rule), got %v", result.Paths[1], eval2.Decision)
	}

	// But if we swapped the checks (source as write, dest as read), the
	// results would be wrong — this is what the per-path fix prevents.
	eval1Wrong := policy.EvaluateFilesystem(context.Background(), corepermission.FilesystemRequest{
		ToolName:   Name,
		Path:       result.Paths[0],
		WorkingDir: "/home/user",
		Access:     corepermission.AccessWrite,
	})
	t.Logf("source checked as write: %v (should be allow — no write-deny match)", eval1Wrong.Decision)
	eval2Wrong := policy.EvaluateFilesystem(context.Background(), corepermission.FilesystemRequest{
		ToolName:   Name,
		Path:       result.Paths[1],
		WorkingDir: "/home/user",
		Access:     corepermission.AccessRead,
	})
	t.Logf("dest checked as read: %v (should be allow — no read-deny match)", eval2Wrong.Decision)
}

// TestFilesystemPolicyInvokeWebRequestMixedIO verifies that Invoke-WebRequest's
// -InFile is treated as read and -OutFile as write.
func TestFilesystemPolicyInvokeWebRequestMixedIO(t *testing.T) {
	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{
		Write: []corepermission.Rule{
			{
				Source:   corepermission.RuleSourceSession,
				Decision: corepermission.DecisionDeny,
				Pattern:  "**/nowrite/**",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy: %v", err)
	}

	cmd := ParsedCommandElement{
		Name:         "Invoke-WebRequest",
		NameType:     "cmdlet",
		ElementType:  "CommandAst",
		Args:         []string{"-Uri", "https://example.com", "-OutFile", "/home/user/nowrite/output.html", "-InFile", "/tmp/input.txt"},
		ElementTypes: []string{"Parameter", "StringConstant", "Parameter", "StringConstant", "Parameter", "StringConstant"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(result.Paths), result.Paths)
	}
	if len(result.PathAccess) != 2 {
		t.Fatalf("expected 2 path access entries, got %d", len(result.PathAccess))
	}

	// -OutFile should be write, -InFile should be read
	// (PositionalSkip=1 skips -Uri)
	if result.PathAccess[0] != opWrite {
		t.Errorf("expected -OutFile to be opWrite, got %v", result.PathAccess[0])
	}
	if result.PathAccess[1] != opRead {
		t.Errorf("expected -InFile to be opRead, got %v", result.PathAccess[1])
	}

	// Verify -OutFile write path is correctly denied by write-deny rule
	eval := policy.EvaluateFilesystem(context.Background(), corepermission.FilesystemRequest{
		ToolName:   Name,
		Path:       "/home/user/nowrite/output.html",
		WorkingDir: "/home/user",
		Access:     corepermission.AccessWrite,
	})
	if eval.Decision != corepermission.DecisionDeny {
		t.Errorf("expected -OutFile write path to be denied, got %v", eval.Decision)
	}
}

// TestFilesystemPolicyExpandArchiveMixedIO verifies that Expand-Archive's
// source path (-Path) is treated as read and -DestinationPath as write.
func TestFilesystemPolicyExpandArchiveMixedIO(t *testing.T) {
	cmd := ParsedCommandElement{
		Name:         "Expand-Archive",
		NameType:     "cmdlet",
		ElementType:  "CommandAst",
		Args:         []string{"-Path", "/home/user/nowrite/archive.zip", "-DestinationPath", "/tmp/output"},
		ElementTypes: []string{"Parameter", "StringConstant", "Parameter", "StringConstant"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(result.Paths), result.Paths)
	}
	if len(result.PathAccess) != 2 {
		t.Fatalf("expected 2 path access entries, got %d", len(result.PathAccess))
	}

	// -Path is the source archive → read, -DestinationPath is the output → write
	if result.PathAccess[0] != opRead {
		t.Errorf("expected -Path (source) to be opRead, got %v", result.PathAccess[0])
	}
	if result.PathAccess[1] != opWrite {
		t.Errorf("expected -DestinationPath to be opWrite, got %v", result.PathAccess[1])
	}
}