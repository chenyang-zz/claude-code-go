package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const filesCommandFallback = "File context listing is not available in Claude Code Go yet."

// FilesCommand exposes the minimum text-only /files behavior available before file context tracking exists in the Go host.
type FilesCommand struct{}

// Metadata returns the canonical slash descriptor for /files.
func (c FilesCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "files",
		Description: "List all files currently in context",
		Usage:       "/files",
	}
}

// Execute reports the stable file-context fallback supported by the current Go host.
func (c FilesCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered files command fallback output", map[string]any{
		"file_context_available": false,
	})

	return command.Result{
		Output: filesCommandFallback,
	}, nil
}
