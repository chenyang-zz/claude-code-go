package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ThemeSettingStore persists the global theme preference for slash commands that mutate terminal appearance settings.
type ThemeSettingStore interface {
	// SaveTheme writes the requested theme setting into durable user-scoped settings.
	SaveTheme(ctx context.Context, theme string) error
}

// ThemeCommand exposes the minimum text-only /theme flow before the interactive picker exists in the Go host.
type ThemeCommand struct {
	// Config carries the resolved runtime configuration snapshot used to derive the current theme.
	Config *coreconfig.Config
	// Store persists the updated theme setting into global settings.
	Store ThemeSettingStore
}

// Metadata returns the canonical slash descriptor for /theme.
func (c ThemeCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "theme",
		Description: "Change the theme",
		Usage:       "/theme <auto|dark|light|light-daltonized|dark-daltonized|light-ansi|dark-ansi>",
	}
}

// Execute reports the current theme or persists an explicit theme setting using the minimum text-only contract.
func (c ThemeCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	currentTheme := coreconfig.ThemeSettingDark
	if c.Config != nil {
		currentTheme = coreconfig.NormalizeThemeSetting(c.Config.Theme)
	}

	requested := strings.TrimSpace(args.RawLine)
	if requested == "" {
		return command.Result{
			Output: fmt.Sprintf(
				"Current theme: %s\nAvailable themes: %s\nClaude Code Go does not provide the interactive theme picker yet. Run /theme <theme> to persist a theme setting.",
				currentTheme,
				strings.Join(coreconfig.SupportedThemeSettings(), ", "),
			),
		}, nil
	}

	if len(args.Raw) != 1 {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	normalized := coreconfig.NormalizeThemeSetting(requested)
	if !coreconfig.IsSupportedThemeSetting(normalized) {
		return command.Result{}, fmt.Errorf("unsupported theme setting %q. Expected one of: %s", requested, strings.Join(coreconfig.SupportedThemeSettings(), ", "))
	}
	if c.Store == nil {
		return command.Result{}, fmt.Errorf("global theme storage is not configured")
	}
	if err := c.Store.SaveTheme(ctx, normalized); err != nil {
		return command.Result{}, err
	}
	if c.Config != nil {
		c.Config.Theme = normalized
	}

	logger.DebugCF("commands", "updated theme via theme command", map[string]any{
		"previous_theme": currentTheme,
		"new_theme":      normalized,
	})

	return command.Result{
		Output: fmt.Sprintf("Theme set to %s. Claude Code Go stores the preference now, but the interactive theme picker and TUI theme rendering are not implemented yet.", normalized),
	}, nil
}
