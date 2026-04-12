package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// MemoryCommand renders the minimum text-only /memory behavior available before memory file editing is migrated.
type MemoryCommand struct{}

// Metadata returns the canonical slash descriptor for /memory.
func (c MemoryCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "memory",
		Description: "Edit Claude memory files",
		Usage:       "/memory",
	}
}

// Execute reports the current Go host fallback until memory file discovery and editor integration exist.
func (c MemoryCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered memory command fallback output", map[string]any{
		"memory_file_selection_available": false,
		"editor_integration_available":    false,
		"memory_file_editing_available":   false,
	})

	return command.Result{
		Output: "Memory file editing is not available in Claude Code Go yet. Memory file discovery, interactive selection, file creation, and editor launch remain unmigrated.",
	}, nil
}
