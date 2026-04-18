package task_create

import (
	"context"
	"fmt"

	coretask "github.com/sheepzhao/claude-code-go/internal/core/task"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

const (
	// Name is the stable registry identifier used by the TaskCreate tool.
	Name = "TaskCreate"
)

// TaskCreator describes the minimum store capability consumed by the create tool.
type TaskCreator interface {
	Create(ctx context.Context, data coretask.NewTask) (string, error)
}

// Tool creates a new task in the persistent task list.
type Tool struct {
	store TaskCreator
}

// NewTool constructs a TaskCreate tool backed by the given store.
func NewTool(store TaskCreator) *Tool {
	return &Tool{store: store}
}

// Input stores the typed request payload accepted by the TaskCreate tool.
type Input struct {
	// Subject is a brief title for the task (required).
	Subject string `json:"subject"`
	// Description explains what needs to be done.
	Description string `json:"description"`
	// ActiveForm is the present-continuous label shown in spinners (e.g. "Running tests").
	ActiveForm string `json:"activeForm,omitempty"`
	// Metadata stores arbitrary key-value pairs to attach to the task.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Output stores the structured result returned when a task is created.
type Output struct {
	// Task holds the created task summary.
	Task struct {
		// ID is the auto-generated monotonic identifier.
		ID string `json:"id"`
		// Subject echoes the provided task title.
		Subject string `json:"subject"`
	} `json:"task"`
}

// Name returns the stable registration name for the TaskCreate tool.
func (t *Tool) Name() string {
	return Name
}

// Description returns the summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return "Use this tool to create a task in the task list."
}

// InputSchema returns the input contract for the TaskCreate tool.
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"subject": {
				Type:        coretool.ValueKindString,
				Description: "A brief title for the task.",
				Required:    true,
			},
			"description": {
				Type:        coretool.ValueKindString,
				Description: "What needs to be done.",
				Required:    true,
			},
			"activeForm": {
				Type:        coretool.ValueKindString,
				Description: `Present continuous form shown in spinner when in_progress (e.g., "Running tests").`,
			},
			"metadata": {
				Type:        coretool.ValueKindObject,
				Description: "Arbitrary metadata to attach to the task.",
			},
		},
	}
}

// IsReadOnly reports that creating a task mutates state.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that create requests are safe alongside other tools.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// Invoke validates input, creates a new task, and returns the result.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("task create tool: nil receiver")
	}
	if t.store == nil {
		return coretool.Result{Error: "Task list is not available in Claude Code Go yet."}, nil
	}

	input, err := coretool.DecodeInput[Input](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	if input.Subject == "" {
		return coretool.Result{Error: "subject is required"}, nil
	}
	if input.Description == "" {
		return coretool.Result{Error: "description is required"}, nil
	}

	id, err := t.store.Create(ctx, coretask.NewTask{
		Subject:     input.Subject,
		Description: input.Description,
		ActiveForm:  input.ActiveForm,
		Metadata:    input.Metadata,
	})
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	var output Output
	output.Task.ID = id
	output.Task.Subject = input.Subject

	return coretool.Result{
		Output: fmt.Sprintf("Task #%s created successfully: %s", id, input.Subject),
		Meta:   map[string]any{"data": output},
	}, nil
}
