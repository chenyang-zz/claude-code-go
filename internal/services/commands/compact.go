package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// CompactCommand renders the minimum text-only /compact behavior available before summary-preserving compaction is migrated.
type CompactCommand struct{}

// Metadata returns the canonical slash descriptor for /compact.
func (c CompactCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "compact",
		Description: "Clear conversation history but keep a summary in context",
		Usage:       "/compact [instructions]",
	}
}

// Execute reports the current Go host fallback until compact summary generation and context replacement exist.
func (c CompactCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered compact command fallback output", map[string]any{
		"compact_available":       false,
		"custom_instructions":     false,
		"summary_preservation":    false,
		"clear_command_available": true,
	})

	return command.Result{
		Output: "Conversation compaction is not available in Claude Code Go yet. Use /clear to start a new session; summary-preserving compact, custom instructions, and compact hooks remain unmigrated.",
	}, nil
}
