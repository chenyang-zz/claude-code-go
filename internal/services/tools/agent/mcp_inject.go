package agent

import (
	"context"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	mcpbridge "github.com/sheepzhao/claude-code-go/internal/platform/mcp/bridge"
	mcpclient "github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// agentMcpResult holds the outcome of initializing agent-specific MCP servers.
type agentMcpResult struct {
	toolDefs []model.ToolDefinition
	tools    []tool.Tool
	cleanup  func()
}

// initializeAgentMCPServers initializes MCP servers declared in the agent definition.
// It handles both name references (Config nil) and inline definitions (Config non-nil).
// Name references look up existing global entries; inline definitions create new
// dynamic connections that are cleaned up when the agent finishes.
func (r *Runner) initializeAgentMCPServers(ctx context.Context, def agent.Definition) (agentMcpResult, error) {
	if len(def.MCPServers) == 0 {
		return agentMcpResult{cleanup: func() {}}, nil
	}

	var newlyCreated []string // names of inline servers to clean up
	var allTools []tool.Tool
	var allToolDefs []model.ToolDefinition

	for _, spec := range def.MCPServers {
		var entry mcpregistry.Entry
		var found bool

		if spec.Config == nil {
			// Name reference: look up in the global registry.
			if r.ServerRegistry == nil {
				logger.WarnCF("agent.runner", "agent MCP server reference but no registry configured", map[string]any{
					"server": spec.Name,
				})
				continue
			}
			entry, found = r.ServerRegistry.GetEntry(spec.Name)
			if !found {
				logger.WarnCF("agent.runner", "agent MCP server not found in registry", map[string]any{
					"server":     spec.Name,
					"agent_type": def.AgentType,
				})
				continue
			}
		} else {
			// Inline definition: create a new dynamic connection.
			if r.ServerRegistry == nil {
				logger.WarnCF("agent.runner", "agent inline MCP server but no registry configured", map[string]any{
					"server": spec.Name,
				})
				continue
			}
			config, err := mapAgentMCPServerSpecToConfig(spec)
			if err != nil {
				logger.WarnCF("agent.runner", "failed to map agent MCP server config", map[string]any{
					"server": spec.Name,
					"error":  err.Error(),
				})
				continue
			}
			connected, err := r.ServerRegistry.ConnectDynamicServer(ctx, spec.Name, config)
			if err != nil {
				logger.WarnCF("agent.runner", "failed to connect agent MCP server", map[string]any{
					"server": spec.Name,
					"error":  err.Error(),
				})
				continue
			}
			entry = *connected
			newlyCreated = append(newlyCreated, spec.Name)
		}

		if entry.Status != mcpregistry.StatusConnected || entry.Client == nil {
			logger.WarnCF("agent.runner", "agent MCP server not connected", map[string]any{
				"server":     entry.Name,
				"status":     entry.Status,
				"agent_type": def.AgentType,
			})
			continue
		}

		tools, toolDefs, err := fetchAgentMCPTools(ctx, entry)
		if err != nil {
			logger.WarnCF("agent.runner", "failed to fetch tools from agent MCP server", map[string]any{
				"server": entry.Name,
				"error":  err.Error(),
			})
			continue
		}

		allTools = append(allTools, tools...)
		allToolDefs = append(allToolDefs, toolDefs...)

		logger.DebugCF("agent.runner", "connected to agent MCP server", map[string]any{
			"server":     entry.Name,
			"tool_count": len(tools),
			"agent_type": def.AgentType,
		})
	}

	cleanup := func() {
		if r.ServerRegistry == nil {
			return
		}
		for _, name := range newlyCreated {
			if err := r.ServerRegistry.DisconnectServer(name); err != nil {
				logger.WarnCF("agent.runner", "failed to disconnect agent MCP server", map[string]any{
					"server": name,
					"error":  err.Error(),
				})
			}
		}
	}

	return agentMcpResult{
		toolDefs: allToolDefs,
		tools:    allTools,
		cleanup:  cleanup,
	}, nil
}

// mapAgentMCPServerSpecToConfig converts an inline AgentMCPServerSpec to a client.ServerConfig.
func mapAgentMCPServerSpecToConfig(spec agent.AgentMCPServerSpec) (mcpclient.ServerConfig, error) {
	if spec.Config == nil {
		return mcpclient.ServerConfig{}, fmt.Errorf("spec has no inline config")
	}

	var cfg mcpclient.ServerConfig

	if v, ok := spec.Config["type"].(string); ok {
		cfg.Type = v
	}
	if v, ok := spec.Config["command"].(string); ok {
		cfg.Command = v
	}
	if v, ok := spec.Config["args"].([]any); ok {
		for _, item := range v {
			if s, ok := item.(string); ok {
				cfg.Args = append(cfg.Args, s)
			}
		}
	}
	if v, ok := spec.Config["env"].(map[string]any); ok {
		cfg.Env = make(map[string]string, len(v))
		for key, val := range v {
			if s, ok := val.(string); ok {
				cfg.Env[key] = s
			}
		}
	}
	if v, ok := spec.Config["url"].(string); ok {
		cfg.URL = v
	}
	if v, ok := spec.Config["headers"].(map[string]any); ok {
		cfg.Headers = make(map[string]string, len(v))
		for key, val := range v {
			if s, ok := val.(string); ok {
				cfg.Headers[key] = s
			}
		}
	}
	if v, ok := spec.Config["headersHelper"].(string); ok {
		cfg.HeadersHelper = v
	}

	return cfg, nil
}

// fetchAgentMCPTools fetches tools from a connected MCP entry and converts them
// into both tool.Tool instances (for execution) and model.ToolDefinition (for catalog).
func fetchAgentMCPTools(ctx context.Context, entry mcpregistry.Entry) ([]tool.Tool, []model.ToolDefinition, error) {
	if entry.Client == nil {
		return nil, nil, fmt.Errorf("entry has no client")
	}

	result, err := entry.Client.ListTools(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("listTools: %w", err)
	}

	var tools []tool.Tool
	var defs []model.ToolDefinition

	for _, t := range result.Tools {
		proxy := mcpbridge.AdaptTool(entry.Name, t, entry.Client)
		tools = append(tools, proxy)
		defs = append(defs, model.ToolDefinition{
			Name:        proxy.Name(),
			Description: proxy.Description(),
			InputSchema: proxy.InputSchema().JSONSchema(),
		})
	}

	return tools, defs, nil
}

// mergeToolDefinitions merges two tool definition slices, deduplicating by name.
// Items from base take precedence over items from overlay (matching TS uniqBy behavior).
func mergeToolDefinitions(base, overlay []model.ToolDefinition) []model.ToolDefinition {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(base)+len(overlay))
	merged := make([]model.ToolDefinition, 0, len(base)+len(overlay))

	for _, t := range base {
		if _, ok := seen[t.Name]; ok {
			continue
		}
		seen[t.Name] = struct{}{}
		merged = append(merged, t)
	}

	for _, t := range overlay {
		if _, ok := seen[t.Name]; ok {
			continue
		}
		seen[t.Name] = struct{}{}
		merged = append(merged, t)
	}

	return merged
}
