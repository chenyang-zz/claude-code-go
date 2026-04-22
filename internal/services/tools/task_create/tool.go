package task_create

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	coretask "github.com/sheepzhao/claude-code-go/internal/core/task"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	runtimehooks "github.com/sheepzhao/claude-code-go/internal/runtime/hooks"
)

const (
	// Name is the stable registry identifier used by the TaskCreate tool.
	Name = "TaskCreate"
)

// TaskCreator describes the minimum store capability consumed by the create tool.
type TaskCreator interface {
	Create(ctx context.Context, data coretask.NewTask) (string, error)
	// Delete removes a task by ID (used for TaskCreated hook rollback).
	Delete(ctx context.Context, id string) (bool, error)
}

// HookDispatcher dispatches hook events for task lifecycle hooks.
type HookDispatcher interface {
	// RunHooks executes hooks for the given event and returns results.
	// Returns nil when no hooks are configured.
	RunHooks(ctx context.Context, event hook.HookEvent, input any, cwd string) []hook.HookResult
}

// Tool creates a new task in the persistent task list.
type Tool struct {
	store           TaskCreator
	hooks           HookDispatcher
	hookCfg         hook.HooksConfig
	disableAllHooks bool
}

// NewTool constructs a TaskCreate tool backed by the given store.
func NewTool(store TaskCreator) *Tool {
	return &Tool{store: store}
}

// NewToolWithHooks constructs a TaskCreate tool with hook dispatch capability.
func NewToolWithHooks(store TaskCreator, dispatcher HookDispatcher, hookCfg hook.HooksConfig, disableAllHooks bool) *Tool {
	return &Tool{store: store, hooks: dispatcher, hookCfg: hookCfg, disableAllHooks: disableAllHooks}
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

// IsEnabled reports whether the TodoV2 feature flag allows this tool to be
// exposed to the provider tool catalog.
func (t *Tool) IsEnabled() bool {
	return featureflag.IsTodoV2Enabled()
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

	// TaskCreated hooks: run after task creation, blocking triggers rollback.
	if t.shouldRunTaskHooks(hook.EventTaskCreated) {
		hookInput := hook.TaskHookInput{
			BaseHookInput: hook.BaseHookInput{
				CWD: call.Context.WorkingDir,
			},
			HookEventName:   string(hook.EventTaskCreated),
			TaskID:          id,
			TaskSubject:     input.Subject,
			TaskDescription: input.Description,
		}
		results := t.hooks.RunHooks(ctx, hook.EventTaskCreated, hookInput, call.Context.WorkingDir)
		if runtimehooks.HasBlockingResult(results) {
			// Rollback: delete the newly created task.
			_, _ = t.store.Delete(ctx, id)
			errs := runtimehooks.BlockingErrors(results)
			return coretool.Result{Error: strings.Join(errs, "\n")}, nil
		}
	}

	var output Output
	output.Task.ID = id
	output.Task.Subject = input.Subject

	return coretool.Result{
		Output: fmt.Sprintf("Task #%s created successfully: %s", id, input.Subject),
		Meta:   map[string]any{"data": output},
	}, nil
}

// shouldRunTaskHooks reports whether task lifecycle hooks should execute.
func (t *Tool) shouldRunTaskHooks(event hook.HookEvent) bool {
	if t.hooks == nil || t.hookCfg == nil {
		return false
	}
	if t.disableAllHooks {
		return false
	}
	return t.hookCfg.HasEvent(event)
}
