package command

import (
	"fmt"
	"strings"
)

// Registry exposes the minimum command lookup surface used by bootstrap and the REPL.
type Registry interface {
	// Register adds one command using its canonical metadata name.
	Register(cmd Command) error
	// Get resolves one command by slash name and reports whether it exists.
	Get(name string) (Command, bool)
	// List returns all commands in registration order for stable help output.
	List() []Command
}

// InMemoryRegistry stores slash commands in process-local registration order.
type InMemoryRegistry struct {
	commands map[string]Command
	order    []string
}

// NewInMemoryRegistry builds an empty registry for bootstrap wiring and tests.
func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{
		commands: make(map[string]Command),
	}
}

// Register adds one command to the registry and rejects duplicate or empty names.
func (r *InMemoryRegistry) Register(cmd Command) error {
	if r == nil {
		return fmt.Errorf("command registry is nil")
	}
	if cmd == nil {
		return fmt.Errorf("command is nil")
	}

	name := NormalizeName(cmd.Metadata().Name)
	if name == "" {
		return fmt.Errorf("command name is empty")
	}
	if _, exists := r.commands[name]; exists {
		return fmt.Errorf("command %q already registered", name)
	}

	r.commands[name] = cmd
	r.order = append(r.order, name)
	return nil
}

// Get resolves one registered command by normalized name.
func (r *InMemoryRegistry) Get(name string) (Command, bool) {
	if r == nil {
		return nil, false
	}
	cmd, ok := r.commands[NormalizeName(name)]
	return cmd, ok
}

// List returns a stable registration-ordered snapshot of registered commands.
func (r *InMemoryRegistry) List() []Command {
	if r == nil {
		return nil
	}

	list := make([]Command, 0, len(r.order))
	for _, name := range r.order {
		list = append(list, r.commands[name])
	}
	return list
}

// NormalizeName trims user input into the canonical slash command lookup key.
func NormalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "/")))
}
