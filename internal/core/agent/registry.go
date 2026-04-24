package agent

// Registry provides lookup and lifecycle management for agent definitions.
type Registry interface {
	// Register adds a definition to the registry.
	// Returns an error if a definition with the same AgentType already exists.
	Register(def Definition) error
	// Get retrieves a definition by its agent type identifier.
	// The second return value reports whether the agent type was found.
	Get(agentType string) (Definition, bool)
	// List returns all registered definitions in an undefined order.
	List() []Definition
	// Remove deletes the definition associated with the given agent type.
	// Returns true if a definition was removed.
	Remove(agentType string) bool
}
