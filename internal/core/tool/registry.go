package tool

// Registry stores tool instances and resolves them by registered name.
type Registry interface {
	// Register adds a tool instance to the registry.
	Register(tool Tool) error
	// Get looks up a tool by its registered name.
	Get(name string) (Tool, bool)
	// List returns all registered tools in registration order.
	List() []Tool
}
