package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const perfIssueCommandFallback = "Perf-issue flow is not available in Claude Code Go yet. Internal performance issue workflow remains unmigrated."

// PerfIssueCommand exposes the minimum hidden /perf-issue behavior before internal performance issue flows exist in the Go host.
type PerfIssueCommand struct{}

// Metadata returns the canonical slash descriptor for /perf-issue.
func (c PerfIssueCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "perf-issue",
		Description: "Run internal performance issue workflow",
		Usage:       "/perf-issue",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /perf-issue fallback.
func (c PerfIssueCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered perf-issue command fallback output", map[string]any{
		"perf_issue_available": false,
		"hidden_command":       true,
	})

	return command.Result{
		Output: perfIssueCommandFallback,
	}, nil
}
