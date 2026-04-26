package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const tagCommandFallback = "Session tag toggling is not available in Claude Code Go yet. Searchable tag persistence, ant-user gating, and session index integration remain unmigrated."

// TagCommand exposes the minimum text-only /tag behavior before tag persistence features exist in the Go runtime.
type TagCommand struct{}

// Metadata returns the canonical slash descriptor for /tag.
func (c TagCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "tag",
		Description: "Toggle a searchable tag on the current session",
		Usage:       "/tag <tag-name>",
	}
}

// Execute validates one tag argument and reports the stable /tag fallback supported by the current Go host.
func (c TagCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	tagName := strings.TrimSpace(args.RawLine)
	if tagName == "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered tag command fallback output", map[string]any{
		"tag_toggle_available": false,
		"tag_name":             tagName,
	})

	return command.Result{
		Output: tagCommandFallback,
	}, nil
}
