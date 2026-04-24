package agent

import (
	"fmt"
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// InMemoryRegistry is a thread-safe in-memory implementation of the Registry interface.
// It stores agent definitions in registration order.
type InMemoryRegistry struct {
	defs  map[string]Definition
	order []string
	mu    sync.RWMutex
}

// NewInMemoryRegistry creates an empty InMemoryRegistry ready for bootstrap wiring and tests.
func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{
		defs:  make(map[string]Definition),
		order: make([]string, 0),
	}
}

// Register adds a definition to the registry.
// Returns an error if a definition with the same AgentType already exists.
func (r *InMemoryRegistry) Register(def Definition) error {
	if r == nil {
		return fmt.Errorf("agent registry is nil")
	}

	if def.AgentType == "" {
		return fmt.Errorf("agent type is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.defs[def.AgentType]; exists {
		logger.WarnCF("agent.registry", "agent type %q already registered", map[string]any{"agentType": def.AgentType})
		return fmt.Errorf("agent type %q already registered", def.AgentType)
	}

	r.defs[def.AgentType] = def
	r.order = append(r.order, def.AgentType)
	logger.InfoCF("agent.registry", "registered agent", map[string]any{"agentType": def.AgentType, "source": def.Source})
	return nil
}

// Get retrieves a definition by its agent type identifier.
// The second return value reports whether the agent type was found.
func (r *InMemoryRegistry) Get(agentType string) (Definition, bool) {
	if r == nil {
		return Definition{}, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	def, ok := r.defs[agentType]
	return def, ok
}

// List returns all registered definitions in registration order.
func (r *InMemoryRegistry) List() []Definition {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Definition, 0, len(r.order))
	for _, agentType := range r.order {
		list = append(list, r.defs[agentType])
	}
	return list
}

// Remove deletes the definition associated with the given agent type.
// Returns true if a definition was removed.
func (r *InMemoryRegistry) Remove(agentType string) bool {
	if r == nil {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.defs[agentType]; !exists {
		return false
	}

	delete(r.defs, agentType)

	// Rebuild order slice to maintain consistency
	newOrder := make([]string, 0, len(r.order)-1)
	for _, t := range r.order {
		if t != agentType {
			newOrder = append(newOrder, t)
		}
	}
	r.order = newOrder

	logger.InfoCF("agent.registry", "removed agent", map[string]any{"agentType": agentType})
	return true
}
