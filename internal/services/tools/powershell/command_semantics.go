package powershell

import (
	"strings"
)

// commandSemantic interprets exit codes for one executable.
type commandSemantic func(exitCode int, stdout, stderr string) semanticResult

type semanticResult struct {
	isError bool
	message string
}

// defaultSemantic treats only exit code 0 as success.
func defaultSemantic(exitCode int, _, _ string) semanticResult {
	if exitCode != 0 {
		return semanticResult{isError: true, message: ""}
	}
	return semanticResult{}
}

// grepSemantic: 0 = matches, 1 = no matches, 2+ = error.
func grepSemantic(exitCode int, _, _ string) semanticResult {
	switch {
	case exitCode == 1:
		return semanticResult{message: "No matches found"}
	case exitCode >= 2:
		return semanticResult{isError: true}
	default:
		return semanticResult{}
	}
}

// robocopySemantic: 0-7 = success (bitfield), 8+ = error.
func robocopySemantic(exitCode int, _, _ string) semanticResult {
	switch {
	case exitCode == 0:
		return semanticResult{message: "No files copied (already in sync)"}
	case exitCode >= 1 && exitCode < 8:
		if exitCode&1 != 0 {
			return semanticResult{message: "Files copied successfully"}
		}
		return semanticResult{message: "Robocopy completed (no errors)"}
	default:
		return semanticResult{isError: exitCode >= 8}
	}
}

// commandSemanticsMap maps lowercase command names (without .exe) to semantic functions.
var commandSemanticsMap = map[string]commandSemantic{
	"grep":    grepSemantic,
	"rg":      grepSemantic,
	"findstr": grepSemantic,
	"robocopy": robocopySemantic,
}

// extractBaseCommand extracts the base command name from a PowerShell pipeline segment.
// Strips call operators (&, .), quotes, path prefixes, and .exe suffix.
func extractBaseCommand(segment string) string {
	// Strip PowerShell call operators: & "cmd", . "cmd"
	stripped := strings.TrimSpace(segment)
	stripped = strings.TrimLeft(stripped, "&. ")
	if len(stripped) > 0 && (stripped[0] == '&' || stripped[0] == '.') {
		// Handle cases where there's no space after the operator
		stripped = strings.TrimSpace(stripped[1:])
	}

	// First token
	tokens := strings.Fields(stripped)
	if len(tokens) == 0 {
		return ""
	}
	first := tokens[0]

	// Strip surrounding quotes
	first = strings.Trim(first, `"'`)

	// Strip path: C:\bin\grep.exe → grep.exe, ./rg.exe → rg.exe
	if idx := strings.LastIndexAny(first, `\/`); idx >= 0 {
		first = first[idx+1:]
	}

	// Strip .exe suffix
	lower := strings.ToLower(first)
	lower = strings.TrimSuffix(lower, ".exe")

	return lower
}

// heuristicallyExtractBaseCommand extracts the primary command from a PowerShell command line.
// Takes the LAST pipeline segment since that determines the exit code.
func heuristicallyExtractBaseCommand(command string) string {
	segments := strings.FieldsFunc(command, func(r rune) bool {
		return r == ';' || r == '|'
	})
	if len(segments) == 0 {
		return extractBaseCommand(command)
	}
	// Filter empty segments
	var nonEmpty []string
	for _, s := range segments {
		if strings.TrimSpace(s) != "" {
			nonEmpty = append(nonEmpty, s)
		}
	}
	if len(nonEmpty) == 0 {
		return extractBaseCommand(command)
	}
	return extractBaseCommand(nonEmpty[len(nonEmpty)-1])
}

// interpretCommandResult interprets a command result based on semantic rules.
func interpretCommandResult(command string, exitCode int, stdout, stderr string) semanticResult {
	baseCmd := heuristicallyExtractBaseCommand(command)
	if semantic, ok := commandSemanticsMap[baseCmd]; ok {
		return semantic(exitCode, stdout, stderr)
	}
	return defaultSemantic(exitCode, stdout, stderr)
}
