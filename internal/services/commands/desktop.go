package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const desktopCommandFallback = "Desktop handoff is not available in Claude Code Go yet. Claude Desktop deep-link continuation, platform-specific app launch, and host bridge integration remain unmigrated."

// DesktopCommand exposes the minimum text-only /desktop behavior before desktop host handoff exists in the Go runtime.
type DesktopCommand struct{}

// Metadata returns the canonical slash descriptor for /desktop.
func (c DesktopCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "desktop",
		Aliases:     []string{"app"},
		Description: "Continue the current session in Claude Desktop",
		Usage:       "/desktop",
	}
}

// Execute reports the stable /desktop fallback supported by the current Go host.
func (c DesktopCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered desktop command fallback output", map[string]any{
		"desktop_handoff_available": false,
	})

	return command.Result{
		Output: desktopCommandFallback,
	}, nil
}
