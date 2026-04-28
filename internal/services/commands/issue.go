package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const issueCommandFallback = "Issue flow is not available in Claude Code Go yet. Internal issue handling remains unmigrated."

// IssueCommand exposes the minimum hidden /issue behavior before issue flows exist in the Go host.
type IssueCommand struct{}

// Metadata returns the canonical slash descriptor for /issue.
func (c IssueCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "issue",
		Description: "Run internal issue workflow",
		Usage:       "/issue",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /issue fallback.
func (c IssueCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered issue command fallback output", map[string]any{
		"issue_available": false,
		"hidden_command":  true,
	})

	return command.Result{
		Output: issueCommandFallback,
	}, nil
}
