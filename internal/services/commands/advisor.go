package commands

import (
	"context"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const advisorCommandFallback = "Advisor model configuration is not available in Claude Code Go yet. Model validation, settings persistence, and advisor runtime wiring remain unmigrated."

// AdvisorCommand exposes the minimum text-only /advisor behavior before advisor model flows exist in the Go host.
type AdvisorCommand struct{}

// Metadata returns the canonical slash descriptor for /advisor.
func (c AdvisorCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "advisor",
		Description: "Configure the advisor model",
		Usage:       "/advisor [<model>|off]",
	}
}

// Execute accepts optional arguments and reports the stable /advisor fallback.
func (c AdvisorCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)

	logger.DebugCF("commands", "rendered advisor command fallback output", map[string]any{
		"advisor_runtime_available": false,
		"arg_provided":              raw != "",
	})

	return command.Result{
		Output: advisorCommandFallback,
	}, nil
}
