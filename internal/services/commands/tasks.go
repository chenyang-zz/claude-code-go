package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const tasksCommandFallback = "Background task management is not available in Claude Code Go yet. Background task listing, status inspection, cancellation, and interactive task controls remain unmigrated."

// TasksCommand exposes the minimum text-only /tasks behavior before background-task UI and task management exist in the Go host.
type TasksCommand struct{}

// Metadata returns the canonical slash descriptor for /tasks.
func (c TasksCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "tasks",
		Aliases:     []string{"bashes"},
		Description: "List and manage background tasks",
		Usage:       "/tasks",
	}
}

// Execute reports the stable /tasks fallback supported by the current Go host.
func (c TasksCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered tasks command fallback output", map[string]any{
		"background_task_list_available": false,
		"task_control_available":         false,
	})

	return command.Result{
		Output: tasksCommandFallback,
	}, nil
}
