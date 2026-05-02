package extractmemories

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// isPathWithinDir checks whether the given file path is within the specified
// directory, using clean absolute paths and a separator-aware prefix check.
func isPathWithinDir(filePath, dir string) bool {
	cleanFP := filepath.Clean(filePath)
	cleanDir := filepath.Clean(dir)

	if cleanFP == cleanDir {
		return true
	}

	dirPrefix := cleanDir
	if !strings.HasSuffix(dirPrefix, string(os.PathSeparator)) {
		dirPrefix += string(os.PathSeparator)
	}

	return strings.HasPrefix(cleanFP, dirPrefix)
}

// ToolPermDecision represents a decision on whether a tool is allowed for the
// memory extraction subagent.
type ToolPermDecision struct {
	// Behavior is "allow" or "deny".
	Behavior string
	// Reason is the human-readable reason for a deny decision.
	Reason string
}

// CanUseToolFn is the signature for a tool permission check function.
// It inspects the tool name and its input, and returns a decision.
type CanUseToolFn func(toolName string, input map[string]any) ToolPermDecision

// CreateAutoMemCanUseTool creates a CanUseToolFn that enforces the
// extractMemories tool permission policy:
//   - Read/Grep/Glob: unrestricted allow
//   - Bash: allow only for read-only commands
//   - Write/Edit: allow only when file_path is within the memory directory
//   - All other tools: deny with reason
func CreateAutoMemCanUseTool(memoryDir string) CanUseToolFn {
	return func(toolName string, input map[string]any) ToolPermDecision {
		// Allow Read/Grep/Glob — all inherently read-only.
		if isReadToolUse(toolName) {
			return ToolPermDecision{Behavior: "allow"}
		}
		if toolNameMatches(toolName, grepToolNames) {
			return ToolPermDecision{Behavior: "allow"}
		}
		if toolNameMatches(toolName, globToolNames) {
			return ToolPermDecision{Behavior: "allow"}
		}

		// Allow Bash only for read-only commands.
		if toolNameMatches(toolName, bashToolNames) {
			if isReadOnlyBashCommand(input) {
				return ToolPermDecision{Behavior: "allow"}
			}
			return ToolPermDecision{
				Behavior: "deny",
				Reason:   "Only read-only shell commands are permitted in this context (ls, find, grep, cat, stat, wc, head, tail, and similar)",
			}
		}

		// Allow Write/Edit only for paths within the memory directory.
		if isWriteOrEditToolUse(toolName) {
			if filePath, ok := input["file_path"]; ok {
				if fp, ok := filePath.(string); ok && fp != "" {
					if isPathWithinDir(fp, memoryDir) {
						return ToolPermDecision{Behavior: "allow"}
					}
				}
			}
			return ToolPermDecision{
				Behavior: "deny",
				Reason:   fmt.Sprintf("only Edit/Write within %s is allowed", memoryDir),
			}
		}

		// Deny all other tools.
		return ToolPermDecision{
			Behavior: "deny",
			Reason: fmt.Sprintf(
				"only Read, Grep, Glob, read-only Bash, and Edit/Write within %s are allowed",
				memoryDir,
			),
		}
	}
}

// readOnlyBashCommands is the set of commands that are considered safe for
// read-only Bash execution. Commands that can write files (e.g., sed -i,
// awk with system(), echo with redirection) are excluded.
var readOnlyBashCommands = map[string]bool{
	"ls":       true,
	"find":     true,
	"grep":     true,
	"cat":      true,
	"stat":     true,
	"wc":       true,
	"head":     true,
	"tail":     true,
	"file":     true,
	"du":       true,
	"df":       true,
	"pwd":      true,
	"which":    true,
	"type":     true,
	"env":      true,
	"printenv": true,
}

// dangerousShellChars is the set of shell metacharacters that can enable
// destructive behavior (redirection, piping, command substitution, backgrounding).
var dangerousShellChars = []string{">", "|", ";", "&", "$(", "`"}

// isReadOnlyBashCommand checks if a Bash tool input represents a read-only
// command. It looks at the "command" field in the input, validates the base
// command against the known read-only list, and rejects commands containing
// shell metacharacters that enable writes or execution.
func isReadOnlyBashCommand(input map[string]any) bool {
	cmd, ok := input["command"]
	if !ok {
		return false
	}
	cmdStr, ok := cmd.(string)
	if !ok || cmdStr == "" {
		return false
	}

	// Reject commands containing dangerous shell metacharacters.
	for _, ch := range dangerousShellChars {
		if strings.Contains(cmdStr, ch) {
			return false
		}
	}

	// Get the first word of the command.
	firstWord := strings.Fields(strings.TrimSpace(cmdStr))
	if len(firstWord) == 0 {
		return false
	}

	base := firstWord[0]
	// Strip path prefixes like /usr/bin/ls → ls.
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}

	return readOnlyBashCommands[base]
}
