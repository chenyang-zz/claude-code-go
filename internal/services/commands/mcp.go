package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	mcpclient "github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
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

		if e.Status == mcpregistry.StatusConnected {
			sections := make([]string, 0, 3)
			if tools, err := toolsForEntry(ctx, e); err == nil && len(tools) > 0 {
				sections = append(sections, fmt.Sprintf("%d tools", len(tools)))
			}
			if e.Capabilities.Resources != nil {
				if resources, err := resourcesForEntry(ctx, e); err == nil && len(resources) > 0 {
					sections = append(sections, fmt.Sprintf("%d resources", len(resources)))
				}
			}
			if e.Capabilities.Prompts != nil {
				if prompts, err := promptsForEntry(ctx, e); err == nil && len(prompts) > 0 {
					sections = append(sections, fmt.Sprintf("%d prompts", len(prompts)))
				}
			}
			if len(sections) > 0 {
				fmt.Fprintf(&b, " (%s)", strings.Join(sections, ", "))
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
	if target.Config.OAuth != nil {
		b.WriteString("OAuth: configured\n")
		if target.Config.OAuth.XAA != nil && *target.Config.OAuth.XAA {
			b.WriteString("XAA: enabled\n")
		}
	}

	if target.Status == mcpregistry.StatusConnected {
		if tools, err := toolsForEntry(ctx, *target); err != nil {
			fmt.Fprintf(&b, "\nFailed to list tools: %v\n", err)
		} else if len(tools) == 0 {
			b.WriteString("\nNo tools exposed by this server.\n")
		} else {
			fmt.Fprintf(&b, "\nTools (%d):\n", len(tools))
			for _, t := range tools {
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
		if target.Capabilities.Resources != nil {
			if resources, err := resourcesForEntry(ctx, *target); err == nil {
				if len(resources) == 0 {
					b.WriteString("\nNo resources exposed by this server.\n")
				} else {
					fmt.Fprintf(&b, "\nResources (%d):\n", len(resources))
					for _, r := range resources {
						fmt.Fprintf(&b, "\n  %s\n", firstNonEmpty(r.Name, r.URI))
						if r.URI != "" {
							fmt.Fprintf(&b, "    URI: %s\n", r.URI)
						}
						if r.Description != "" {
							fmt.Fprintf(&b, "    %s\n", r.Description)
						}
						if r.MimeType != "" {
							fmt.Fprintf(&b, "    MIME: %s\n", r.MimeType)
						}
					}
				}
			}
		}
		if target.Capabilities.Prompts != nil {
			if prompts, err := promptsForEntry(ctx, *target); err == nil {
				if len(prompts) == 0 {
					b.WriteString("\nNo prompts exposed by this server.\n")
				} else {
					fmt.Fprintf(&b, "\nPrompts (%d):\n", len(prompts))
					for _, p := range prompts {
						fmt.Fprintf(&b, "\n  %s\n", firstNonEmpty(p.Title, p.Name))
						if p.Name != "" {
							fmt.Fprintf(&b, "    Name: %s\n", p.Name)
						}
						if p.Description != "" {
							fmt.Fprintf(&b, "    %s\n", p.Description)
						}
						if len(p.Arguments) > 0 {
							b.WriteString("    Arguments:\n")
							for _, arg := range p.Arguments {
								marker := ""
								if arg.Required {
									marker = " (required)"
								}
								fmt.Fprintf(&b, "      %s%s\n", arg.Name, marker)
								if arg.Description != "" {
									fmt.Fprintf(&b, "        %s\n", arg.Description)
								}
							}
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

// toolsForEntry returns the cached tools snapshot when available, otherwise it falls back to a live fetch.
func toolsForEntry(ctx context.Context, entry mcpregistry.Entry) ([]mcpclient.Tool, error) {
	if entry.Tools != nil {
		return entry.Tools, nil
	}
	if entry.Client == nil {
		return nil, nil
	}
	result, err := entry.Client.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// resourcesForEntry returns the cached resources snapshot when available, otherwise it falls back to a live fetch.
func resourcesForEntry(ctx context.Context, entry mcpregistry.Entry) ([]mcpclient.Resource, error) {
	if entry.Resources != nil {
		return entry.Resources, nil
	}
	if entry.Client == nil {
		return nil, nil
	}
	result, err := entry.Client.ListResources(ctx)
	if err != nil {
		return nil, err
	}
	return result.Resources, nil
}

// promptsForEntry returns the cached prompts snapshot when available, otherwise it falls back to a live fetch.
func promptsForEntry(ctx context.Context, entry mcpregistry.Entry) ([]mcpclient.Prompt, error) {
	if entry.Prompts != nil {
		return entry.Prompts, nil
	}
	if entry.Client == nil {
		return nil, nil
	}
	result, err := entry.Client.ListPrompts(ctx)
	if err != nil {
		return nil, err
	}
	return result.Prompts, nil
}

// firstNonEmpty returns the first non-empty string from the provided values.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
