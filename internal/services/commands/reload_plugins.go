package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const reloadPluginsCommandFallback = "Plugin hot-reload is not available in Claude Code Go yet. Live refresh of plugin commands, agents, hooks, and MCP/LSP servers remains unmigrated."

// ReloadPluginsCommand exposes the minimum text-only /reload-plugins behavior before plugin hot-reload exists in the Go host.
type ReloadPluginsCommand struct{}

// Metadata returns the canonical slash descriptor for /reload-plugins.
func (c ReloadPluginsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "reload-plugins",
		Description: "Activate pending plugin changes in the current session",
		Usage:       "/reload-plugins",
	}
}

// Execute accepts no arguments and reports the stable /reload-plugins fallback.
func (c ReloadPluginsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered reload-plugins command fallback output", map[string]any{
		"reload_plugins_available": false,
	})

	return command.Result{
		Output: reloadPluginsCommandFallback,
	}, nil
}
