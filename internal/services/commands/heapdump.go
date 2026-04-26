package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const heapdumpCommandFallback = "Heap dump export is not available in Claude Code Go yet. JavaScript heap snapshot and diagnostics file generation remain unmigrated."

// HeapdumpCommand exposes the minimum hidden /heapdump behavior before heap snapshot support exists in the Go host.
type HeapdumpCommand struct{}

// Metadata returns the canonical slash descriptor for /heapdump.
func (c HeapdumpCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "heapdump",
		Description: "Dump the JS heap to ~/Desktop",
		Usage:       "/heapdump",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /heapdump fallback.
func (c HeapdumpCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered heapdump command fallback output", map[string]any{
		"heapdump_available": false,
		"hidden_command":     true,
	})

	return command.Result{
		Output: heapdumpCommandFallback,
	}, nil
}
