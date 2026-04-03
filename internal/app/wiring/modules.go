package wiring

import "github.com/sheepzhao/claude-code-go/internal/core/tool"

// Modules aggregates host-level runtime dependencies assembled during startup.
type Modules struct {
	// Tools is the registry exposed to executors for tool lookup and dispatch.
	Tools tool.Registry
}

// NewModules wires the provided tools into the default in-memory registry.
func NewModules(tools ...tool.Tool) (Modules, error) {
	registry := tool.NewMemoryRegistry()
	for _, item := range tools {
		if err := registry.Register(item); err != nil {
			return Modules{}, err
		}
	}

	return Modules{
		Tools: registry,
	}, nil
}
