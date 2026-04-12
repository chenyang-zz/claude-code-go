package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const exportCommandFallback = "Conversation export is not available in Claude Code Go yet."

// ExportCommand exposes the minimum text-only /export behavior available before conversation rendering and export dialogs exist in the Go host.
type ExportCommand struct{}

// Metadata returns the canonical slash descriptor for /export.
func (c ExportCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "export",
		Description: "Export the current conversation to a file or clipboard",
		Usage:       "/export [filename]",
	}
}

// Execute reports the stable conversation-export fallback supported by the current Go host.
func (c ExportCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered export command fallback output", map[string]any{
		"conversation_export_available": false,
		"clipboard_available":           false,
	})

	return command.Result{
		Output: exportCommandFallback,
	}, nil
}
