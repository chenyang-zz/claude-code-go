package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const exitCommandFallback = "Session exit shortcut is not available in Claude Code Go yet. Interactive REPL shutdown coordination and immediate local-jsx exit handling remain unmigrated."

// ExitCommand exposes the minimum text-only /exit behavior before immediate REPL shutdown integration exists in the Go runtime.
type ExitCommand struct{}

// Metadata returns the canonical slash descriptor for /exit.
func (c ExitCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "exit",
		Aliases:     []string{"quit"},
		Description: "Exit the REPL",
		Usage:       "/exit",
	}
}

// Execute reports the stable /exit fallback supported by the current Go host.
func (c ExitCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered exit command fallback output", map[string]any{
		"repl_exit_shortcut_available": false,
	})

	return command.Result{
		Output: exitCommandFallback,
	}, nil
}
