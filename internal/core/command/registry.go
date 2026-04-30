package command

import (
	"fmt"
	"strings"
)

// Registry exposes the minimum command lookup surface used by bootstrap and the REPL.
type Registry interface {
	// Register adds one command using its canonical metadata name.
	Register(cmd Command) error
	// Unregister removes a command and all its aliases from the registry.
	Unregister(name string) error
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

// Register adds one command to the registry and rejects duplicate or empty names and aliases.
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

	identifiers := append([]string{name}, cmd.Metadata().Aliases...)
	seen := make(map[string]struct{}, len(identifiers))
	for _, identifier := range identifiers {
		normalized := NormalizeName(identifier)
		if normalized == "" {
			return fmt.Errorf("command alias is empty")
		}
		if _, exists := seen[normalized]; exists {
			return fmt.Errorf("command %q duplicates its own alias", normalized)
		}
		if _, exists := r.commands[normalized]; exists {
			return fmt.Errorf("command %q already registered", normalized)
		}
		seen[normalized] = struct{}{}
	}

	for identifier := range seen {
		r.commands[identifier] = cmd
	}
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

// Unregister removes a command and all its aliases from the registry.
func (r *InMemoryRegistry) Unregister(name string) error {
	if r == nil {
		return fmt.Errorf("command registry is nil")
	}
	normalized := NormalizeName(name)
	if normalized == "" {
		return fmt.Errorf("command name is empty")
	}
	cmd, ok := r.commands[normalized]
	if !ok {
		return fmt.Errorf("command %q not found", normalized)
	}

	meta := cmd.Metadata()
	identifiers := append([]string{meta.Name}, meta.Aliases...)
	for _, identifier := range identifiers {
		delete(r.commands, NormalizeName(identifier))
	}

	for i, n := range r.order {
		if NormalizeName(n) == NormalizeName(meta.Name) {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return nil
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
