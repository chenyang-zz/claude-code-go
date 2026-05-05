package coordinator

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
	"github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
)

// CoordinatorUserContext holds the runtime context injected into the
// coordinator system prompt. It carries worker tool availability, MCP
// server names, and an optional scratchpad directory.
type CoordinatorUserContext struct {
	// EnabledToolNames is the full set of tool names available in the session.
	EnabledToolNames map[string]struct{}
	// MCPServerNames lists the names of connected MCP servers.
	MCPServerNames []string
	// ScratchpadDir is the path to the shared scratchpad directory, or empty.
	ScratchpadDir string
}

// BuildCoordinatorUserContext constructs a CoordinatorUserContext from the
// current runtime state. mcpRegistry may be nil if MCP is not configured.
func BuildCoordinatorUserContext(
	enabledToolNames map[string]struct{},
	mcpRegistry *registry.ServerRegistry,
	scratchpadDir string,
) CoordinatorUserContext {
	ctx := CoordinatorUserContext{
		EnabledToolNames: enabledToolNames,
	}

	if mcpRegistry != nil {
		connected := mcpRegistry.Connected()
		names := make([]string, 0, len(connected))
		for _, entry := range connected {
			if entry.Name != "" {
				names = append(names, entry.Name)
			}
		}
		sort.Strings(names)
		ctx.MCPServerNames = names
	}

	if featureflag.IsEnabled(featureflag.FlagCoordinatorScratchpad) && scratchpadDir != "" {
		ctx.ScratchpadDir = scratchpadDir
	}

	return ctx
}

// RenderWorkerToolsContext builds the worker tools context string injected
// into the coordinator prompt. This replaces the TS getCoordinatorUserContext.
// When simpleMode is true, only Bash/FileRead/FileEdit are listed.
func RenderWorkerToolsContext(ctx CoordinatorUserContext, simpleMode bool) string {
	var toolNames []string
	if simpleMode {
		toolNames = SimpleModeTools
	} else {
		toolNames = make([]string, 0, len(AsyncAgentAllowedTools))
		for name := range AsyncAgentAllowedTools {
			if _, internal := InternalWorkerTools[name]; internal {
				continue
			}
			toolNames = append(toolNames, name)
		}
		sort.Strings(toolNames)
	}

	content := fmt.Sprintf("Workers spawned via the Agent tool have access to these tools: %s",
		strings.Join(toolNames, ", "))

	if len(ctx.MCPServerNames) > 0 {
		content += fmt.Sprintf("\n\nWorkers also have access to MCP tools from connected MCP servers: %s",
			strings.Join(ctx.MCPServerNames, ", "))
	}

	if ctx.ScratchpadDir != "" {
		content += fmt.Sprintf("\n\nScratchpad directory: %s\nWorkers can read and write here without permission prompts. Use this for durable cross-worker knowledge — structure files however fits the work.",
			ctx.ScratchpadDir)
	}

	return content
}

// IsSimpleMode reports whether CLAUDE_CODE_SIMPLE is enabled.
func IsSimpleMode() bool {
	val := strings.TrimSpace(os.Getenv("CLAUDE_CODE_SIMPLE"))
	switch strings.ToLower(val) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
