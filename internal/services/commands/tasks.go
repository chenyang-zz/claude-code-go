package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	tasksCommandEmptyState = "No background tasks are running.\nBackground task controls are not available in Claude Code Go yet."
	tasksCommandNoControl  = "Task listing is available, but stop/resume controls are not migrated yet."
	tasksCommandStopReady  = "Controls: stop available for local bash tasks."
)

// BackgroundTaskSnapshotLister exposes the minimum runtime task snapshot source consumed by `/tasks`.
type BackgroundTaskSnapshotLister interface {
	// List returns the currently visible runtime background tasks.
	List() []coresession.BackgroundTaskSnapshot
}

// TasksCommand exposes the minimum text-only /tasks behavior before background-task UI and task management exist in the Go host.
type TasksCommand struct {
	// TaskStore provides the shared runtime task snapshot source.
	TaskStore BackgroundTaskSnapshotLister
}

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

	tasks := c.listTasks()
	output := renderTasksOutput(tasks)

	logger.DebugCF("commands", "rendered tasks command fallback output", map[string]any{
		"background_task_count":          len(tasks),
		"background_task_list_available": true,
		"task_control_available":         tasksControlsAvailable(tasks),
	})

	return command.Result{
		Output: output,
	}, nil
}

// listTasks returns the shared runtime task snapshots when available.
func (c TasksCommand) listTasks() []coresession.BackgroundTaskSnapshot {
	if c.TaskStore == nil {
		return nil
	}
	return c.TaskStore.List()
}

// renderTasksOutput formats the minimum stable `/tasks` output for the current runtime snapshots.
func renderTasksOutput(tasks []coresession.BackgroundTaskSnapshot) string {
	if len(tasks) == 0 {
		return tasksCommandEmptyState
	}

	lines := []string{
		fmt.Sprintf("Background tasks: %d", len(tasks)),
	}
	for _, task := range tasks {
		lines = append(lines, renderTaskLine(task))
	}
	if tasksControlsAvailable(tasks) {
		lines = append(lines, tasksCommandStopReady)
	} else {
		lines = append(lines, tasksCommandNoControl)
	}
	return strings.Join(lines, "\n")
}

// renderTaskLine formats one read-only background task summary line.
func renderTaskLine(task coresession.BackgroundTaskSnapshot) string {
	line := fmt.Sprintf("- %s: %s - %s", displayValue(task.ID), displayValue(task.Type), displayValue(string(task.Status)))
	if summary := strings.TrimSpace(task.Summary); summary != "" {
		line += fmt.Sprintf(" - %s", summary)
	}
	return line
}

// tasksControlsAvailable reports whether every visible task currently supports control actions.
func tasksControlsAvailable(tasks []coresession.BackgroundTaskSnapshot) bool {
	if len(tasks) == 0 {
		return false
	}
	for _, task := range tasks {
		if !task.ControlsAvailable {
			return false
		}
	}
	return true
}
