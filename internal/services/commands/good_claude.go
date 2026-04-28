package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const goodClaudeCommandFallback = "Good-Claude flow is not available in Claude Code Go yet. Internal reward workflow remains unmigrated."

// GoodClaudeCommand exposes the minimum hidden /good-claude behavior before real flows exist in the Go host.
type GoodClaudeCommand struct{}

// Metadata returns the canonical slash descriptor for /good-claude.
func (c GoodClaudeCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "good-claude",
		Description: "Run internal good-claude workflow",
		Usage:       "/good-claude",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /good-claude fallback.
func (c GoodClaudeCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered good-claude command fallback output", map[string]any{
		"good_claude_available": false,
		"hidden_command":        true,
	})

	return command.Result{
		Output: goodClaudeCommandFallback,
	}, nil
}
