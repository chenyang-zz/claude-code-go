package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const bughunterCommandFallback = "Bughunter flow is not available in Claude Code Go yet. Internal bughunter workflow remains unmigrated."

// BughunterCommand exposes the minimum hidden /bughunter behavior before bughunter flows exist in the Go host.
type BughunterCommand struct{}

// Metadata returns the canonical slash descriptor for /bughunter.
func (c BughunterCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "bughunter",
		Description: "Run internal bughunter workflow",
		Usage:       "/bughunter",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /bughunter fallback.
func (c BughunterCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered bughunter command fallback output", map[string]any{
		"bughunter_available": false,
		"hidden_command":      true,
	})

	return command.Result{
		Output: bughunterCommandFallback,
	}, nil
}
