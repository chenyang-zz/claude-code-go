package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const pluginCommandFallback = "Plugin management is not available in Claude Code Go yet. Plugin settings, marketplace browsing, trust prompts, marketplace management, and interactive plugin management flows remain unmigrated."

// PluginCommand exposes the minimum text-only /plugin behavior before plugin management UI exists in the Go runtime.
type PluginCommand struct{}

// Metadata returns the canonical slash descriptor for /plugin.
func (c PluginCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "plugin",
		Aliases:     []string{"plugins", "marketplace"},
		Description: "Manage Claude Code plugins",
		Usage:       "/plugin [subcommand]",
	}
}

// Execute reports the stable /plugin fallback supported by the current Go host.
func (c PluginCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered plugin command fallback output", map[string]any{
		"plugin_management_available": false,
	})

	return command.Result{
		Output: pluginCommandFallback,
	}, nil
}
