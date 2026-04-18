package task_list

import (
	"context"
	"fmt"
	"strings"

	coretask "github.com/sheepzhao/claude-code-go/internal/core/task"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

const (
	// Name is the stable registry identifier used by the TaskList tool.
	Name = "TaskList"
)

// TaskLister describes the minimum store capability consumed by the list tool.
type TaskLister interface {
	List(ctx context.Context) ([]*coretask.Task, error)
}

// Tool lists all non-internal tasks in the current task list.
type Tool struct {
	store TaskLister
}

// NewTool constructs a TaskList tool backed by the given store.
func NewTool(store TaskLister) *Tool {
	return &Tool{store: store}
}

// Output stores the structured result returned when tasks are listed.
type Output struct {
	// Tasks holds the summary list of all non-internal tasks.
	Tasks []TaskSummary `json:"tasks"`
}

// TaskSummary is the compact representation of a task in list views.
type TaskSummary struct {
	ID        string   `json:"id"`
	Subject   string   `json:"subject"`
	Status    string   `json:"status"`
	Owner     string   `json:"owner,omitempty"`
	BlockedBy []string `json:"blockedBy"`
}

// Name returns the stable registration name for the TaskList tool.
func (t *Tool) Name() string {
	return Name
}

// Description returns the summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return "Use this tool to list all tasks in the task list."
}

// InputSchema returns the input contract for the TaskList tool (empty).
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{},
	}
}

// IsReadOnly reports that listing tasks does not mutate state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that list requests are safe alongside other tools.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// Invoke retrieves all tasks and returns a filtered summary list.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("task list tool: nil receiver")
	}
	if t.store == nil {
		return coretool.Result{Error: "Task list is not available in Claude Code Go yet."}, nil
	}

	_, err := coretool.DecodeInput[struct{}](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	allTasks, err := t.store.List(ctx)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	// Filter out internal tasks (metadata._internal == true).
	visible := make([]*coretask.Task, 0, len(allTasks))
	for _, t := range allTasks {
		if !t.IsInternal() {
			visible = append(visible, t)
		}
	}

	// Build a set of completed task IDs for filtering blockedBy references.
	completed := make(map[string]bool)
	for _, t := range visible {
		if t.Status == coretask.StatusCompleted {
			completed[t.ID] = true
		}
	}

	summaries := make([]TaskSummary, 0, len(visible))
	for _, t := range visible {
		// Filter blockedBy to exclude completed tasks.
		blockedBy := make([]string, 0, len(t.BlockedBy))
		for _, id := range t.BlockedBy {
			if !completed[id] {
				blockedBy = append(blockedBy, id)
			}
		}
		summaries = append(summaries, TaskSummary{
			ID:        t.ID,
			Subject:   t.Subject,
			Status:    string(t.Status),
			Owner:     t.Owner,
			BlockedBy: blockedBy,
		})
	}

	if len(summaries) == 0 {
		return coretool.Result{
			Output: "No tasks found",
			Meta:   map[string]any{"data": Output{Tasks: summaries}},
		}, nil
	}

	return coretool.Result{
		Output: renderTaskList(summaries),
		Meta:   map[string]any{"data": Output{Tasks: summaries}},
	}, nil
}

// renderTaskList formats a list of task summaries for display.
func renderTaskList(tasks []TaskSummary) string {
	lines := make([]string, len(tasks))
	for i, t := range tasks {
		owner := ""
		if t.Owner != "" {
			owner = fmt.Sprintf(" (%s)", t.Owner)
		}
		blocked := ""
		if len(t.BlockedBy) > 0 {
			parts := make([]string, len(t.BlockedBy))
			for j, id := range t.BlockedBy {
				parts[j] = "#" + id
			}
			blocked = fmt.Sprintf(" [blocked by %s]", strings.Join(parts, ", "))
		}
		lines[i] = fmt.Sprintf("#%s [%s] %s%s%s", t.ID, t.Status, t.Subject, owner, blocked)
	}
	return strings.Join(lines, "\n")
}
