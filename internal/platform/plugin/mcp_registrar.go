package plugin

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// McpRegistrar converts plugin McpServerConfig values into core
// client.ServerConfig values and loads them into an MCP ServerRegistry.
type McpRegistrar struct {
	registry *mcpregistry.ServerRegistry
}

// NewMcpRegistrar creates an MCP registrar backed by the given registry.
func NewMcpRegistrar(registry *mcpregistry.ServerRegistry) *McpRegistrar {
	return &McpRegistrar{registry: registry}
}

// RegisterMcpServers converts and loads every MCP server configuration from
// the given slice.  Duplicate server names are handled by the underlying
// registry's LoadConfigs semantics (appends entries; the registry does not
// deduplicate, matching the TS side where later sources override earlier
// ones via the full refresh cycle).  After loading, ConnectAll is called to
// initiate connections for the newly loaded servers.
func (r *McpRegistrar) RegisterMcpServers(servers []*McpServerConfig) (registered int, errs []*PluginError) {
	if r == nil || r.registry == nil {
		return 0, []*PluginError{{
			Type:    "registration-error",
			Source:  "mcp-registrar",
			Message: "MCP registrar or registry is nil",
		}}
	}

	configs := make(map[string]client.ServerConfig, len(servers))
	for _, s := range servers {
		if s == nil {
			continue
		}
		configs[s.Name] = toClientServerConfig(s)
	}

	if len(configs) == 0 {
		return 0, nil
	}

	r.registry.LoadConfigs(configs)

	registered = len(configs)
	for name, cfg := range configs {
		logger.InfoCF("plugin.mcp_registrar", "loaded MCP server config", map[string]any{
			"name":      name,
			"transport": cfg.Type,
		})
	}

	// Initiate connections for all loaded servers.  Errors during
	// connection are recorded in the registry entries, not returned here.
	ctx := context.Background()
	r.registry.ConnectAll(ctx)

	return registered, nil
}

// toClientServerConfig maps a plugin McpServerConfig to the core
// client.ServerConfig used by the MCP runtime.
func toClientServerConfig(s *McpServerConfig) client.ServerConfig {
	return client.ServerConfig{
		Type:    s.Transport,
		Command: s.Command,
		Args:    s.Args,
		Env:     s.Env,
		URL:     s.URL,
		Headers: s.Headers,
	}
}
