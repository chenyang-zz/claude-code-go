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
	tasksCommandEmptyState = "No background tasks are running."
	tasksCommandNoControl  = "Controls: no stoppable background tasks right now."
	tasksCommandStopReady  = "Controls: stop available via /tasks stop <task-id>."
)

// BackgroundTaskSnapshotResumer exposes the minimum runtime resume capability consumed by `/tasks resume`.
type BackgroundTaskSnapshotResumer interface {
	// Resume requests one stopped background task to resume execution with an optional message.
	Resume(id string, message string) (coresession.BackgroundTaskSnapshot, error)
}

// BackgroundTaskSnapshotLister exposes the minimum runtime task snapshot source consumed by `/tasks`.
type BackgroundTaskSnapshotLister interface {
	// List returns the currently visible runtime background tasks.
	List() []coresession.BackgroundTaskSnapshot
	// Stop requests termination of one running background task.
	Stop(id string) (coresession.BackgroundTaskSnapshot, error)
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
		Usage:       "/tasks | /tasks stop <task-id> | /tasks resume <task-id> [message]",
	}
}

// Execute reports the stable /tasks fallback supported by the current Go host.
func (c TasksCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	if len(args.Raw) > 0 && args.Raw[0] == "stop" {
		return c.executeStop(args.Raw)
	}
	if len(args.Raw) > 0 && args.Raw[0] == "resume" {
		return c.executeResume(args.Raw)
	}

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

// executeResume resumes one background task by ID through the shared task store when resume is implemented.
func (c TasksCommand) executeResume(rawArgs []string) (command.Result, error) {
	resumer, ok := c.TaskStore.(BackgroundTaskSnapshotResumer)
	if !ok || resumer == nil {
		logger.DebugCF("commands", "tasks resume skipped: task resume unavailable", nil)
		return command.Result{Output: "Background task resume is unavailable."}, nil
	}
	if len(rawArgs) < 2 {
		logger.DebugCF("commands", "tasks resume rejected: invalid usage", map[string]any{
			"raw_args_count": len(rawArgs),
		})
		return command.Result{Output: "Usage: /tasks resume <task-id> [message]"}, nil
	}
	taskID := strings.TrimSpace(rawArgs[1])
	if taskID == "" {
		logger.DebugCF("commands", "tasks resume rejected: empty task id", nil)
		return command.Result{Output: "Usage: /tasks resume <task-id> [message]"}, nil
	}
	message := ""
	if len(rawArgs) > 2 {
		message = strings.TrimSpace(strings.Join(rawArgs[2:], " "))
	}
	snapshot, err := resumer.Resume(taskID, message)
	if err != nil {
		logger.DebugCF("commands", "tasks resume failed", map[string]any{
			"task_id": taskID,
			"error":   err.Error(),
		})
		return command.Result{Output: fmt.Sprintf("Failed to resume task %s: %v", taskID, err)}, nil
	}
	logger.DebugCF("commands", "tasks resume succeeded", map[string]any{
		"task_id":   snapshot.ID,
		"task_type": snapshot.Type,
		"status":    string(snapshot.Status),
	})
	summary := strings.TrimSpace(snapshot.Summary)
	if summary == "" {
		return command.Result{Output: fmt.Sprintf("Resumed background task: %s", snapshot.ID)}, nil
	}
	return command.Result{Output: fmt.Sprintf("Resumed background task: %s (%s)", snapshot.ID, summary)}, nil
}

// executeStop stops one background task by ID through the shared task store.
func (c TasksCommand) executeStop(rawArgs []string) (command.Result, error) {
	if c.TaskStore == nil {
		logger.DebugCF("commands", "tasks stop skipped: task store unavailable", nil)
		return command.Result{Output: "Background task store is unavailable."}, nil
	}
	if len(rawArgs) != 2 {
		logger.DebugCF("commands", "tasks stop rejected: invalid usage", map[string]any{
			"raw_args_count": len(rawArgs),
		})
		return command.Result{Output: "Usage: /tasks stop <task-id>"}, nil
	}
	taskID := strings.TrimSpace(rawArgs[1])
	if taskID == "" {
		logger.DebugCF("commands", "tasks stop rejected: empty task id", nil)
		return command.Result{Output: "Usage: /tasks stop <task-id>"}, nil
	}
	snapshot, err := c.TaskStore.Stop(taskID)
	if err != nil {
		logger.DebugCF("commands", "tasks stop failed", map[string]any{
			"task_id": taskID,
			"error":   err.Error(),
		})
		return command.Result{Output: fmt.Sprintf("Failed to stop task %s: %v", taskID, err)}, nil
	}
	logger.DebugCF("commands", "tasks stop succeeded", map[string]any{
		"task_id":   snapshot.ID,
		"task_type": snapshot.Type,
		"status":    string(snapshot.Status),
	})
	summary := strings.TrimSpace(snapshot.Summary)
	if summary == "" {
		return command.Result{Output: fmt.Sprintf("Stopped background task: %s", snapshot.ID)}, nil
	}
	return command.Result{Output: fmt.Sprintf("Stopped background task: %s (%s)", snapshot.ID, summary)}, nil
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
	for _, task := range tasks {
		if !task.ControlsAvailable {
			continue
		}
		switch task.Status {
		case coresession.BackgroundTaskStatusRunning, coresession.BackgroundTaskStatusPending:
			return true
		}
	}
	return false
}
