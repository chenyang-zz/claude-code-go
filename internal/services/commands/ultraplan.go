package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const ultraplanCommandFallback = "Ultraplan web planning flow is not available in Claude Code Go yet. Remote session launch, approval polling, and CCR handoff remain unmigrated."

// UltraplanCommand exposes the minimum hidden /ultraplan behavior before remote ultraplan flows exist in the Go host.
type UltraplanCommand struct{}

// Metadata returns the canonical slash descriptor for /ultraplan.
func (c UltraplanCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "ultraplan",
		Description: "Draft an advanced plan in Claude Code on the web",
		Usage:       "/ultraplan <prompt>",
		Hidden:      true,
	}
}

// Execute validates the required prompt argument and reports the stable hidden /ultraplan fallback.
func (c UltraplanCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	prompt := strings.TrimSpace(args.RawLine)
	if prompt == "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered ultraplan command fallback output", map[string]any{
		"ultraplan_available": false,
		"hidden_command":      true,
		"prompt_provided":     true,
	})

	return command.Result{
		Output: ultraplanCommandFallback,
	}, nil
}
