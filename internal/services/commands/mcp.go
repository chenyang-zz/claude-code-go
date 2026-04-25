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
		Usage:       "/mcp [enable|disable <server-name>] | /mcp detail <server-name>",
	}
}

// Execute shows the list of connected MCP servers and their tool counts.
// It also supports a "detail" subcommand to show the full tool list for a server.
func (c MCPCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	// Handle detail subcommand.
	if len(args.Raw) >= 2 && args.Raw[0] == "detail" {
		return c.executeDetail(ctx, args.Raw[1])
	}

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
		fmt.Fprintf(&b, "  %s: %s", e.Name, status)

		if e.Status == mcpregistry.StatusConnected && e.Client != nil {
			if result, err := e.Client.ListTools(ctx); err == nil {
				if len(result.Tools) > 0 {
					fmt.Fprintf(&b, " (%d tools)", len(result.Tools))
					var names []string
					for _, t := range result.Tools {
						names = append(names, t.Name)
					}
					if len(names) > 0 {
						b.WriteString("\n    ")
						b.WriteString(strings.Join(names, ", "))
					}
				}
			}
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

// executeDetail shows the full tool list and descriptions for a specific server.
func (c MCPCommand) executeDetail(ctx context.Context, serverName string) (command.Result, error) {
	registry := mcpregistry.GetLastRegistry()
	if registry == nil {
		return command.Result{
			Output: "No MCP servers configured.",
		}, nil
	}

	var target *mcpregistry.Entry
	for _, e := range registry.List() {
		if e.Name == serverName {
			target = &e
			break
		}
	}

	if target == nil {
		return command.Result{
			Output: fmt.Sprintf("MCP server %q not found.", serverName),
		}, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "MCP Server: %s\n", target.Name)
	fmt.Fprintf(&b, "Status: %s\n", target.Status)
	if target.Error != "" {
		fmt.Fprintf(&b, "Error: %s\n", target.Error)
	}

	if target.Status == mcpregistry.StatusConnected && target.Client != nil {
		result, err := target.Client.ListTools(ctx)
		if err != nil {
			fmt.Fprintf(&b, "\nFailed to list tools: %v\n", err)
		} else if len(result.Tools) == 0 {
			b.WriteString("\nNo tools exposed by this server.\n")
		} else {
			fmt.Fprintf(&b, "\nTools (%d):\n", len(result.Tools))
			for _, t := range result.Tools {
				fmt.Fprintf(&b, "\n  %s\n", t.Name)
				if t.Description != "" {
					fmt.Fprintf(&b, "    %s\n", t.Description)
				}
				if len(t.InputSchema.Properties) > 0 {
					b.WriteString("    Parameters:\n")
					reqSet := make(map[string]bool, len(t.InputSchema.Required))
					for _, r := range t.InputSchema.Required {
						reqSet[r] = true
					}
					for name, rawProp := range t.InputSchema.Properties {
						propType := "any"
						propDesc := ""
						if m, ok := rawProp.(map[string]any); ok {
							if t, ok := m["type"].(string); ok {
								propType = t
							}
							if d, ok := m["description"].(string); ok {
								propDesc = d
							}
						}
						reqMarker := ""
						if reqSet[name] {
							reqMarker = " (required)"
						}
						fmt.Fprintf(&b, "      %s (%s)%s\n", name, propType, reqMarker)
						if propDesc != "" {
							fmt.Fprintf(&b, "        %s\n", propDesc)
						}
					}
				}
			}
		}
	} else {
		b.WriteString("\nServer is not connected; tool list unavailable.\n")
	}

	return command.Result{
		Output: strings.TrimSpace(b.String()),
	}, nil
}
