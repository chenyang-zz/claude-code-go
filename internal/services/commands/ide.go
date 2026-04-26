package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const ideCommandFallback = "IDE integrations are not available in Claude Code Go yet. Interactive IDE status panels, extension discovery, and one-click open flows remain unmigrated."

// IdeCommand exposes the minimum text-only /ide behavior before IDE host integrations exist in the Go runtime.
type IdeCommand struct{}

// Metadata returns the canonical slash descriptor for /ide.
func (c IdeCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "ide",
		Description: "Manage IDE integrations and show status",
		Usage:       "/ide [open]",
	}
}

// Execute reports the stable /ide fallback supported by the current Go host.
func (c IdeCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered ide command fallback output", map[string]any{
		"ide_integrations_available": false,
	})

	return command.Result{
		Output: ideCommandFallback,
	}, nil
}
