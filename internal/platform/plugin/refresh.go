package plugin

import (
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// RefreshActivePlugins performs a full plugin reload sequence:
//  1. Load all plugins from the installed store via LoadAll.
//  2. Iterate enabled plugins and extract capabilities in order: commands,
//     agents, MCP servers, LSP servers, hooks.
//  3. Return a RefreshResult with all counts and the collected command/agent
//     arrays.
//
// Per-plugin extraction failures are non-fatal — errors are collected and
// the method continues processing remaining plugins. The result is always
// returned, even when some extractions fail.
func (l *PluginLoader) RefreshActivePlugins() (*RefreshResult, error) {
	result, err := l.LoadAll()
	if err != nil {
		return nil, err
	}

	refresh := &RefreshResult{
		EnabledCount:  len(result.Enabled),
		DisabledCount: len(result.Disabled),
		ErrorCount:    len(result.Errors),
	}

	var allCommands []*PluginCommand
	var allAgents []*AgentDefinition
	mcpCount := 0
	lspCount := 0
	hookCount := 0

	var extraErrors []string

	for _, plugin := range result.Enabled {
		// Commands and skills.
		cmds, err := ExtractCommands(plugin)
		if err != nil {
			extraErrors = append(extraErrors, err.Error())
		} else if len(cmds) > 0 {
			allCommands = append(allCommands, cmds...)
		}

		skills, err := ExtractSkills(plugin)
		if err != nil {
			extraErrors = append(extraErrors, err.Error())
		} else if len(skills) > 0 {
			allCommands = append(allCommands, skills...)
		}

		// Agents.
		agents, err := ExtractAgents(plugin)
		if err != nil {
			extraErrors = append(extraErrors, err.Error())
		} else if len(agents) > 0 {
			allAgents = append(allAgents, agents...)
		}

		// MCP servers (from .mcp.json).
		mcpServers, err := ExtractMcpServers(plugin)
		if err != nil {
			extraErrors = append(extraErrors, err.Error())
		} else {
			mcpCount += len(mcpServers)
		}

		// MCPB servers (from .mcpb / .dxt files).
		mcpbServers, err := ExtractMcpbServers(plugin)
		if err != nil {
			extraErrors = append(extraErrors, err.Error())
		} else {
			mcpCount += len(mcpbServers)
		}

		// LSP servers.
		lspServers, err := ExtractLspServers(plugin)
		if err != nil {
			extraErrors = append(extraErrors, err.Error())
		} else {
			lspCount += len(lspServers)
		}

		// Hooks.
		hooks := ExtractHooks(plugin)
		for _, matchers := range hooks {
			hookCount += len(matchers)
		}
	}

	refresh.CommandCount = len(allCommands)
	refresh.AgentCount = len(allAgents)
	refresh.McpCount = mcpCount
	refresh.LspCount = lspCount
	refresh.HookCount = hookCount
	refresh.Commands = allCommands
	refresh.Agents = allAgents
	refresh.Plugins = result.Enabled

	if len(extraErrors) > 0 {
		refresh.ErrorCount += len(extraErrors)
		logger.DebugCF("plugin.refresh", "extraction errors during refresh", map[string]any{
			"count":  len(extraErrors),
			"detail": strings.Join(extraErrors, "; "),
		})
	}

	logger.DebugCF("plugin.refresh", "refresh complete", map[string]any{
		"enabled":  refresh.EnabledCount,
		"commands": refresh.CommandCount,
		"agents":   refresh.AgentCount,
		"mcp":      refresh.McpCount,
		"lsp":      refresh.LspCount,
		"hooks":    refresh.HookCount,
		"errors":   refresh.ErrorCount,
	})
	return refresh, nil
}
