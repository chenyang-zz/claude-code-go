package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const keybindingsCommandFallback = "Keybindings file management is not available in Claude Code Go yet. Keybindings file discovery, template creation, file writes, and editor launch remain unmigrated."

// KeybindingsCommand exposes the minimum text-only /keybindings behavior before file creation and editor integration exist in the Go host.
type KeybindingsCommand struct{}

// Metadata returns the canonical slash descriptor for /keybindings.
func (c KeybindingsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "keybindings",
		Description: "Open or create your keybindings configuration file",
		Usage:       "/keybindings",
	}
}

// Execute reports the stable keybindings fallback supported by the current Go host.
func (c KeybindingsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered keybindings command fallback output", map[string]any{
		"file_discovery_available": false,
		"template_creation":        false,
		"editor_launch":            false,
	})

	return command.Result{
		Output: keybindingsCommandFallback,
	}, nil
}
