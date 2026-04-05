package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// SessionCommand renders the minimum text-only /session behavior available before remote mode exists in the Go host.
type SessionCommand struct{}

// Metadata returns the canonical slash descriptor for /session.
func (c SessionCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "session",
		Description: "Show remote session URL and QR code",
		Usage:       "/session",
	}
}

// Execute reports the stable non-remote fallback until remote session infrastructure is migrated.
func (c SessionCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered session command fallback output", map[string]any{
		"remote_mode_available": false,
	})

	return command.Result{
		Output: "Not in remote mode. Start with `claude --remote` to use this command.",
	}, nil
}
