package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const stickersCommandFallback = "Sticker ordering is not available in Claude Code Go yet. Browser launch and external store handoff remain unmigrated."

// StickersCommand exposes the minimum text-only /stickers behavior before browser handoff exists in the Go runtime.
type StickersCommand struct{}

// Metadata returns the canonical slash descriptor for /stickers.
func (c StickersCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "stickers",
		Description: "Order Claude Code stickers",
		Usage:       "/stickers",
	}
}

// Execute reports the stable /stickers fallback supported by the current Go host.
func (c StickersCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered stickers command fallback output", map[string]any{
		"stickers_ordering_available": false,
	})

	return command.Result{
		Output: stickersCommandFallback,
	}, nil
}
