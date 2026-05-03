package plugin

import (
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
	"github.com/sheepzhao/claude-code-go/internal/platform/lsp"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// RegistrationSummary captures the outcome of registering plugin capabilities
// to the Go runtime subsystems.
type RegistrationSummary struct {
	AgentsRegistered    int
	CommandsRegistered  int
	HooksEventsRegistered int
	McpServersLoaded    int
	LspServersRegistered int
	Errors              []string
}

// PluginRegistrar provides a single entry point for registering all capabilities
// extracted from plugins into the corresponding Go runtime subsystems.
type PluginRegistrar struct {
	AgentRegistry   agent.Registry
	CommandRegistry command.Registry
	HooksConfig     *hook.HooksConfig
	McpRegistry     *mcpregistry.ServerRegistry
	LspManager      *lsp.Manager
	// PluginConfigs holds per-plugin configuration blobs keyed by plugin id,
	// sourced from the merged settings. Used to resolve ${user_config.X}
	// placeholders during command execution.
	PluginConfigs map[string]config.PluginConfig
}

// NewPluginRegistrar creates a PluginRegistrar with the given subsystem references.
// Any nil reference causes registration for that subsystem to be skipped.
func NewPluginRegistrar(
	agentRegistry agent.Registry,
	commandRegistry command.Registry,
	hooksConfig *hook.HooksConfig,
	mcpRegistry *mcpregistry.ServerRegistry,
	lspManager *lsp.Manager,
) *PluginRegistrar {
	return NewPluginRegistrarWithConfigs(agentRegistry, commandRegistry, hooksConfig, mcpRegistry, lspManager, nil)
}

// NewPluginRegistrarWithConfigs creates a PluginRegistrar with the given subsystem
// references and per-plugin configuration blobs. The pluginConfigs map is keyed by
// plugin id and holds user-configurable option values used to resolve ${user_config.X}
// placeholders during command execution.
func NewPluginRegistrarWithConfigs(
	agentRegistry agent.Registry,
	commandRegistry command.Registry,
	hooksConfig *hook.HooksConfig,
	mcpRegistry *mcpregistry.ServerRegistry,
	lspManager *lsp.Manager,
	pluginConfigs map[string]config.PluginConfig,
) *PluginRegistrar {
	return &PluginRegistrar{
		AgentRegistry:   agentRegistry,
		CommandRegistry: commandRegistry,
		HooksConfig:     hooksConfig,
		McpRegistry:     mcpRegistry,
		LspManager:      lspManager,
		PluginConfigs:   pluginConfigs,
	}
}

// RegisterAll takes a RefreshResult and registers every extracted capability
// into the corresponding runtime subsystem.  It returns a summary of what
// was registered and any non-fatal errors encountered.
func (r *PluginRegistrar) RegisterAll(result *RefreshResult, baseHooks hook.HooksConfig) (*RegistrationSummary, error) {
	if r == nil {
		return nil, fmt.Errorf("plugin registrar is nil")
	}

	summary := &RegistrationSummary{}

	// 1. Register agents.
	if r.AgentRegistry != nil && len(result.Agents) > 0 {
		agentReg := NewAgentRegistrar(r.AgentRegistry)
		count, errs := agentReg.RegisterAgents(result.Agents)
		summary.AgentsRegistered = count
		for _, e := range errs {
			summary.Errors = append(summary.Errors, formatPluginError(e))
		}
	}

	// 2. Register commands.
	if r.CommandRegistry != nil && len(result.Commands) > 0 {
		for _, pc := range result.Commands {
			if pc == nil {
				continue
			}
			// Inject user config values from the merged settings so that
			// CommandAdapter can resolve ${user_config.X} placeholders.
			if r.PluginConfigs != nil {
				pc.UserConfigValues = r.PluginConfigs[pc.PluginName]
			}
			adapter := NewCommandAdapter(pc)
			if err := r.CommandRegistry.Register(adapter); err != nil {
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("command %q: %v", pc.Name, err))
				logger.WarnCF("plugin.registrar", "failed to register plugin command", map[string]any{
					"command": pc.Name,
					"error":   err.Error(),
				})
				continue
			}
			summary.CommandsRegistered++
			logger.InfoCF("plugin.registrar", "registered plugin command", map[string]any{
				"command": pc.Name,
			})
		}
	}

	// 3. Register hooks (needs original plugin list).
	if r.HooksConfig != nil && len(result.Plugins) > 0 {
		count, err := r.RegisterHooks(result.Plugins, baseHooks)
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("hooks: %v", err))
		}
		summary.HooksEventsRegistered = count
	}

	// 4. Register MCP servers (re-extract from plugins, including MCPB).
	if r.McpRegistry != nil && len(result.Plugins) > 0 {
		var allMcpServers []*McpServerConfig
		for _, p := range result.Plugins {
			servers, err := ExtractMcpServers(p)
			if err != nil {
				summary.Errors = append(summary.Errors, fmt.Sprintf("mcp extract %q: %v", p.Name, err))
				continue
			}
			allMcpServers = append(allMcpServers, servers...)

			// Also extract MCPB servers (.mcpb / .dxt files).
			mcpbServers, err := ExtractMcpbServers(p)
			if err != nil {
				summary.Errors = append(summary.Errors, fmt.Sprintf("mcpb extract %q: %v", p.Name, err))
				continue
			}
			allMcpServers = append(allMcpServers, mcpbServers...)
		}
		if len(allMcpServers) > 0 {
			count, err := r.RegisterMcpServers(allMcpServers)
			if err != nil {
				summary.Errors = append(summary.Errors, fmt.Sprintf("mcp register: %v", err))
			}
			summary.McpServersLoaded = count
		}
	}

	// 5. Register LSP servers (re-extract from plugins).
	if r.LspManager != nil && len(result.Plugins) > 0 {
		var allLspServers []*LspServerConfig
		for _, p := range result.Plugins {
			servers, err := ExtractLspServers(p)
			if err != nil {
				summary.Errors = append(summary.Errors, fmt.Sprintf("lsp extract %q: %v", p.Name, err))
				continue
			}
			allLspServers = append(allLspServers, servers...)
		}
		if len(allLspServers) > 0 {
			count, err := r.RegisterLspServers(allLspServers)
			if err != nil {
				summary.Errors = append(summary.Errors, fmt.Sprintf("lsp register: %v", err))
			}
			summary.LspServersRegistered = count
		}
	}

	logger.InfoCF("plugin.registrar", "plugin registration complete", map[string]any{
		"agents":   summary.AgentsRegistered,
		"commands": summary.CommandsRegistered,
		"hooks":    summary.HooksEventsRegistered,
		"mcp":      summary.McpServersLoaded,
		"lsp":      summary.LspServersRegistered,
		"errors":   len(summary.Errors),
	})

	return summary, nil
}

