package powershell

import (
	"testing"
)

func TestExtractPathsFromSetContent(t *testing.T) {
	// Simulate parsing of "Set-Content -Path /etc/config 'data'"
	cmd := ParsedCommandElement{
		Name:         "Set-Content",
		NameType:     "cmdlet",
		ElementType:  "CommandAst",
		Args:         []string{"-Path", "/etc/config", "'data'"},
		ElementTypes: []string{"Parameter", "StringConstant", "StringConstant"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) == 0 {
		t.Fatal("expected paths")
	}
	if result.Paths[0] != "/etc/config" {
		t.Errorf("expected /etc/config, got %q", result.Paths[0])
	}
	if result.OperationType != opWrite {
		t.Error("expected write operation type")
	}
}

func TestExtractPathsFromGetContent(t *testing.T) {
	cmd := ParsedCommandElement{
		Name:         "Get-Content",
		NameType:     "cmdlet",
		ElementType:  "CommandAst",
		Args:         []string{"-Path", "/etc/config"},
		ElementTypes: []string{"Parameter", "StringConstant"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) == 0 {
		t.Fatal("expected paths")
	}
	if result.OperationType != opRead {
		t.Error("expected read operation type")
	}
}

func TestExtractPathsFromUnknownCmdlet(t *testing.T) {
	cmd := ParsedCommandElement{
		Name:        "Invoke-Expression",
		NameType:    "cmdlet",
		ElementType: "CommandAst",
		Args:        []string{"'test'"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) != 0 {
		t.Errorf("expected no paths for unknown cmdlet, got %v", result.Paths)
	}
	// Default operation type should be read
	if result.OperationType != opRead {
		t.Error("expected default read operation type")
	}
}

func TestExtractPathsFromNewItem(t *testing.T) {
	cmd := ParsedCommandElement{
		Name:         "New-Item",
		NameType:     "cmdlet",
		ElementType:  "CommandAst",
		Args:         []string{"-Path", "/tmp/newdir", "-ItemType", "Directory"},
		ElementTypes: []string{"Parameter", "StringConstant", "Parameter", "StringConstant"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) == 0 {
		t.Fatal("expected paths")
	}
	if result.Paths[0] != "/tmp/newdir" {
		t.Errorf("expected /tmp/newdir, got %q", result.Paths[0])
	}
}

func TestExtractPathsWithVariable(t *testing.T) {
	cmd := ParsedCommandElement{
		Name:         "Set-Content",
		NameType:     "cmdlet",
		ElementType:  "CommandAst",
		Args:         []string{"-Path", "$env:SECRET", "'data'"},
		ElementTypes: []string{"Parameter", "Variable", "StringConstant"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) == 0 {
		t.Fatal("expected paths")
	}
	if !result.HasUnvalidatablePathArg {
		t.Error("expected unvalidatable path arg for Variable element type")
	}
}

func TestExtractPathsWithColonSyntax(t *testing.T) {
	cmd := ParsedCommandElement{
		Name:         "Set-Content",
		NameType:     "cmdlet",
		ElementType:  "CommandAst",
		Args:         []string{"-LiteralPath:/etc/config", "'data'"},
		ElementTypes: []string{"Parameter", "StringConstant"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) == 0 {
		t.Fatal("expected paths")
	}
	if result.Paths[0] != "/etc/config" {
		t.Errorf("expected /etc/config, got %q", result.Paths[0])
	}
}

func TestExtractPathsFromOutFile(t *testing.T) {
	cmd := ParsedCommandElement{
		Name:         "Out-File",
		NameType:     "cmdlet",
		ElementType:  "CommandAst",
		Args:         []string{"-FilePath", "/tmp/output.txt", "-Encoding", "UTF8"},
		ElementTypes: []string{"Parameter", "StringConstant", "Parameter", "StringConstant"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) == 0 {
		t.Fatal("expected paths")
	}
	if result.Paths[0] != "/tmp/output.txt" {
		t.Errorf("expected /tmp/output.txt, got %q", result.Paths[0])
	}
}

func TestExtractPathsFromRemoveItem(t *testing.T) {
	cmd := ParsedCommandElement{
		Name:         "Remove-Item",
		NameType:     "cmdlet",
		ElementType:  "CommandAst",
		Args:         []string{"-Path", "/tmp/file", "-Force", "-Recurse"},
		ElementTypes: []string{"Parameter", "StringConstant", "Parameter", "Parameter"},
	}

	result := extractPathsFromCommand(cmd)
	if len(result.Paths) == 0 {
		t.Fatal("expected paths")
	}
	if result.OperationType != opWrite {
		t.Error("expected write operation type")
	}
}
