package task_update

import (
	"context"
	"fmt"
	"strings"

	coretask "github.com/sheepzhao/claude-code-go/internal/core/task"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

const (
	// Name is the stable registry identifier used by the TaskUpdate tool.
	Name = "TaskUpdate"
)

// StatusDeleted is the special status value that triggers task deletion.
const StatusDeleted = "deleted"

// TaskUpdater describes the minimum store capability consumed by the update tool.
type TaskUpdater interface {
	Get(ctx context.Context, id string) (*coretask.Task, error)
	UpdateWithDependencies(ctx context.Context, taskID string, updates coretask.Updates, addBlocks []string, addBlockedBy []string) (*coretask.Task, error)
	Delete(ctx context.Context, id string) (bool, error)
}

// Tool updates an existing task or deletes it when status is "deleted".
type Tool struct {
	store TaskUpdater
}

// NewTool constructs a TaskUpdate tool backed by the given store.
func NewTool(store TaskUpdater) *Tool {
	return &Tool{store: store}
}

// Input stores the typed request payload accepted by the TaskUpdate tool.
type Input struct {
	// TaskID identifies the task to update (required).
	TaskID string `json:"taskId"`
	// Subject is an optional new title for the task.
	Subject *string `json:"subject,omitempty"`
	// Description is an optional new explanation.
	Description *string `json:"description,omitempty"`
	// ActiveForm is an optional new present-continuous spinner label.
	ActiveForm *string `json:"activeForm,omitempty"`
	// Status is an optional new lifecycle state. Use "deleted" to remove the task.
	Status *string `json:"status,omitempty"`
	// Owner is an optional new agent/user identifier.
	Owner *string `json:"owner,omitempty"`
	// AddBlocks lists task IDs that this task should block (not already present).
	AddBlocks []string `json:"addBlocks,omitempty"`
	// AddBlockedBy lists task IDs that should block this task (not already present).
	AddBlockedBy []string `json:"addBlockedBy,omitempty"`
	// Metadata stores keys to merge. Set a key to null to delete it.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Output stores the structured result returned when a task is updated.
type Output struct {
	// Success reports whether the update was applied.
	Success bool `json:"success"`
	// TaskID echoes the target task identifier.
	TaskID string `json:"taskId"`
	// UpdatedFields lists the names of fields that were changed.
	UpdatedFields []string `json:"updatedFields"`
	// Error describes why the update failed, if applicable.
	Error string `json:"error,omitempty"`
	// StatusChange records the before/after status when the status field changed.
	StatusChange *StatusChange `json:"statusChange,omitempty"`
}

// StatusChange records the before and after status when a task status transitions.
type StatusChange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Name returns the stable registration name for the TaskUpdate tool.
func (t *Tool) Name() string {
	return Name
}

// Description returns the summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return "Use this tool to update a task in the task list."
}

// InputSchema returns the input contract for the TaskUpdate tool.
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"taskId": {
				Type:        coretool.ValueKindString,
				Description: "The ID of the task to update.",
				Required:    true,
			},
			"subject": {
				Type:        coretool.ValueKindString,
				Description: "New subject for the task.",
			},
			"description": {
				Type:        coretool.ValueKindString,
				Description: "New description for the task.",
			},
			"activeForm": {
				Type:        coretool.ValueKindString,
				Description: `Present continuous form shown in spinner when in_progress (e.g., "Running tests").`,
			},
			"status": {
				Type:        coretool.ValueKindString,
				Description: "New status for the task. Use \"deleted\" to delete the task.",
			},
			"owner": {
				Type:        coretool.ValueKindString,
				Description: "New owner for the task.",
			},
			"addBlocks": {
				Type:        coretool.ValueKindArray,
				Description: "Task IDs that this task blocks.",
			},
			"addBlockedBy": {
				Type:        coretool.ValueKindArray,
				Description: "Task IDs that block this task.",
			},
			"metadata": {
				Type:        coretool.ValueKindObject,
				Description: "Metadata keys to merge into the task. Set a key to null to delete it.",
			},
		},
	}
}

