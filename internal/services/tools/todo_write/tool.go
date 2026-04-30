package todo_write

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/task"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const Name = "TodoWrite"

const guidanceMessage = "The TodoWrite tool is no longer active. Use the TodoV2 tools instead:\n" +
	"- TodoCreate: create new tasks\n" +
	"- TodoGet: read task details\n" +
	"- TodoList: list all tasks\n" +
	"- TodoUpdate: update task fields (status, owner, dependencies)\n\n" +
	"Your current tasks are returned below in the old format for reference."

// Tool provides backward compatibility for the legacy TodoWrite tool by
// bridging reads to the TodoV2 task store. Write operations are no-ops;
// models should use the TodoV2 tool family for mutations.
type Tool struct {
	store task.Store
}

// NewTool creates a TodoWrite compatibility bridge backed by the given task store.
func NewTool(store task.Store) *Tool {
	return &Tool{store: store}
}

// Name returns the stable tool identifier.
func (t *Tool) Name() string {
	return Name
}

// Description returns a summary directing models to TodoV2 tools.
func (t *Tool) Description() string {
	return "Legacy todo management tool. Use the TodoV2 family (TodoCreate, TodoGet, TodoList, TodoUpdate) instead for full todo management. This tool reads your current todo list in the legacy format for backward compatibility."
}

// IsReadOnly returns false — although writes are no-ops, the tool accepts write-shaped input.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that the tool can be invoked in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// InputSchema declares the legacy TodoWrite input contract.
func (t *Tool) InputSchema() tool.InputSchema {
	return tool.InputSchema{
		Properties: map[string]tool.FieldSchema{
			"todos": {
				Type:        tool.ValueKindArray,
				Description: "The updated todo list (legacy format).",
				Required:    true,
				Items: &tool.FieldSchema{
					Type: tool.ValueKindObject,
				},
			},
		},
	}
}

// todoWriteInput is the legacy input shape accepted by the old TodoWrite tool.
type todoWriteInput struct {
	Todos []todoItem `json:"todos"`
}

// todoItem is the legacy per-item format.
type todoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm"`
}

// todoWriteOutput mirrors the legacy output schema.
type todoWriteOutput struct {
	OldTodos []todoItem `json:"oldTodos"`
	NewTodos []todoItem `json:"newTodos"`
	Message  string     `json:"message"`
}

// Invoke reads the current tasks from the TodoV2 store, maps them to the legacy
// format, and returns them as both oldTodos and newTodos. It does not apply any
// mutations — models should use the TodoV2 tool family for writes.
func (t *Tool) Invoke(ctx context.Context, call tool.Call) (tool.Result, error) {
	input, err := tool.DecodeInput[todoWriteInput](t.InputSchema(), call.Input)
	if err != nil {
		return tool.Result{Error: err.Error()}, nil
	}

	currentTasks, err := t.store.List(ctx)
	if err != nil {
		logger.DebugCF("todo_write", "failed to read tasks from store", map[string]any{
			"error": err.Error(),
		})
		return tool.Result{Error: fmt.Sprintf("failed to read current tasks: %s", err.Error())}, nil
	}

	// Map TodoV2 tasks to legacy format.
	mapped := make([]todoItem, 0, len(currentTasks))
	for _, t := range currentTasks {
		mapped = append(mapped, todoItem{
			Content:    t.Subject,
			Status:     string(t.Status),
			ActiveForm: t.ActiveForm,
		})
	}

	output := todoWriteOutput{
		OldTodos: mapped,
		NewTodos: input.Todos, // Return submitted todos so old scripts don't break
		Message:  guidanceMessage,
	}

	raw, _ := json.Marshal(output)
	logger.DebugCF("todo_write", "bridged legacy TodoWrite call to TodoV2 store", map[string]any{
		"submitted_todos": len(input.Todos),
		"current_tasks":   len(currentTasks),
	})

	return tool.Result{
		Output: string(raw),
		Meta:   map[string]any{"data": output},
	}, nil
}
