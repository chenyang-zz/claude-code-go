package powershell

import (
	"testing"
)

func TestExtractBaseCommand(t *testing.T) {
	tests := []struct {
		segment  string
		expected string
	}{
		{"grep -r pattern", "grep"},
		{"rg pattern", "rg"},
		{"findstr /s pattern", "findstr"},
		{"& \"grep.exe\" -r pattern", "grep"},
		{"./rg.exe pattern", "rg"},
		{"C:\\bin\\robocopy.exe C:\\src C:\\dst", "robocopy"},
		{".\\findstr.exe pattern", "findstr"},
		{"Get-ChildItem C:\\", "get-childitem"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.segment, func(t *testing.T) {
			got := extractBaseCommand(tt.segment)
			if got != tt.expected {
				t.Errorf("extractBaseCommand(%q) = %q, want %q", tt.segment, got, tt.expected)
			}
		})
	}
}

func TestHeuristicallyExtractBaseCommand(t *testing.T) {
	tests := []struct {
		command  string
		expected string
	}{
		{"grep -r pattern", "grep"},
		{"Get-ChildItem C:\\; grep -r pattern", "grep"},
		{"Get-ChildItem | Select-String pattern", "select-string"},
		{"Get-Process; Get-Service", "get-service"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := heuristicallyExtractBaseCommand(tt.command)
			if got != tt.expected {
				t.Errorf("heuristicallyExtractBaseCommand(%q) = %q, want %q", tt.command, got, tt.expected)
			}
		})
	}
}

func TestInterpretCommandResultDefault(t *testing.T) {
	// Default semantic: only 0 is success
	tests := []struct {
		name     string
		exitCode int
		isError  bool
	}{
		{"exit 0", 0, false},
		{"exit 1", 1, true},
		{"exit 127", 127, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interpretCommandResult("Get-ChildItem", tt.exitCode, "", "")
			if result.isError != tt.isError {
				t.Errorf("exit %d: isError=%v, want %v", tt.exitCode, result.isError, tt.isError)
			}
		})
	}
}

func TestInterpretCommandResultGrep(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		exitCode int
		isError  bool
		message  string
	}{
		{"grep matches", "grep -r pattern .", 0, false, ""},
		{"grep no match", "grep pattern file.txt", 1, false, "No matches found"},
		{"grep error", "grep -r pattern .", 2, true, ""},
		{"rg matches", "rg pattern", 0, false, ""},
		{"rg no match", "rg pattern", 1, false, "No matches found"},
		{"findstr matches", "findstr pattern file.txt", 0, false, ""},
		{"findstr no match", "findstr /s pattern", 1, false, "No matches found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interpretCommandResult(tt.command, tt.exitCode, "", "")
			if result.isError != tt.isError {
				t.Errorf("isError=%v, want %v", result.isError, tt.isError)
			}
			if result.message != tt.message {
				t.Errorf("message=%q, want %q", result.message, tt.message)
			}
		})
	}
}

func TestInterpretCommandResultRobocopy(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
		isError  bool
		message  string
	}{
		{"in sync", 0, false, "No files copied (already in sync)"},
		{"files copied", 1, false, "Files copied successfully"},
		{"extra files", 2, false, "Robocopy completed (no errors)"},
		{"mixed success", 3, false, "Files copied successfully"},
		{"mismatched", 4, false, "Robocopy completed (no errors)"},
		{"errors", 8, true, ""},
		{"serious error", 16, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interpretCommandResult("robocopy C:\\src C:\\dst", tt.exitCode, "", "")
			if result.isError != tt.isError {
				t.Errorf("isError=%v, want %v", result.isError, tt.isError)
			}
			if result.message != tt.message {
				t.Errorf("message=%q, want %q", result.message, tt.message)
			}
		})
	}
}
