package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/platform/plugin"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// PluginCommand provides plugin status and management.
type PluginCommand struct {
	Loader *plugin.PluginLoader
}

// Metadata returns the canonical slash descriptor for /plugin.
func (c PluginCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "plugin",
		Aliases:     []string{"plugins", "marketplace"},
		Description: "Manage Claude Code plugins",
		Usage:       "/plugin [subcommand]",
	}
}

// Execute reports the current plugin load status. Without arguments it prints
// a summary of enabled plugins and their capability counts.
func (c PluginCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	if c.Loader == nil {
		return command.Result{
			Output: "Plugin management is not available in this session.",
		}, nil
	}

	result, err := c.Loader.RefreshActivePlugins()
	if err != nil {
		logger.WarnCF("commands", "plugin status refresh failed", map[string]any{
			"error": err.Error(),
		})
		return command.Result{}, fmt.Errorf("failed to load plugin status: %w", err)
	}

	output := formatPluginStatus(result, args.Raw)
	return command.Result{
		Output: output,
	}, nil
}

func formatPluginStatus(result *plugin.RefreshResult, subcommands []string) string {
	// Minimal subcommand support: "list" or no args shows the summary.
	if len(subcommands) > 0 {
		switch subcommands[0] {
		case "list", "ls":
			return formatPluginList(result)
		default:
			return fmt.Sprintf("Unknown subcommand %q. Try /plugin or /plugin list.", subcommands[0])
		}
	}

	var parts []string
	parts = append(parts, "## Plugin Status")
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("- Enabled plugins: %d", result.EnabledCount))
	parts = append(parts, fmt.Sprintf("- Disabled plugins: %d", result.DisabledCount))
	parts = append(parts, fmt.Sprintf("- Commands: %d", result.CommandCount))
	parts = append(parts, fmt.Sprintf("- Agents: %d", result.AgentCount))
	parts = append(parts, fmt.Sprintf("- MCP servers: %d", result.McpCount))
	parts = append(parts, fmt.Sprintf("- LSP servers: %d", result.LspCount))
	parts = append(parts, fmt.Sprintf("- Hook events: %d", result.HookCount))
	if result.ErrorCount > 0 {
		parts = append(parts, fmt.Sprintf("- Errors: %d", result.ErrorCount))
	}
	if len(result.Plugins) > 0 {
		parts = append(parts, "")
		parts = append(parts, "### Loaded Plugins")
		for _, p := range result.Plugins {
			parts = append(parts, fmt.Sprintf("- %s (%s)", p.Name, p.Source.Value))
		}
	}
	return strings.Join(parts, "\n")
}

func formatPluginList(result *plugin.RefreshResult) string {
	if len(result.Plugins) == 0 {
		return "No plugins loaded."
	}
	var parts []string
	parts = append(parts, "Loaded plugins:")
	for _, p := range result.Plugins {
		parts = append(parts, fmt.Sprintf("  - %s @ %s", p.Name, p.Path))
	}
	return strings.Join(parts, "\n")
}
