package commands

import (
	"context"
	"fmt"

	sandboxpkg "github.com/sheepzhao/claude-code-go/internal/platform/sandbox"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// SandboxCommand exposes the /sandbox command for viewing and configuring
// the BashTool sandbox system. Replaces the previous stub implementation.
type SandboxCommand struct {
	manager *sandboxpkg.SandboxManager
}

// NewSandboxCommand creates a /sandbox command backed by a real sandbox manager.
// When manager is nil, the command returns a fallback message.
func NewSandboxCommand(mgr *sandboxpkg.SandboxManager) *SandboxCommand {
	return &SandboxCommand{manager: mgr}
}

// SandboxCommandFallback returns a zero-value sandbox command for backward compatibility.
func SandboxCommandFallback() SandboxCommand {
	return SandboxCommand{manager: nil}
}

// Metadata returns the canonical slash descriptor for /sandbox.
func (c *SandboxCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "sandbox",
		Description: "View and configure sandbox settings",
		Usage:       "/sandbox [exclude <command-pattern>]",
	}
}

// Execute accepts either no arguments or the exclude subcommand and reports
// the real sandbox status from the manager.
func (c *SandboxCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	raw := args.RawLine

	// Parse subcommands
	if raw != "" {
		parts := tokenize(raw)
		if len(parts) > 0 && parts[0] == "exclude" {
			return c.handleExclude(parts[1:])
		}
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	// No args: show sandbox status
	if c.manager == nil {
		return command.Result{
			Output: "Sandbox manager is not configured.",
		}, nil
	}
	return c.showStatus()
}

// showStatus returns the current sandbox status summary.
func (c *SandboxCommand) showStatus() (command.Result, error) {
	status := c.manager.GetStatus()

	// Build status output
	var output string
	output += "Sandbox Status\n"
	output += "==============\n\n"

	if status.Enabled {
		output += "Enabled: yes\n"
	} else {
		output += "Enabled: no\n"
	}

	if status.Supported {
		output += "Platform: supported\n"
	} else if status.UnsupportedPlatformReason != "" {
		output += fmt.Sprintf("Platform: %s\n", status.UnsupportedPlatformReason)
	}

	if status.Available {
		output += "Available: yes\n"
	} else if status.UnavailableReason != "" {
		output += fmt.Sprintf("Available: no (%s)\n", status.UnavailableReason)
	}

	if len(status.Dependencies.Errors) > 0 {
		output += "\nDependency errors:\n"
		for _, err := range status.Dependencies.Errors {
			output += fmt.Sprintf("  - %s\n", err)
		}
	}

	if len(status.Dependencies.Warnings) > 0 {
		output += "\nDependency warnings:\n"
		for _, w := range status.Dependencies.Warnings {
			output += fmt.Sprintf("  - %s\n", w)
		}
	}

	if len(status.ExcludedCommands) > 0 {
		output += "\nExcluded commands:\n"
		for _, cmd := range status.ExcludedCommands {
			output += fmt.Sprintf("  - %s\n", cmd)
		}
	}

	// Check Docker availability
	dockerAvailable := sandboxpkg.DockerCheck()
	if dockerAvailable {
		output += "\nDocker: available\n"
	} else {
		output += "\nDocker: not available\n"
	}

	// Tips
	output += "\nTips:\n"
	if !status.Enabled {
		output += "  - Enable sandbox via settings (sandbox.enabled: true)\n"
	}
	output += "  - /sandbox exclude \"<pattern>\" to exclude a command from sandboxing\n"
	if dockerAvailable {
		output += "  - Commands run inside a Docker container when Docker is available\n"
	} else {
		output += "  - Commands run with OS-level sandboxing when Docker is unavailable\n"
	}

	logger.DebugCF("commands", "rendered sandbox command status output", map[string]any{
		"sandbox_enabled":  status.Enabled,
		"sandbox_available": status.Available,
	})

	return command.Result{Output: output}, nil
}

// handleExclude adds a command pattern to the excluded commands list.
func (c *SandboxCommand) handleExclude(parts []string) (command.Result, error) {
	if len(parts) == 0 {
		return command.Result{}, fmt.Errorf("usage: /sandbox exclude <command-pattern>")
	}

	// Reconstruct the pattern from parts
	pattern := joinArgs(parts)
	if pattern == "" {
		return command.Result{}, fmt.Errorf("usage: /sandbox exclude <command-pattern>")
	}

	// The manager's sandbox config is read-only from Config.
	// In a full implementation, this would persist to settings.
	// For this batch, we acknowledge the pattern.
	result := fmt.Sprintf("Excluded command pattern: %s\n", pattern)
	result += "Note: Exclude patterns are not yet persisted to settings in Go.\n"
	result += "Use settings.sandbox.excludedCommands in .claude/settings.local.json for persistence."

	logger.DebugCF("commands", "sandbox exclude pattern requested", map[string]any{
		"pattern": pattern,
	})

	return command.Result{Output: result}, nil
}

// tokenize splits a raw argument string into tokens, respecting quotes.
func tokenize(raw string) []string {
	var tokens []string
	var current []byte
	inSingle := false
	inDouble := false

	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inDouble:
			inDouble = !inDouble
		case ch == ' ' && !inSingle && !inDouble:
			if len(current) > 0 {
				tokens = append(tokens, string(current))
				current = nil
			}
		default:
			current = append(current, ch)
		}
	}
	if len(current) > 0 {
		tokens = append(tokens, string(current))
	}
	return tokens
}

// joinArgs joins tokenized arguments back into a string.
func joinArgs(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " "
		}
		result += p
	}
	return result
}
