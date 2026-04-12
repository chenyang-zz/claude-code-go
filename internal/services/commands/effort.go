package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// EffortLevelStore persists the global effort preference for slash commands that mutate inference defaults.
type EffortLevelStore interface {
	// SaveEffortLevel writes the requested effort override into durable user-scoped settings.
	SaveEffortLevel(ctx context.Context, effort string) error
}

// EffortCommand exposes the minimum text-only /effort flow before the interactive controls exist in the Go host.
type EffortCommand struct {
	// Config carries the resolved runtime configuration snapshot used to derive the current effort setting.
	Config *coreconfig.Config
	// Store persists the updated effort setting into global settings.
	Store EffortLevelStore
}

// Metadata returns the canonical slash descriptor for /effort.
func (c EffortCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "effort",
		Description: "Set effort level for model usage",
		Usage:       "/effort [low|medium|high|max|auto]",
	}
}

// Execute reports the current effort level or persists one explicit effort override.
func (c EffortCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	requested := strings.TrimSpace(strings.ToLower(args.RawLine))
	previous := currentEffortValue(c.Config)

	if requested == "" || requested == "current" || requested == "status" {
		return command.Result{
			Output: fmt.Sprintf(
				"Current effort level: %s\nRun /effort <low|medium|high|max> to persist an effort override, or /effort auto to clear it.\nClaude Code Go does not provide env overrides, session-only effort, model capability checks, or interactive effort controls yet.",
				previous,
			),
		}, nil
	}
	if len(args.Raw) != 1 {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}
	if c.Store == nil {
		return command.Result{}, fmt.Errorf("global effort storage is not configured")
	}

	persisted := requested
	if requested == "auto" || requested == "unset" {
		persisted = ""
	} else if !coreconfig.IsSupportedEffortLevel(requested) {
		return command.Result{}, fmt.Errorf("unsupported effort level %q. Expected one of: low, medium, high, max, auto", requested)
	}

	if err := c.Store.SaveEffortLevel(ctx, persisted); err != nil {
		return command.Result{}, err
	}
	if c.Config != nil {
		c.Config.EffortLevel = persisted
		c.Config.HasEffortLevelSetting = persisted != ""
	}

	logger.DebugCF("commands", "updated effort via effort command", map[string]any{
		"previous_effort": previous,
		"new_effort":      renderEffortValue(persisted),
		"cleared":         persisted == "",
	})

	if persisted == "" {
		return command.Result{
			Output: "Effort level set to auto. Claude Code Go clears the persisted effort override now, but env overrides, session-only effort, model capability checks, and interactive effort controls are not implemented yet.",
		}, nil
	}

	return command.Result{
		Output: fmt.Sprintf("Effort level set to %s. Claude Code Go stores the preference now, but env overrides, session-only effort, model capability checks, and interactive effort controls are not implemented yet.", persisted),
	}, nil
}

// currentEffortValue returns the resolved effort label from the runtime snapshot.
func currentEffortValue(cfg *coreconfig.Config) string {
	if cfg == nil || !cfg.HasEffortLevelSetting || strings.TrimSpace(cfg.EffortLevel) == "" {
		return "auto"
	}
	return cfg.EffortLevel
}

// renderEffortValue formats a persisted effort value into one stable user-facing label.
func renderEffortValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "auto"
	}
	return value
}
