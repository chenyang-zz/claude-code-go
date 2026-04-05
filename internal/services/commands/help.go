package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// HelpCommand renders a stable text summary of the currently registered minimum slash commands.
type HelpCommand struct {
	// Registry supplies the command catalog that help should expose to users.
	Registry command.Registry
}

// Metadata returns the canonical slash descriptor for /help.
func (c HelpCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "help",
		Description: "Show help and available commands",
		Usage:       "/help",
	}
}

// Execute formats the currently registered commands into a stable text help block.
func (c HelpCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	lines := []string{
		"Available commands:",
	}
	for _, registered := range c.Registry.List() {
		meta := registered.Metadata()
		lines = append(lines, fmt.Sprintf("/%s - %s", meta.Name, meta.Description))
		if len(meta.Aliases) > 0 {
			aliases := make([]string, 0, len(meta.Aliases))
			for _, alias := range meta.Aliases {
				aliases = append(aliases, "/"+command.NormalizeName(alias))
			}
			lines = append(lines, fmt.Sprintf("  Aliases: %s", strings.Join(aliases, ", ")))
		}
		if meta.Usage != "" && meta.Usage != "/"+meta.Name {
			lines = append(lines, fmt.Sprintf("  Usage: %s", meta.Usage))
		}
	}
	lines = append(lines, "Send plain text without a leading slash to start a normal prompt.")

	logger.DebugCF("commands", "rendered help command output", map[string]any{
		"command_count": len(c.Registry.List()),
	})

	return command.Result{
		Output: strings.Join(lines, "\n"),
	}, nil
}
