package plugin

import (
	"fmt"
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Registry is a thread-safe container for loaded plugins. It provides
// registration, lookup, enable/disable, and removal operations. Plugins are
// indexed by name; duplicate registration of the same name returns an error.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]*LoadedPlugin
}

// NewRegistry creates an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]*LoadedPlugin),
	}
}

// Register adds a plugin to the registry. If a plugin with the same name is
// already registered, an error is returned and the existing entry is unchanged.
func (r *Registry) Register(p *LoadedPlugin) error {
	if p == nil {
		return fmt.Errorf("cannot register nil plugin")
	}
	if p.Name == "" {
		return fmt.Errorf("cannot register plugin with empty name")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[p.Name]; exists {
		return fmt.Errorf("plugin %q is already registered", p.Name)
	}

	r.plugins[p.Name] = p
	logger.DebugCF("plugin.registry", "plugin registered", map[string]any{
		"name":    p.Name,
		"enabled": p.Enabled,
	})
	return nil
}

// Unregister removes a plugin from the registry by name. It is a no-op if the
// plugin is not registered.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[name]; exists {
		delete(r.plugins, name)
		logger.DebugCF("plugin.registry", "plugin unregistered", map[string]any{
			"name": name,
		})
	}
}

// Get returns the plugin with the given name, or nil if not found.
func (r *Registry) Get(name string) *LoadedPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plugins[name]
}

// List returns a snapshot of all registered plugins. The returned slice has no
// guaranteed order.
func (r *Registry) List() []*LoadedPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*LoadedPlugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		result = append(result, p)
	}
	return result
}

// SetEnabled sets the enabled state of a registered plugin. It is a no-op if
// the plugin is not found.
func (r *Registry) SetEnabled(name string, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p, exists := r.plugins[name]; exists {
		p.Enabled = enabled
		logger.DebugCF("plugin.registry", "plugin enabled state changed", map[string]any{
			"name":    name,
			"enabled": enabled,
		})
	}
}

// Count returns the total number of registered plugins.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.plugins)
}

// Clear removes all plugins from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins = make(map[string]*LoadedPlugin)
	logger.DebugCF("plugin.registry", "registry cleared", nil)
}
