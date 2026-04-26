package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const rewindCommandFallback = "Conversation rewind is not available in Claude Code Go yet. Checkpoint browsing, selective code restore, and conversation timeline rollback remain unmigrated."

// RewindCommand exposes the minimum text-only /rewind behavior before checkpoint rewind flows exist in the Go runtime.
type RewindCommand struct{}

// Metadata returns the canonical slash descriptor for /rewind.
func (c RewindCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "rewind",
		Aliases:     []string{"checkpoint"},
		Description: "Restore the code and/or conversation to a previous point",
		Usage:       "/rewind",
	}
}

// Execute reports the stable /rewind fallback supported by the current Go host.
func (c RewindCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered rewind command fallback output", map[string]any{
		"rewind_available": false,
	})

	return command.Result{
		Output: rewindCommandFallback,
	}, nil
}
