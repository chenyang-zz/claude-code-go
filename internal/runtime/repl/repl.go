package repl

import (
	"context"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	"github.com/sheepzhao/claude-code-go/internal/ui/console"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Runner coordinates one CLI turn between parsed input, engine execution and console rendering.
type Runner struct {
	// Engine handles normal prompt execution.
	Engine engine.Engine
	// Renderer handles console output for both engine events and slash placeholders.
	Renderer *console.StreamRenderer
}

// NewRunner builds a runner from explicit dependencies.
func NewRunner(eng engine.Engine, renderer *console.StreamRenderer) *Runner {
	return &Runner{
		Engine:   eng,
		Renderer: renderer,
	}
}

// Run parses the CLI args and dispatches either a slash placeholder or one text turn.
func (r *Runner) Run(ctx context.Context, args []string) error {
	parsed, err := ParseArgs(args)
	if err != nil {
		return err
	}

	logger.DebugCF("repl", "parsed cli input", map[string]any{
		"is_slash_command": parsed.IsSlashCommand,
		"command":          parsed.Command,
	})

	if parsed.IsSlashCommand {
		return r.Renderer.RenderLine(fmt.Sprintf("Slash command /%s is not supported yet.", parsed.Command))
	}

	stream, err := r.Engine.Run(ctx, conversation.RunRequest{
		SessionID: "cli",
		Input:     parsed.Body,
	})
	if err != nil {
		return err
	}

	return r.Renderer.Render(stream)
}
