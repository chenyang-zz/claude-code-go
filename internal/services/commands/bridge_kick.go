package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const bridgeKickCommandFallback = "Bridge debug fault injection is not available in Claude Code Go yet. Bridge debug handle and recovery-path fault orchestration remain unmigrated."

// BridgeKickCommand exposes the minimum hidden /bridge-kick behavior used for bridge diagnostics in the TS host.
type BridgeKickCommand struct{}

// Metadata returns the canonical slash descriptor for /bridge-kick.
func (c BridgeKickCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "bridge-kick",
		Description: "Inject bridge failure states for manual recovery testing",
		Usage:       "/bridge-kick <subcommand>",
		Hidden:      true,
	}
}

// Execute validates the required subcommand argument and reports the stable hidden fallback.
func (c BridgeKickCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw == "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered bridge-kick command fallback output", map[string]any{
		"bridge_kick_available": false,
		"hidden_command":        true,
		"arg_provided":          true,
	})

	return command.Result{
		Output: bridgeKickCommandFallback,
	}, nil
}
