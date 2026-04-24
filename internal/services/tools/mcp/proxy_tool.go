package mcp

// Re-export the bridge adapter so the services layer can register MCP tools.
import (
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/platform/mcp/bridge"
	"github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
)

// AdaptTool converts an MCP tool declaration into a core tool.Tool wrapper.
// It is re-exported from the bridge layer so services can register MCP tools
// without depending on platform/mcp/bridge directly.
func AdaptTool(serverName string, mcpTool client.Tool, mcpClient *client.Client) tool.Tool {
	return bridge.AdaptTool(serverName, mcpTool, mcpClient)
}