// IsReadOnly reports that updating a task mutates state.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that update requests are safe alongside other tools.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// Invoke validates input, updates or deletes the task, and returns the result.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("task update tool: nil receiver")
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

	// Check existence.
	existing, err := t.store.Get(ctx, input.TaskID)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}
	if existing == nil {
		return errorResult(input.TaskID, "Task not found"), nil
	}

	// Handle deletion.
	if input.Status != nil && *input.Status == StatusDeleted {
		deleted, err := t.store.Delete(ctx, input.TaskID)
		if err != nil {
			return errorResult(input.TaskID, "Failed to delete task"), nil
		}
		if deleted {
			return coretool.Result{
				Output: fmt.Sprintf("Deleted task #%s", input.TaskID),
				Meta: map[string]any{"data": Output{
					Success:       true,
					TaskID:        input.TaskID,
					UpdatedFields: []string{"deleted"},
					StatusChange:  &StatusChange{From: string(existing.Status), To: StatusDeleted},
				}},
			}, nil
		}
		return errorResult(input.TaskID, "Failed to delete task"), nil
	}

	// Build updates.
	updates := coretask.Updates{}
	var updatedFields []string

	if input.Subject != nil && *input.Subject != existing.Subject {
		updates.Subject = input.Subject
		updatedFields = append(updatedFields, "subject")
	}
	if input.Description != nil && *input.Description != existing.Description {
		updates.Description = input.Description
		updatedFields = append(updatedFields, "description")
	}
	if input.ActiveForm != nil && *input.ActiveForm != existing.ActiveForm {
		updates.ActiveForm = input.ActiveForm
		updatedFields = append(updatedFields, "activeForm")
	}
	if input.Owner != nil && *input.Owner != existing.Owner {
		updates.Owner = input.Owner
		updatedFields = append(updatedFields, "owner")
	}
	if input.Status != nil {
		newStatus := coretask.Status(*input.Status)
		if !coretask.IsValidStatus(newStatus) {
			return coretool.Result{Error: fmt.Sprintf("invalid status %q: must be one of pending, in_progress, completed", *input.Status)}, nil
		}
		if newStatus != existing.Status {
			updates.Status = &newStatus
			updatedFields = append(updatedFields, "status")
		}
	}
	if input.Metadata != nil {
		updates.Metadata = input.Metadata
		updatedFields = append(updatedFields, "metadata")
	}

	// Deduplicate addBlocks/addBlockedBy input and filter already-existing entries.
	newBlocks := filterNew(existing.Blocks, dedupe(input.AddBlocks))
	for _, blockID := range newBlocks {
		target, err := t.store.Get(ctx, blockID)
		if err != nil {
			return coretool.Result{Error: err.Error()}, nil
		}
		if target == nil {
			return errorResult(input.TaskID, fmt.Sprintf("failed to add block %s: referenced task not found", blockID)), nil
		}
	}

	newBlockedBy := filterNew(existing.BlockedBy, dedupe(input.AddBlockedBy))
	for _, blockerID := range newBlockedBy {
		target, err := t.store.Get(ctx, blockerID)
		if err != nil {
			return coretool.Result{Error: err.Error()}, nil
		}
		if target == nil {
			return errorResult(input.TaskID, fmt.Sprintf("failed to add blockedBy %s: referenced task not found", blockerID)), nil
		}
	}

	// Include block relationships in the same atomic update.
	if len(newBlocks) > 0 {
		merged := make([]string, len(existing.Blocks))
		copy(merged, existing.Blocks)
		merged = append(merged, newBlocks...)
		updates.Blocks = &merged
		updatedFields = append(updatedFields, "blocks")
	}
	if len(newBlockedBy) > 0 {
		merged := make([]string, len(existing.BlockedBy))
		copy(merged, existing.BlockedBy)
		merged = append(merged, newBlockedBy...)
		updates.BlockedBy = &merged
		updatedFields = append(updatedFields, "blockedBy")
	}

	// Apply all updates and reverse dependency mutations under a single store lock.
	if len(updatedFields) > 0 {
		updated, err := t.store.UpdateWithDependencies(ctx, input.TaskID, updates, newBlocks, newBlockedBy)
		if err != nil {
			return coretool.Result{Error: err.Error()}, nil
		}
		if updated == nil {
			return errorResult(input.TaskID, "Task not found"), nil
		}
	}

	// Build status change info.
	var statusChange *StatusChange
	if updates.Status != nil {
		statusChange = &StatusChange{
			From: string(existing.Status),
			To:   string(*updates.Status),
		}
	}

	result := Output{
		Success:       true,
		TaskID:        input.TaskID,
		UpdatedFields: updatedFields,
		StatusChange:  statusChange,
	}

	return coretool.Result{
		Output: fmt.Sprintf("Updated task #%s %s", input.TaskID, strings.Join(updatedFields, ", ")),
		Meta:   map[string]any{"data": result},
	}, nil
}

// errorResult returns a non-error tool result indicating failure.
func errorResult(taskID, errMsg string) coretool.Result {
	return coretool.Result{
		Output: errMsg,
		Meta: map[string]any{"data": Output{
			Success: false,
			TaskID:  taskID,
			Error:   errMsg,
		}},
	}
}

// dedupe removes duplicate strings from a slice while preserving order.
func dedupe(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// filterNew returns items from candidates that are not already in existing.
func filterNew(existing []string, candidates []string) []string {
	set := make(map[string]bool, len(existing))
	for _, id := range existing {
		set[id] = true
	}
	var result []string
	for _, id := range candidates {
		if !set[id] {
			result = append(result, id)
		}
	}
	return result
}
