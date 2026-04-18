package task_stop

import (
	"context"
	"fmt"
	"strings"

	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

const (
	// Name is the stable registry identifier used by the migrated task stop tool.
	Name = "TaskStop"
)

// BackgroundTaskStopper describes the minimum shared task-store stop capability consumed by the stop tool.
type BackgroundTaskStopper interface {
	// Stop requests termination of one running background task and returns the final visible snapshot.
	Stop(id string) (coresession.BackgroundTaskSnapshot, error)
}

// Tool stops one running background task by ID.
type Tool struct {
	// taskStore provides the shared task-stop capability used by the tool.
	taskStore BackgroundTaskStopper
}

// Input stores the typed request payload accepted by the migrated task stop tool.
type Input struct {
	// TaskID identifies the running background task that should be stopped.
	TaskID string `json:"task_id"`
}

// Output stores the structured result metadata returned when one background task is stopped.
type Output struct {
	// TaskID echoes the stopped task identifier.
	TaskID string `json:"taskId"`
	// TaskType stores the normalized task kind such as bash.
	TaskType string `json:"taskType"`
	// Summary stores the minimum user-visible task label.
	Summary string `json:"summary"`
}

// NewTool constructs a task stop tool backed by one shared background-task store.
func NewTool(taskStore BackgroundTaskStopper) *Tool {
	return &Tool{taskStore: taskStore}
}

// Name returns the stable registration name for the migrated task stop tool.
func (t *Tool) Name() string {
	return Name
}

// Description returns the summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return "Stop a running background task by ID."
}

// InputSchema returns the minimum source-aligned task stop input contract used by the Go host.
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"task_id": {
				Type:        coretool.ValueKindString,
				Description: "The ID of the running background task to stop.",
				Required:    true,
			},
		},
	}
}

// IsReadOnly reports that stopping a task mutates runtime state.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that stop requests are safe to invoke alongside other tools.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// Invoke validates input, stops the selected task, and returns one stable status message.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	_ = ctx

	if t == nil {
		return coretool.Result{}, fmt.Errorf("task stop tool: nil receiver")
	}
	if t.taskStore == nil {
		return coretool.Result{Error: "Background task stop is not available in Claude Code Go yet."}, nil
	}

	input, err := coretool.DecodeInput[Input](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	taskID := strings.TrimSpace(input.TaskID)
	if taskID == "" {
		return coretool.Result{Error: "task_id is required"}, nil
	}

	snapshot, err := t.taskStore.Stop(taskID)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	output := Output{
		TaskID:   snapshot.ID,
		TaskType: snapshot.Type,
		Summary:  snapshot.Summary,
	}
	return coretool.Result{
		Output: renderStopSuccess(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// renderStopSuccess converts one stopped task result into one stable caller-facing text payload.
func renderStopSuccess(output Output) string {
	if output.Summary == "" {
		return fmt.Sprintf("Stopped background task: %s", output.TaskID)
	}
	return fmt.Sprintf("Stopped background task: %s (%s)", output.TaskID, output.Summary)
}
