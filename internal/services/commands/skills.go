package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const skillsCommandFallback = "Skills listing is not available in Claude Code Go yet. Dynamic skill discovery, plugin-provided skill surfacing, and rich skill metadata rendering remain unmigrated."

// SkillsCommand exposes the minimum text-only /skills behavior before dynamic skill discovery exists in the Go runtime.
type SkillsCommand struct{}

// Metadata returns the canonical slash descriptor for /skills.
func (c SkillsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "skills",
		Description: "List available skills",
		Usage:       "/skills",
	}
}

// Execute reports the stable /skills fallback supported by the current Go host.
func (c SkillsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered skills command fallback output", map[string]any{
		"skills_listing_available": false,
	})

	return command.Result{
		Output: skillsCommandFallback,
	}, nil
}
