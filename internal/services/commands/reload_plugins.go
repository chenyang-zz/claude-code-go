package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/platform/plugin"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ReloadPluginsCommand triggers a full plugin refresh and returns a summary.
// When a Reloader is configured, it uses the full unregister-refresh-register
// pipeline; otherwise it falls back to direct loader+registrar calls.
type ReloadPluginsCommand struct {
	Reloader  *plugin.Reloader
	Loader    *plugin.PluginLoader
	Registrar *plugin.PluginRegistrar
}

// Metadata returns the canonical slash descriptor for /reload-plugins.
func (c ReloadPluginsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "reload-plugins",
		Description: "Activate pending plugin changes in the current session",
		Usage:       "/reload-plugins",
	}
}

// Execute performs a full plugin refresh and, when a registrar is configured,
// registers all extracted capabilities with the Go runtime subsystems.
func (c ReloadPluginsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	var result *plugin.RefreshResult
	var summary *plugin.RegistrationSummary
	var err error

	// Prefer the reloader pipeline when available.
	if c.Reloader != nil {
		summary, err = c.Reloader.Reload()
		if err != nil {
			logger.WarnCF("commands", "plugin reload failed", map[string]any{
				"error": err.Error(),
			})
			return command.Result{}, fmt.Errorf("plugin reload failed: %w", err)
		}
		// Extract result counts from summary when available; fallback to zero.
		result = &plugin.RefreshResult{}
	} else {
		if c.Loader == nil {
			return command.Result{}, fmt.Errorf("plugin loader is not available")
		}
		result, err = c.Loader.RefreshActivePlugins()
		if err != nil {
			logger.WarnCF("commands", "plugin refresh failed", map[string]any{
				"error": err.Error(),
			})
			return command.Result{}, fmt.Errorf("plugin refresh failed: %w", err)
		}
		if c.Registrar != nil {
			summary, err = c.Registrar.RegisterAll(result, nil)
			if err != nil {
				logger.WarnCF("commands", "plugin registration failed", map[string]any{
					"error": err.Error(),
				})
				return command.Result{}, fmt.Errorf("plugin registration failed: %w", err)
			}
		}
	}

	output := fmt.Sprintf(
		"Reloaded %d enabled plugin(s), %d disabled.\n\n%s",
		result.EnabledCount,
		result.DisabledCount,
		formatReloadSummary(result, summary),
	)

	return command.Result{
		Output: output,
	}, nil
}

func formatReloadSummary(result *plugin.RefreshResult, summary *plugin.RegistrationSummary) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Commands: %d", result.CommandCount))
	parts = append(parts, fmt.Sprintf("Agents: %d", result.AgentCount))
	parts = append(parts, fmt.Sprintf("MCP servers: %d", result.McpCount))
	parts = append(parts, fmt.Sprintf("LSP servers: %d", result.LspCount))
	parts = append(parts, fmt.Sprintf("Hook events: %d", result.HookCount))
	if result.ErrorCount > 0 {
		parts = append(parts, fmt.Sprintf("Errors: %d", result.ErrorCount))
	}
	if summary != nil {
		parts = append(parts, "")
		parts = append(parts, summary.FormatSummary())
	}
	return strings.Join(parts, "\n")
}
