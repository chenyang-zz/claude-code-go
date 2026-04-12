package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const copyCommandFallback = "Copying Claude's last response is not available in Claude Code Go yet."

// CopyCommand exposes the minimum text-only /copy behavior available before clipboard and message-selection support exists in the Go host.
type CopyCommand struct{}

// Metadata returns the canonical slash descriptor for /copy.
func (c CopyCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "copy",
		Description: "Copy Claude's last response to clipboard (or /copy N for the Nth-latest)",
		Usage:       "/copy [N]",
	}
}

// Execute reports the stable clipboard fallback supported by the current Go host.
func (c CopyCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered copy command fallback output", map[string]any{
		"clipboard_available": false,
		"message_picker":      false,
	})

	return command.Result{
		Output: copyCommandFallback,
	}, nil
}
