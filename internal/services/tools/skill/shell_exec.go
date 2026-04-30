package skill

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ShellExecFunc executes a shell command and returns stdout and stderr.
// workingDir is the directory in which the command should run.
type ShellExecFunc func(ctx context.Context, command string, workingDir string) (stdout, stderr string, err error)

// ShellExecutor is set by bootstrap to enable shell command execution in skill content.
// When non-nil, Skill.Execute processes inline !`...` and block ```! ... ```
// patterns found in the skill body, replaces them with command output, and
// returns the transformed content to the model.
var ShellExecutor ShellExecFunc

// blockPattern matches fenced shell command blocks: ```! command ```
var blockPattern = regexp.MustCompile("```!\\s*\\n?([\\s\\S]*?)\\n?```")

// inlinePattern matches inline shell commands: !`command`
// The pattern requires a leading space or start-of-string before ! to reduce
// false matches inside markdown inline code spans and shell variables like $!.
var inlinePattern = regexp.MustCompile("(?:^|\\s)!`([^`]+)`")

// hasShellCommands checks whether the content contains any shell command
// patterns that need processing. This is a cheap pre-check to avoid
// running the full regex scan on content without shell commands.
func hasShellCommands(content string) bool {
	return strings.Contains(content, "```!") || strings.Contains(content, "!`")
}

// processShellCommands finds all inline and block shell command patterns in
// content, executes each one through the provided executor, and returns the
// content with matches replaced by command output.
func processShellCommands(ctx context.Context, content string, executor ShellExecFunc, workingDir string) (string, error) {
	if executor == nil {
		return content, nil
	}
	if !hasShellCommands(content) {
		return content, nil
	}

	result := content

	// Process block patterns first (they are longer and less likely to
	// accidentally overlap with inline patterns).
	blockMatches := blockPattern.FindAllStringSubmatchIndex(result, -1)
	for i := len(blockMatches) - 1; i >= 0; i-- {
		match := blockMatches[i]
		fullStart, fullEnd := match[0], match[1]
		cmdStart, cmdEnd := match[2], match[3]
		command := strings.TrimSpace(result[cmdStart:cmdEnd])
		if command == "" {
			continue
		}

		replacement := executeAndFormat(ctx, executor, command, workingDir, false)
		result = result[:fullStart] + replacement + result[fullEnd:]
	}

	// Process inline patterns.
	// Gated on !` presence for performance (inlinePattern's regex is
	// more expensive than blockPattern).
	if strings.Contains(result, "!`") {
		inlineMatches := inlinePattern.FindAllStringSubmatchIndex(result, -1)
		for i := len(inlineMatches) - 1; i >= 0; i-- {
			match := inlineMatches[i]
			fullStart, fullEnd := match[0], match[1]
			cmdStart, cmdEnd := match[2], match[3]
			command := strings.TrimSpace(result[cmdStart:cmdEnd])
			if command == "" {
				continue
			}

			// Preserve the leading character (space or start-of-string)
			// that was consumed by the inline pattern.
			var prefix string
			if fullStart > 0 && result[fullStart] == ' ' {
				prefix = " "
			}
			replacement := executeAndFormat(ctx, executor, command, workingDir, true)
			result = result[:fullStart] + prefix + replacement + result[fullEnd:]
		}
	}

	return result, nil
}

// executeAndFormat runs a single shell command and formats its output for
// insertion into the skill content. On error, the error is formatted and
// returned as a MalformedCommandError that surfaces back to the caller.
func executeAndFormat(ctx context.Context, executor ShellExecFunc, command string, workingDir string, inline bool) string {
	stdout, stderr, err := executor(ctx, command, workingDir)
	if err != nil {
		logger.WarnCF("skill", "shell command failed in skill", map[string]any{
			"command": command,
			"error":   err.Error(),
		})
		if inline {
			return fmt.Sprintf("[Error: %s]", err.Error())
		}
		return fmt.Sprintf("[Error]\n%s", err.Error())
	}

	return formatShellOutput(stdout, stderr, inline)
}

// formatShellOutput formats stdout and stderr for display in skill content.
func formatShellOutput(stdout, stderr string, inline bool) string {
	var parts []string

	if strings.TrimSpace(stdout) != "" {
		parts = append(parts, strings.TrimSpace(stdout))
	}

	if strings.TrimSpace(stderr) != "" {
		if inline {
			parts = append(parts, fmt.Sprintf("[stderr: %s]", strings.TrimSpace(stderr)))
		} else {
			parts = append(parts, fmt.Sprintf("[stderr]\n%s", strings.TrimSpace(stderr)))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	if inline {
		return strings.Join(parts, " ")
	}
	return strings.Join(parts, "\n")
}
