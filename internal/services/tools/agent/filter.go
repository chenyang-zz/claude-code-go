package agent

import (
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// allAgentDisallowedTools contains tools that all agents (both built-in and
// custom) are not allowed to use by default. This aligns with the TypeScript
// ALL_AGENT_DISALLOWED_TOOLS constant.
var allAgentDisallowedTools = map[string]struct{}{
	"Agent":    {}, // Prevent recursive agent spawning
	"TaskStop": {}, // Requires access to main thread task state
}

// customAgentDisallowedTools contains additional tools that only custom agents
// (non-built-in, non-plugin) are not allowed to use. Currently identical to
// allAgentDisallowedTools but kept separate for future expansion.
var customAgentDisallowedTools = map[string]struct{}{
	"Agent":    {},
	"TaskStop": {},
}

// ResolvedTools holds the result of resolving an agent's tool configuration
// against the available tool catalog.
type ResolvedTools struct {
	// HasWildcard is true when the agent's allowlist is nil or ["*"].
	HasWildcard bool
	// ValidSpecs contains the tool specs that were successfully resolved.
	ValidSpecs []string
	// InvalidSpecs contains the tool specs that could not be found in the catalog.
	InvalidSpecs []string
	// Tools is the filtered and deduplicated list of tool definitions.
	Tools []model.ToolDefinition
}

// filterToolsForAgent applies the default agent filtering rules:
//   - MCP tools (names starting with "mcp__") are always allowed.
//   - Tools in allAgentDisallowedTools are removed for all agents.
//   - Tools in customAgentDisallowedTools are removed for non-built-in agents.
func filterToolsForAgent(tools []model.ToolDefinition, isBuiltIn bool) []model.ToolDefinition {
	filtered := make([]model.ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		// MCP tools are always allowed for all agents.
		// Go-side MCP tools use "serverName__toolName" naming.
		if strings.Contains(tool.Name, "__") {
			filtered = append(filtered, tool)
			continue
		}
		if _, disallowed := allAgentDisallowedTools[tool.Name]; disallowed {
			continue
		}
		if !isBuiltIn {
			if _, disallowed := customAgentDisallowedTools[tool.Name]; disallowed {
				continue
			}
		}
		filtered = append(filtered, tool)
	}
	return filtered
}

// resolveAgentTools resolves an agent definition's tool configuration against
// the available tool catalog. It handles allowlists, denylists, and wildcard.
func resolveAgentTools(def agent.Definition, availableTools []model.ToolDefinition) ResolvedTools {
	// Step 1: Apply default filtering rules (built-in vs custom, disallowed sets).
	filteredAvailable := filterToolsForAgent(availableTools, def.IsBuiltIn())

	// Separate MCP tools — they always pass through all restrictions.
	// Go-side MCP tools use "serverName__toolName" naming.
	var mcpTools []model.ToolDefinition
	var nonMcpTools []model.ToolDefinition
	for _, tool := range filteredAvailable {
		if strings.Contains(tool.Name, "__") {
			mcpTools = append(mcpTools, tool)
		} else {
			nonMcpTools = append(nonMcpTools, tool)
		}
	}

	// Step 2: Apply the agent's explicit disallowedTools to non-MCP tools only.
	disallowedSet := make(map[string]struct{}, len(def.DisallowedTools))
	for _, name := range def.DisallowedTools {
		disallowedSet[name] = struct{}{}
	}

	allowedAvailable := make([]model.ToolDefinition, 0, len(nonMcpTools))
	for _, tool := range nonMcpTools {
		if _, denied := disallowedSet[tool.Name]; !denied {
			allowedAvailable = append(allowedAvailable, tool)
		}
	}

	// Step 3: Handle allowlist (Tools field).
	// If nil or ["*"], allow all non-MCP tools after deny filtering + all MCP tools.
	hasWildcard := def.Tools == nil || (len(def.Tools) == 1 && def.Tools[0] == "*")
	if hasWildcard {
		resolved := make([]model.ToolDefinition, 0, len(allowedAvailable)+len(mcpTools))
		resolved = append(resolved, allowedAvailable...)
		resolved = append(resolved, mcpTools...)
		logger.DebugCF("agent.filter", "resolved tools with wildcard", map[string]any{
			"agent_type": def.AgentType,
			"tool_count": len(resolved),
		})
		return ResolvedTools{
			HasWildcard:  true,
			ValidSpecs:   []string{},
			InvalidSpecs: []string{},
			Tools:        resolved,
		}
	}

	// Build lookup map for explicit allowlist (non-MCP tools only).
	availableToolMap := make(map[string]model.ToolDefinition, len(allowedAvailable))
	for _, tool := range allowedAvailable {
		availableToolMap[tool.Name] = tool
	}

	var validTools []string
	var invalidTools []string
	var resolved []model.ToolDefinition
	resolvedSet := make(map[string]struct{})

	for _, toolSpec := range def.Tools {
		toolName := parseToolSpecName(toolSpec)
		if tool, ok := availableToolMap[toolName]; ok {
			validTools = append(validTools, toolSpec)
			if _, alreadyAdded := resolvedSet[toolName]; !alreadyAdded {
				resolved = append(resolved, tool)
				resolvedSet[toolName] = struct{}{}
			}
		} else {
			invalidTools = append(invalidTools, toolSpec)
		}
	}

	// Always append MCP tools — they bypass the allowlist.
	for _, tool := range mcpTools {
		if _, alreadyAdded := resolvedSet[tool.Name]; !alreadyAdded {
			resolved = append(resolved, tool)
			resolvedSet[tool.Name] = struct{}{}
		}
	}

	logger.DebugCF("agent.filter", "resolved tools with allowlist", map[string]any{
		"agent_type": def.AgentType,
		"valid":      len(validTools),
		"invalid":    len(invalidTools),
		"resolved":   len(resolved),
	})

	return ResolvedTools{
		HasWildcard:  false,
		ValidSpecs:   validTools,
		InvalidSpecs: invalidTools,
		Tools:        resolved,
	}
}

// parseToolSpecName extracts the tool name from a tool specification string.
// Tool specs may include permission patterns like "Agent(worker, researcher)"
// or "Agent:Explore,Plan". For the minimal implementation, we extract just
// the tool name (the part before any unescaped '(' or ':').
func parseToolSpecName(spec string) string {
	spec = strings.TrimSpace(spec)
	for i := 0; i < len(spec); i++ {
		if spec[i] == '(' || spec[i] == ':' {
			// Count preceding backslashes to detect escaped parens.
			backslashCount := 0
			for j := i - 1; j >= 0 && spec[j] == '\\'; j-- {
				backslashCount++
			}
			if backslashCount%2 == 0 {
				return strings.TrimSpace(spec[:i])
			}
		}
	}
	return spec
}

// formatToolList formats the agent's tool configuration into a human-readable
// description for system prompts.
func formatToolList(def agent.Definition) string {
	hasWildcard := def.Tools == nil || (len(def.Tools) == 1 && def.Tools[0] == "*")

	if hasWildcard {
		if len(def.DisallowedTools) == 0 {
			return "All tools"
		}
		return "All tools except " + strings.Join(def.DisallowedTools, ", ")
	}

	if len(def.Tools) == 0 {
		return "None"
	}

	if len(def.DisallowedTools) == 0 {
		return strings.Join(def.Tools, ", ")
	}

	// Both allowlist and denylist: filter denylist from allowlist.
	denied := make(map[string]struct{}, len(def.DisallowedTools))
	for _, d := range def.DisallowedTools {
		denied[d] = struct{}{}
	}
	var filtered []string
	for _, t := range def.Tools {
		if _, ok := denied[t]; !ok {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) == 0 {
		return "None"
	}
	return strings.Join(filtered, ", ")
}
