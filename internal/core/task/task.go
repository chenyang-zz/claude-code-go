// Package task defines the data model and file-persisted storage for the
// TodoV2 task list system. Tasks represent collaborative work items that AI
// agents can create, query, update, and delete. Each task is stored as an
// individual JSON file on disk with a monotonic ID generated from a high-water
// mark file, protected by flock for multi-process safety.
package task

// Status represents the lifecycle state of a task.
type Status string

const (
	// StatusPending indicates the task has been created but work has not started.
	StatusPending Status = "pending"
	// StatusInProgress indicates the task is actively being worked on.
	StatusInProgress Status = "in_progress"
	// StatusCompleted indicates the task has been finished.
	StatusCompleted Status = "completed"
)

// validStatuses is the set of allowed Status values used for validation.
var validStatuses = map[Status]bool{
	StatusPending:    true,
	StatusInProgress: true,
	StatusCompleted:  true,
}

// IsValidStatus reports whether s is a recognized task status value.
func IsValidStatus(s Status) bool {
	return validStatuses[s]
}

// Task is the persisted representation of a single collaborative work item.
type Task struct {
	// ID is the unique numeric identifier assigned at creation (monotonically increasing).
	ID string `json:"id"`
	// Subject is a brief title for the task.
	Subject string `json:"subject"`
	// Description is a longer explanation of what needs to be done.
	Description string `json:"description"`
	// ActiveForm is the present-continuous label shown in spinners when the task is in_progress (e.g. "Running tests").
	ActiveForm string `json:"activeForm,omitempty"`
	// Owner is the agent or user identifier currently responsible for the task.
	Owner string `json:"owner,omitempty"`
	// Status is the current lifecycle state of the task.
	Status Status `json:"status"`
	// Blocks holds task IDs that this task blocks (this task must complete before they can proceed).
	Blocks []string `json:"blocks"`
	// BlockedBy holds task IDs that block this task (they must complete before this task can proceed).
	BlockedBy []string `json:"blockedBy"`
	// Metadata stores arbitrary key-value pairs attached to the task. Keys prefixed with
	// underscore (e.g. "_internal") are reserved for system use and filtered from user-visible listings.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Summary returns a lightweight snapshot of the task suitable for list views.
type Summary struct {
	ID        string   `json:"id"`
	Subject   string   `json:"subject"`
	Status    Status   `json:"status"`
	Owner     string   `json:"owner,omitempty"`
	BlockedBy []string `json:"blockedBy"`
}

// ToSummary produces a compact summary from the full task, suitable for list
// rendering. The blockedBy list is returned as-is; callers should filter out
// completed task IDs based on the full task set before presenting to users.
func (t *Task) ToSummary() Summary {
	return Summary{
		ID:        t.ID,
		Subject:   t.Subject,
		Status:    t.Status,
		Owner:     t.Owner,
		BlockedBy: t.BlockedBy,
	}
}

// IsInternal reports whether the task carries the _internal metadata flag,
// indicating it should be hidden from user-facing listings.
func (t *Task) IsInternal() bool {
	if t.Metadata == nil {
		return false
	}
	v, ok := t.Metadata["_internal"]
	return ok && v == true
}
