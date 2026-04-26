package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const mobileCommandFallback = "Mobile app handoff is not available in Claude Code Go yet. QR code rendering, platform-specific install links, and interactive mobile continuation remain unmigrated."

// MobileCommand exposes the minimum text-only /mobile behavior before mobile handoff exists in the Go runtime.
type MobileCommand struct{}

// Metadata returns the canonical slash descriptor for /mobile.
func (c MobileCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "mobile",
		Aliases:     []string{"ios", "android"},
		Description: "Show QR code to download the Claude mobile app",
		Usage:       "/mobile",
	}
}

// Execute reports the stable /mobile fallback supported by the current Go host.
func (c MobileCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered mobile command fallback output", map[string]any{
		"mobile_handoff_available": false,
	})

	return command.Result{
		Output: mobileCommandFallback,
	}, nil
}
