package task_get

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
	coretask "github.com/sheepzhao/claude-code-go/internal/core/task"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

const (
	// Name is the stable registry identifier used by the TaskGet tool.
	Name = "TaskGet"
)

// TaskGetter describes the minimum store capability consumed by the get tool.
type TaskGetter interface {
	Get(ctx context.Context, id string) (*coretask.Task, error)
}

// Tool retrieves a single task by its ID.
type Tool struct {
	store TaskGetter
}

// NewTool constructs a TaskGet tool backed by the given store.
func NewTool(store TaskGetter) *Tool {
	return &Tool{store: store}
}

// Input stores the typed request payload accepted by the TaskGet tool.
type Input struct {
	// TaskID identifies the task to retrieve (required).
	TaskID string `json:"taskId"`
}

// Output stores the structured result returned when a task is retrieved.
type Output struct {
	// Task holds the full task data, or nil when not found.
	Task *TaskDetail `json:"task"`
}

// TaskDetail is the full representation of a task returned by TaskGet.
type TaskDetail struct {
	ID          string   `json:"id"`
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Blocks      []string `json:"blocks"`
	BlockedBy   []string `json:"blockedBy"`
}

// Name returns the stable registration name for the TaskGet tool.
func (t *Tool) Name() string {
	return Name
}

// Description returns the summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return "Use this tool to retrieve a task by ID."
}

// InputSchema returns the input contract for the TaskGet tool.
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"taskId": {
				Type:        coretool.ValueKindString,
				Description: "The ID of the task to retrieve.",
				Required:    true,
			},
		},
	}
}

// IsReadOnly reports that getting a task does not mutate state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that get requests are safe alongside other tools.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// IsEnabled reports whether the TodoV2 feature flag allows this tool to be
// exposed to the provider tool catalog.
func (t *Tool) IsEnabled() bool {
	return featureflag.IsTodoV2Enabled()
}

// Invoke validates input, retrieves the task, and returns the result.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("task get tool: nil receiver")
	}
	if t.store == nil {
		return coretool.Result{Error: "Task list is not available in Claude Code Go yet."}, nil
	}

	input, err := coretool.DecodeInput[Input](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	if strings.TrimSpace(input.TaskID) == "" {
		return coretool.Result{Error: "taskId is required"}, nil
	}

	task, err := t.store.Get(ctx, input.TaskID)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	if task == nil {
		return coretool.Result{
			Output: "Task not found",
			Meta:   map[string]any{"data": Output{Task: nil}},
		}, nil
	}

	detail := &TaskDetail{
		ID:          task.ID,
		Subject:     task.Subject,
		Description: task.Description,
		Status:      string(task.Status),
		Blocks:      task.Blocks,
		BlockedBy:   task.BlockedBy,
	}

	return coretool.Result{
		Output: renderTaskDetail(detail),
		Meta:   map[string]any{"data": Output{Task: detail}},
	}, nil
}

// renderTaskDetail formats a task for display in tool results.
func renderTaskDetail(t *TaskDetail) string {
	lines := []string{
		fmt.Sprintf("Task #%s: %s", t.ID, t.Subject),
		fmt.Sprintf("Status: %s", t.Status),
		fmt.Sprintf("Description: %s", t.Description),
	}
	if len(t.BlockedBy) > 0 {
		lines = append(lines, fmt.Sprintf("Blocked by: %s", formatIDList(t.BlockedBy)))
	}
	if len(t.Blocks) > 0 {
		lines = append(lines, fmt.Sprintf("Blocks: %s", formatIDList(t.Blocks)))
	}
	return strings.Join(lines, "\n")
}

// formatIDList renders a list of task IDs as "#1, #2, #3".
func formatIDList(ids []string) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = "#" + id
	}
	return strings.Join(parts, ", ")
}
