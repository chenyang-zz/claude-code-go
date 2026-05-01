package prompts

import "context"

// MCPResourcePromptSection provides usage guidance for the MCP resource tools.
type MCPResourcePromptSection struct{}

// Name returns the section identifier.
func (s MCPResourcePromptSection) Name() string { return "mcp_resource_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s MCPResourcePromptSection) IsVolatile() bool { return false }

// Compute generates the MCP resource tools usage guidance.
func (s MCPResourcePromptSection) Compute(ctx context.Context) (string, error) {
	return `# MCP Resource Tools

## ListMcpResources

List available resources from configured MCP servers.
Each returned resource will include all standard MCP resource fields plus a 'server' field indicating which server the resource belongs to.

Parameters:
- server (optional): The name of a specific MCP server to get resources from. If not provided, resources from all servers will be returned.

## ReadMcpResource

Reads a specific resource from an MCP server, identified by server name and resource URI.

Parameters:
- server (required): The name of the MCP server from which to read the resource
- uri (required): The URI of the resource to read`, nil
}
