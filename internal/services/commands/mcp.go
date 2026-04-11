package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// MCPCommand renders the minimum text-only /mcp behavior available before MCP management is migrated.
type MCPCommand struct{}

// Metadata returns the canonical slash descriptor for /mcp.
func (c MCPCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "mcp",
		Description: "Manage MCP servers",
		Usage:       "/mcp [enable|disable <server-name>]",
	}
}

// Execute reports the stable fallback until MCP server management commands exist in the Go host.
func (c MCPCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered mcp command fallback output", map[string]any{
		"management_available": false,
	})

	return command.Result{
		Output: "MCP server management is not available in Claude Code Go yet. Configure MCP servers before startup instead.",
	}, nil
}
