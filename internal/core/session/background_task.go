package session

// BackgroundTaskStatus identifies the minimum lifecycle state surfaced by `/tasks`.
type BackgroundTaskStatus string

const (
	// BackgroundTaskStatusRunning marks one task that is actively executing.
	BackgroundTaskStatusRunning BackgroundTaskStatus = "running"
	// BackgroundTaskStatusPending marks one task that is queued or waiting.
	BackgroundTaskStatusPending BackgroundTaskStatus = "pending"
	// BackgroundTaskStatusCompleted marks one task that finished successfully.
	BackgroundTaskStatusCompleted BackgroundTaskStatus = "completed"
	// BackgroundTaskStatusFailed marks one task that exited with a failure status.
	BackgroundTaskStatusFailed BackgroundTaskStatus = "failed"
	// BackgroundTaskStatusStopped marks one task that was explicitly stopped by the host.
	BackgroundTaskStatusStopped BackgroundTaskStatus = "stopped"
)

// BackgroundTaskSnapshot carries the minimum read-only task summary shared with `/tasks`.
type BackgroundTaskSnapshot struct {
	// ID identifies one background task instance.
	ID string
	// Type stores the normalized task kind such as shell, agent, or remote_agent.
	Type string
	// Status stores the user-visible lifecycle state.
	Status BackgroundTaskStatus
	// Summary stores the minimum human-readable label shown in task listings.
	Summary string
	// ControlsAvailable reports whether stop/resume controls are currently implemented.
	ControlsAvailable bool
}

// CloneBackgroundTaskSnapshots returns a detached copy of one task snapshot slice.
func CloneBackgroundTaskSnapshots(tasks []BackgroundTaskSnapshot) []BackgroundTaskSnapshot {
	if len(tasks) == 0 {
		return nil
	}

	cloned := make([]BackgroundTaskSnapshot, len(tasks))
	copy(cloned, tasks)
	return cloned
}
