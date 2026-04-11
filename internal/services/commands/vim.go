package commands

import (
	"context"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// EditorModeStore persists the global editor mode for slash commands that mutate prompt editor settings.
type EditorModeStore interface {
	// SaveEditorMode writes the requested editor mode into durable user-scoped settings.
	SaveEditorMode(ctx context.Context, mode string) error
}

// VimCommand toggles the persisted prompt editor mode between normal and vim.
type VimCommand struct {
	// Config carries the resolved runtime configuration snapshot used to derive the current mode.
	Config *coreconfig.Config
	// Store persists the updated editor mode into global settings.
	Store EditorModeStore
}

// Metadata returns the canonical slash descriptor for /vim.
func (c VimCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "vim",
		Description: "Toggle between Vim and Normal editing modes",
		Usage:       "/vim",
	}
}

// Execute flips the current editor mode, persists it, and reports the stable Go-host boundary.
func (c VimCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = args

	currentMode := coreconfig.EditorModeNormal
	if c.Config != nil {
		currentMode = coreconfig.NormalizeEditorMode(c.Config.EditorMode)
	}

	newMode := coreconfig.EditorModeVim
	if currentMode == coreconfig.EditorModeVim {
		newMode = coreconfig.EditorModeNormal
	}

	if c.Store == nil {
		return command.Result{}, fmt.Errorf("global editor mode storage is not configured")
	}
	if err := c.Store.SaveEditorMode(ctx, newMode); err != nil {
		return command.Result{}, err
	}
	if c.Config != nil {
		c.Config.EditorMode = newMode
	}

	logger.DebugCF("commands", "updated editor mode via vim command", map[string]any{
		"previous_mode": currentMode,
		"new_mode":      newMode,
	})

	return command.Result{
		Output: fmt.Sprintf("Editor mode set to %s. Claude Code Go stores the setting now, but prompt-editor Vim keybindings are not implemented yet.", newMode),
	}, nil
}
