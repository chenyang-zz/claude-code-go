package plugin

import (
	"fmt"
	"sync"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// UnregisterAll removes every plugin-originated capability from the runtime
// subsystems, restoring the base hook configuration.  It is the inverse of
// RegisterAll and is designed to be called before a full refresh reload.
//
// Errors during individual unregistrations are logged but do not block
// continued cleanup of other subsystems.
func (r *PluginRegistrar) UnregisterAll(baseHooks hook.HooksConfig) {
	if r == nil {
		return
	}

	// 1. Unregister plugin commands.
	if r.CommandRegistry != nil {
		for _, cmd := range r.CommandRegistry.List() {
			if _, ok := cmd.(*CommandAdapter); ok {
				name := cmd.Metadata().Name
				if err := r.CommandRegistry.Unregister(name); err != nil {
					logger.WarnCF("plugin.registrar", "failed to unregister command", map[string]any{
						"command": name,
						"error":   err.Error(),
					})
				} else {
					logger.DebugCF("plugin.registrar", "unregistered command", map[string]any{
						"command": name,
					})
				}
			}
		}
	}

	// 2. Unregister plugin agents.
	if r.AgentRegistry != nil {
		if reg, ok := r.AgentRegistry.(interface{ List() []agent.Definition }); ok {
			for _, def := range reg.List() {
				if def.Source == "plugin" {
					if removed := r.AgentRegistry.Remove(def.AgentType); removed {
						logger.DebugCF("plugin.registrar", "unregistered agent", map[string]any{
							"agent_type": def.AgentType,
						})
					}
				}
			}
		}
	}

	// 3. Clear MCP servers.
	if r.McpRegistry != nil {
		if err := r.McpRegistry.Clear(); err != nil {
			logger.WarnCF("plugin.registrar", "failed to clear MCP registry", map[string]any{
				"error": err.Error(),
			})
		} else {
			logger.DebugCF("plugin.registrar", "cleared MCP registry", nil)
		}
	}

	// 4. Stop all LSP servers.
	if r.LspManager != nil {
		if err := r.LspManager.StopAllServers(); err != nil {
			logger.WarnCF("plugin.registrar", "failed to stop LSP servers", map[string]any{
				"error": err.Error(),
			})
		} else {
			logger.DebugCF("plugin.registrar", "stopped all LSP servers", nil)
		}
	}

	// 5. Restore base hooks.
	if r.HooksConfig != nil {
		*r.HooksConfig = baseHooks
		logger.DebugCF("plugin.registrar", "restored base hooks", nil)
	}

	logger.InfoCF("plugin.registrar", "plugin unregistration complete", nil)
}

// Reloader coordinates the full unload-reload-register cycle for plugin
// hot-reloading.  It is safe for concurrent use: only one reload runs at a
// time; additional change notifications are coalesced into a single follow-up
// reload.
type Reloader struct {
	loader    *PluginLoader
	registrar *PluginRegistrar
	baseHooks hook.HooksConfig
	mu        sync.Mutex
	reloading bool
	pending   bool
}

// NewReloader creates a Reloader backed by the given loader, registrar, and
// base hook configuration.
func NewReloader(loader *PluginLoader, registrar *PluginRegistrar, baseHooks hook.HooksConfig) *Reloader {
	return &Reloader{
		loader:    loader,
		registrar: registrar,
		baseHooks: baseHooks,
	}
}

// Reload executes the full unregister → refresh → register pipeline.
// It returns a summary of the reload outcome.
//
// Reload is serialised: if another reload is already in progress, this call
// sets a pending flag and returns immediately.  When the in-progress reload
// finishes, the pending reload is executed automatically.
func (rl *Reloader) Reload() (*RegistrationSummary, error) {
	rl.mu.Lock()
	if rl.reloading {
		rl.pending = true
		rl.mu.Unlock()
		logger.DebugCF("plugin.reloader", "reload already in progress, marking pending", nil)
		return nil, fmt.Errorf("reload already in progress")
	}
	rl.reloading = true
	rl.pending = false
	rl.mu.Unlock()

	logger.InfoCF("plugin.reloader", "starting plugin reload", nil)

	// 1. Unregister all plugin capabilities.
	if rl.registrar != nil {
		rl.registrar.UnregisterAll(rl.baseHooks)
	}

	// 2. Refresh active plugins.
	var result *RefreshResult
	var refreshErr error
	if rl.loader != nil {
		result, refreshErr = rl.loader.RefreshActivePlugins()
		if refreshErr != nil {
			logger.WarnCF("plugin.reloader", "refresh failed", map[string]any{
				"error": refreshErr.Error(),
			})
		}
	}

	// 3. Register all refreshed capabilities.
	var summary *RegistrationSummary
	var regErr error
	if rl.registrar != nil && result != nil {
		summary, regErr = rl.registrar.RegisterAll(result, rl.baseHooks)
		if regErr != nil {
			logger.WarnCF("plugin.reloader", "registration failed", map[string]any{
				"error": regErr.Error(),
			})
		}
	}

	// 4. Release lock and process pending reload if any.
	rl.mu.Lock()
	rl.reloading = false
	hasPending := rl.pending
	rl.pending = false
	rl.mu.Unlock()

	if hasPending {
		logger.DebugCF("plugin.reloader", "processing pending reload", nil)
		_, _ = rl.Reload()
	}

	logger.InfoCF("plugin.reloader", "plugin reload complete", nil)

	// Return the primary error if any.
	if refreshErr != nil {
		return summary, refreshErr
	}
	if regErr != nil {
		return summary, regErr
	}
	return summary, nil
}

// IsReloading reports whether a reload is currently in progress.
func (rl *Reloader) IsReloading() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.reloading
}
