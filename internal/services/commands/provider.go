package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// ProviderStatus carries diagnostic information about one provider in the pool.
type ProviderStatus struct {
	Name                string
	Position            int
	CircuitBreakerState string
	FailureCount        int
	TripCount           int
}

// ProviderSwitcher provides runtime provider inspection and switching for the /provider command.
type ProviderSwitcher interface {
	// Strategy returns the load-balance strategy name.
	Strategy() string
	// NumProviders returns the number of configured providers.
	NumProviders() int
	// Providers returns diagnostic info for all providers.
	Providers() []ProviderStatus
	// SetActiveProvider moves the named provider to the front of the pool.
	SetActiveProvider(name string) error
	// FallbackModel returns the current fallback model name, empty if none.
	FallbackModel() string
	// SetFallbackModel sets the fallback model name.
	SetFallbackModel(model string)
}

// ProviderCommand displays and controls provider load-balancing and failover settings.
type ProviderCommand struct {
	Switcher ProviderSwitcher
}

// Metadata returns the canonical slash descriptor for /provider.
func (c ProviderCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "provider",
		Description: "Show or switch the active provider for load balancing and failover",
		Usage:       "/provider [/provider switch <name> | /provider fallback <model>]",
	}
}

// Execute reports provider status or performs provider switching.
func (c ProviderCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	if c.Switcher == nil {
		return command.Result{Output: "Provider command is not available: runtime not wired."}, nil
	}

	raw := strings.TrimSpace(args.RawLine)
	parts := strings.Fields(raw)

	if len(parts) == 0 {
		return c.showStatus()
	}

	switch parts[0] {
	case "switch":
		return c.switchProvider(parts[1:])
	case "fallback":
		return c.setFallback(parts[1:])
	default:
		return command.Result{}, fmt.Errorf("unknown subcommand: %s\nUsage: %s", parts[0], c.Metadata().Usage)
	}
}

func (c ProviderCommand) showStatus() (command.Result, error) {
	var b strings.Builder
	b.WriteString("Provider Status:\n")
	b.WriteString(fmt.Sprintf("  Strategy: %s\n", c.Switcher.Strategy()))
	b.WriteString(fmt.Sprintf("  Providers: %d\n\n", c.Switcher.NumProviders()))

	for _, pi := range c.Switcher.Providers() {
		state := pi.CircuitBreakerState
		if state == "" {
			state = "unknown"
		}
		active := ""
		if pi.Position == 0 {
			active = " ← active"
		}
		b.WriteString(fmt.Sprintf("  %d. %s%s\n", pi.Position+1, pi.Name, active))
		b.WriteString(fmt.Sprintf("     Circuit breaker: %s\n", state))
		if pi.FailureCount > 0 || pi.TripCount > 0 {
			b.WriteString(fmt.Sprintf("     Failures: %d, Trips: %d\n", pi.FailureCount, pi.TripCount))
		}
	}

	if fm := c.Switcher.FallbackModel(); fm != "" {
		b.WriteString(fmt.Sprintf("\n  Fallback model: %s\n", fm))
	}

	return command.Result{Output: b.String()}, nil
}

func (c ProviderCommand) switchProvider(args []string) (command.Result, error) {
	if len(args) == 0 {
		return command.Result{}, fmt.Errorf("usage: /provider switch <name>")
	}

	target := strings.Join(args, " ")
	if err := c.Switcher.SetActiveProvider(target); err != nil {
		return command.Result{}, fmt.Errorf("switch failed: %w", err)
	}

	return command.Result{Output: fmt.Sprintf("Switched active provider to %q.", target)}, nil
}

func (c ProviderCommand) setFallback(args []string) (command.Result, error) {
	if len(args) == 0 {
		fm := c.Switcher.FallbackModel()
		if fm == "" {
			return command.Result{Output: "No fallback model configured."}, nil
		}
		return command.Result{Output: fmt.Sprintf("Current fallback model: %s", fm)}, nil
	}

	model := strings.Join(args, " ")
	c.Switcher.SetFallbackModel(model)
	return command.Result{Output: fmt.Sprintf("Fallback model set to %q.", model)}, nil
}