// RegisterHooks is a convenience wrapper that registers hooks from the given
// enabled plugins, merging them with the provided base configuration.  The
// result is written to r.HooksConfig if it is non-nil.
func (r *PluginRegistrar) RegisterHooks(plugins []*LoadedPlugin, base hook.HooksConfig) (int, error) {
	if r == nil || r.HooksConfig == nil {
		return 0, nil
	}
	registrar := NewHooksRegistrar()
	merged, errs := registrar.RegisterHooks(plugins, base)
	if len(errs) > 0 {
		for _, e := range errs {
			logger.WarnCF("plugin.registrar", "hook registration error", map[string]any{
				"error": formatPluginError(e),
			})
		}
	}
	*r.HooksConfig = merged
	eventCount := len(merged)
	return eventCount, nil
}

// RegisterMcpServers loads MCP server configurations into the MCP registry.
func (r *PluginRegistrar) RegisterMcpServers(servers []*McpServerConfig) (int, error) {
	if r == nil || r.McpRegistry == nil {
		return 0, nil
	}
	registrar := NewMcpRegistrar(r.McpRegistry)
	count, errs := registrar.RegisterMcpServers(servers)
	if len(errs) > 0 {
		for _, e := range errs {
			logger.WarnCF("plugin.registrar", "MCP registration error", map[string]any{
				"error": formatPluginError(e),
			})
		}
	}
	return count, nil
}

// RegisterLspServers registers LSP server configurations with the LSP manager.
func (r *PluginRegistrar) RegisterLspServers(servers []*LspServerConfig) (int, error) {
	if r == nil || r.LspManager == nil {
		return 0, nil
	}
	registrar := NewLspRegistrar(r.LspManager)
	count, errs := registrar.RegisterLspServers(servers)
	if len(errs) > 0 {
		for _, e := range errs {
			logger.WarnCF("plugin.registrar", "LSP registration error", map[string]any{
				"error": formatPluginError(e),
			})
		}
	}
	return count, nil
}

func formatPluginError(e *PluginError) string {
	if e.Plugin != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Type, e.Plugin, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// FormatSummary returns a human-readable summary of the registration result.
func (s *RegistrationSummary) FormatSummary() string {
	if s == nil {
		return "No plugin registration performed."
	}
	var parts []string
	parts = append(parts, fmt.Sprintf("Registered %d agent(s), %d command(s).",
		s.AgentsRegistered, s.CommandsRegistered))
	if s.McpServersLoaded > 0 {
		parts = append(parts, fmt.Sprintf("Loaded %d MCP server(s).", s.McpServersLoaded))
	}
	if s.LspServersRegistered > 0 {
		parts = append(parts, fmt.Sprintf("Registered %d LSP server(s).", s.LspServersRegistered))
	}
	if len(s.Errors) > 0 {
		parts = append(parts, fmt.Sprintf("Encountered %d error(s):", len(s.Errors)))
		for _, e := range s.Errors {
			parts = append(parts, "  - "+e)
		}
	}
	return strings.Join(parts, "\n")
}
