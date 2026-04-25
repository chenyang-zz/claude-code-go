package prompts

import (
	"context"
	"fmt"
	"strings"

	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
)

// MCPInstructionsSection surfaces instructions provided by connected MCP servers.
type MCPInstructionsSection struct{}

// Name returns the section identifier.
func (s MCPInstructionsSection) Name() string { return "mcp_instructions" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s MCPInstructionsSection) IsVolatile() bool { return false }

// Compute generates the MCP server instructions section from the latest registry snapshot.
func (s MCPInstructionsSection) Compute(ctx context.Context) (string, error) {
	registry := mcpregistry.GetLastRegistry()
	if registry == nil {
		return "", nil
	}

	entries := registry.List()
	blocks := make([]string, 0, len(entries))
	for _, entry := range entries {
		instructions := strings.TrimSpace(entry.Instructions)
		if instructions == "" {
			continue
		}
		blocks = append(blocks, fmt.Sprintf("## %s\n%s", entry.Name, instructions))
	}
	if len(blocks) == 0 {
		return "", nil
	}

	return "# MCP Server Instructions\n\nThe following MCP servers have provided instructions for how to use their tools and resources:\n\n" + strings.Join(blocks, "\n\n"), nil
}
