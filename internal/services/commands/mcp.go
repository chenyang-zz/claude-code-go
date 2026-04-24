package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
)

// MCPCommand renders the minimum MCP server observability available in the Go host.
type MCPCommand struct{}

// Metadata returns the canonical slash descriptor for /mcp.
func (c MCPCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "mcp",
		Description: "Manage MCP servers",
		Usage:       "/mcp [enable|disable <server-name>]",
	}
}

// Execute shows the list of connected MCP servers and their tool counts.
func (c MCPCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	registry := mcpregistry.GetLastRegistry()
	if registry == nil {
		return command.Result{
			Output: "No MCP servers configured. Set CLAUDE_CODE_MCP_SERVERS to configure servers.",
		}, nil
	}

	entries := registry.List()
	if len(entries) == 0 {
		return command.Result{
			Output: "No MCP servers configured.",
		}, nil
	}

	var b strings.Builder
	b.WriteString("MCP Servers:\n")
	for _, e := range entries {
		status := string(e.Status)
		toolCount := 0
		if e.Status == mcpregistry.StatusConnected && e.Client != nil {
			if result, err := e.Client.ListTools(ctx); err == nil {
				toolCount = len(result.Tools)
			}
		}
		fmt.Fprintf(&b, "  %s: %s", e.Name, status)
		if toolCount > 0 {
			fmt.Fprintf(&b, " (%d tools)", toolCount)
		}
		if e.Error != "" {
			fmt.Fprintf(&b, " — %s", e.Error)
		}
		b.WriteByte('\n')
	}

	return command.Result{
		Output: strings.TrimSpace(b.String()),
	}, nil
}
