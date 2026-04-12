package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// FastModeStore persists the global fast-mode preference for slash commands that mutate inference defaults.
type FastModeStore interface {
	// SaveFastMode writes the requested fast-mode preference into durable user-scoped settings.
	SaveFastMode(ctx context.Context, enabled bool) error
}

// FastCommand exposes the minimum text-only /fast flow before the interactive fast-mode picker exists in the Go host.
type FastCommand struct {
	// Config carries the resolved runtime configuration snapshot used to derive the current fast-mode state.
	Config *coreconfig.Config
	// Store persists the updated fast-mode setting into global settings.
	Store FastModeStore
}

// Metadata returns the canonical slash descriptor for /fast.
func (c FastCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "fast",
		Description: "Toggle fast mode (Opus 4.6 only)",
		Usage:       "/fast [on|off]",
	}
}

// Execute reports the current fast-mode state or persists one explicit on/off preference.
func (c FastCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	requested := strings.TrimSpace(strings.ToLower(args.RawLine))
	current := currentFastModeValue(c.Config)

	if requested == "" {
		return command.Result{
			Output: fmt.Sprintf(
				"Fast mode is currently %s.\nRun /fast on to persist fast mode, or /fast off to clear it.\nClaude Code Go does not provide the interactive fast-mode picker, subscription gating, model auto-switching, or cooldown and quota handling yet.",
				renderFastModeState(current),
			),
		}, nil
	}
	if len(args.Raw) != 1 {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}
	if requested != "on" && requested != "off" {
		return command.Result{}, fmt.Errorf("unsupported fast mode setting %q. Expected one of: on, off", requested)
	}
	if c.Store == nil {
		return command.Result{}, fmt.Errorf("global fast mode storage is not configured")
	}

	enabled := requested == "on"
	if err := c.Store.SaveFastMode(ctx, enabled); err != nil {
		return command.Result{}, err
	}
	if c.Config != nil {
		c.Config.FastMode = enabled
		c.Config.HasFastModeSetting = enabled
	}

	logger.DebugCF("commands", "updated fast mode via fast command", map[string]any{
		"previous_fast_mode": current,
		"new_fast_mode":      enabled,
		"cleared":            !enabled,
	})

	if enabled {
		return command.Result{
			Output: "Fast mode enabled. Claude Code Go stores the preference now, but the interactive fast-mode picker, subscription gating, model auto-switching, and cooldown/quota handling are not implemented yet.",
		}, nil
	}

	return command.Result{
		Output: "Fast mode disabled. Claude Code Go clears the persisted preference now, but the interactive fast-mode picker, subscription gating, model auto-switching, and cooldown/quota handling are not implemented yet.",
	}, nil
}

// currentFastModeValue returns the resolved fast-mode preference from the runtime snapshot.
func currentFastModeValue(cfg *coreconfig.Config) bool {
	if cfg == nil || !cfg.HasFastModeSetting {
		return false
	}
	return cfg.FastMode
}

// renderFastModeState formats the boolean fast-mode state into one stable ON/OFF label.
func renderFastModeState(enabled bool) string {
	if enabled {
		return "ON"
	}
	return "OFF"
}
