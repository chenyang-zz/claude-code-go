package tool

import (
	"fmt"
	"slices"
	"sync"
)

// MemoryRegistry keeps registered tools in memory for a single process.
type MemoryRegistry struct {
	// mu protects all registry reads and writes so registration and listing stay race-free.
	mu sync.RWMutex
	// tools maps the stable tool name to its runtime instance.
	tools map[string]Tool
	// order preserves registration order for deterministic listing.
	order []string
}

// NewMemoryRegistry creates an empty in-memory registry implementation.
func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{
		tools: make(map[string]Tool),
	}
}

// Register inserts a tool into the registry and rejects nil, empty-name, or duplicate entries.
func (r *MemoryRegistry) Register(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("tool registry: nil tool")
	}

	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tool registry: empty tool name")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Preserve name uniqueness so executor dispatch stays deterministic.
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool registry: tool %q already registered", name)
	}

	r.tools[name] = tool
	r.order = append(r.order, name)
	return nil
}

// Get returns the registered tool matching the provided name.
func (r *MemoryRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	return tool, ok
}

// List returns the registered tools in the order they were added.
func (r *MemoryRegistry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Clone the order slice so callers cannot mutate registry-owned state.
	names := slices.Clone(r.order)
	tools := make([]Tool, 0, len(names))
	for _, name := range names {
		tools = append(tools, r.tools[name])
	}
	return tools
}
