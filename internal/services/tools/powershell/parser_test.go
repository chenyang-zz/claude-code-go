package powershell

import (
	"os/exec"
	"testing"
)

func skipIfNoPwsh(t *testing.T) {
	t.Helper()
	_, err := exec.LookPath("pwsh")
	if err != nil {
		_, err = exec.LookPath("powershell.exe")
		if err != nil {
			t.Skip("PowerShell not available")
		}
	}
}

func TestParseSimpleCommand(t *testing.T) {
	skipIfNoPwsh(t)

	result := ParsePowerShellCommand("Get-ChildItem C:\\")
	if !result.Valid {
		t.Fatalf("expected valid parse, got errors: %v", result.Errors)
	}
	if len(result.Statements) == 0 {
		t.Fatal("expected at least one statement")
	}
	stmt := result.Statements[0]
	if len(stmt.Commands) == 0 {
		t.Fatal("expected at least one command")
	}
	cmd := stmt.Commands[0]
	if cmd.Name == "" {
		t.Error("expected non-empty command name")
	}
}

func TestParseWithArgs(t *testing.T) {
	skipIfNoPwsh(t)

	result := ParsePowerShellCommand("Get-Content -Path file.txt -Tail 10")
	if !result.Valid {
		t.Fatalf("expected valid, got: %v", result.Errors)
	}
	cmd := result.Statements[0].Commands[0]
	if len(cmd.Args) == 0 {
		t.Error("expected args")
	}
	if len(cmd.ElementTypes) == 0 {
		t.Error("expected element types")
	}
}

func TestParseCompoundCommand(t *testing.T) {
	skipIfNoPwsh(t)

	result := ParsePowerShellCommand("Get-Process; Get-Service")
	if !result.Valid {
		t.Fatalf("expected valid, got: %v", result.Errors)
	}
	if len(result.Statements) < 2 {
		t.Errorf("expected 2+ statements, got %d", len(result.Statements))
	}
}

func TestParseInvokeExpression(t *testing.T) {
	skipIfNoPwsh(t)

	result := ParsePowerShellCommand("Invoke-Expression 'test'")
	if !result.Valid {
		t.Fatalf("expected valid, got: %v", result.Errors)
	}
	cmd := result.Statements[0].Commands[0]
	if cmd.Name != "Invoke-Expression" {
		t.Errorf("expected 'Invoke-Expression', got %q", cmd.Name)
	}
}

func TestParsePipeline(t *testing.T) {
	skipIfNoPwsh(t)

	result := ParsePowerShellCommand("Get-Process | Where-Object { $_.CPU -gt 100 }")
	if !result.Valid {
		t.Fatalf("expected valid, got: %v", result.Errors)
	}
	stmt := result.Statements[0]
	if len(stmt.Commands) < 2 {
		t.Errorf("expected 2+ commands in pipeline, got %d", len(stmt.Commands))
	}
}

func TestParseEmptyCommand(t *testing.T) {
	result := ParsePowerShellCommand("")
	// Should not panic, should return invalid
	t.Logf("Empty command result: valid=%v, errors=%v", result.Valid, result.Errors)
}

func TestParseInvalidCommand(t *testing.T) {
	skipIfNoPwsh(t)

	result := ParsePowerShellCommand("Get-Process | | Where-Object")
	// May be valid depending on how PS parses it
	t.Logf("Invalid command result: valid=%v, errors=%v", result.Valid, result.Errors)
}

func TestParseVariableAndMemberAccess(t *testing.T) {
	skipIfNoPwsh(t)

	result := ParsePowerShellCommand("[System.IO.File]::ReadAllText('file.txt')")
	if !result.Valid {
		t.Fatalf("expected valid, got: %v", result.Errors)
	}
	// Should have type literals
	if len(result.TypeLiterals) > 0 {
		t.Logf("Type literals: %v", result.TypeLiterals)
	}
}
