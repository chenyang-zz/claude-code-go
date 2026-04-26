package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const btwCommandFallback = "Side-question execution is not available in Claude Code Go yet. The dedicated forked side-thread model flow and interactive answer pane remain unmigrated."

// BtwCommand exposes the minimum text-only /btw behavior before side-question runtime flows exist in the Go host.
type BtwCommand struct{}

// Metadata returns the canonical slash descriptor for /btw.
func (c BtwCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "btw",
		Description: "Ask a quick side question without interrupting the main conversation",
		Usage:       "/btw <question>",
	}
}

// Execute validates the required question argument and reports the stable /btw fallback.
func (c BtwCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	question := strings.TrimSpace(args.RawLine)
	if question == "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered btw command fallback output", map[string]any{
		"btw_available": false,
		"question":      question,
	})

	return command.Result{
		Output: btwCommandFallback,
	}, nil
}
