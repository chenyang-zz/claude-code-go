package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const initVerifiersCommandFallback = "Init-verifiers flow is not available in Claude Code Go yet. Internal verifier bootstrap workflow remains unmigrated."

// InitVerifiersCommand exposes the minimum hidden /init-verifiers behavior before internal verifier bootstrap flows exist in the Go host.
type InitVerifiersCommand struct{}

// Metadata returns the canonical slash descriptor for /init-verifiers.
func (c InitVerifiersCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "init-verifiers",
		Description: "Create verifier skill(s) for automated verification of code changes",
		Usage:       "/init-verifiers",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /init-verifiers fallback.
func (c InitVerifiersCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered init-verifiers command fallback output", map[string]any{
		"init_verifiers_available": false,
		"hidden_command":           true,
	})

	return command.Result{
		Output: initVerifiersCommandFallback,
	}, nil
}
