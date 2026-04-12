package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const terminalSetupCommandFallback = "Terminal setup automation is not available in Claude Code Go yet. Shift+Enter shortcut installation, terminal-specific config writes, backup/restore, remote-session guidance, and shell completion setup remain unmigrated."

// TerminalSetupCommand exposes the minimum text-only /terminal-setup behavior before host-specific terminal integration exists in the Go runtime.
type TerminalSetupCommand struct{}

// Metadata returns the canonical slash descriptor for /terminal-setup.
func (c TerminalSetupCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "terminal-setup",
		Description: "Install Shift+Enter key binding for newlines",
		Usage:       "/terminal-setup",
	}
}

// Execute reports the stable terminal-setup fallback supported by the current Go host.
func (c TerminalSetupCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered terminal-setup command fallback output", map[string]any{
		"shortcut_install_available":      false,
		"terminal_config_write_available": false,
		"shell_completion_setup":          false,
	})

	return command.Result{
		Output: terminalSetupCommandFallback,
	}, nil
}
