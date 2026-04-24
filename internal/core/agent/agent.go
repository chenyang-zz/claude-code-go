package agent

import "time"

// StatusType indicates the current operational state of an agent.
type StatusType string

const (
	// StatusIdle means the agent has no active tasks.
	StatusIdle StatusType = "idle"
	// StatusBusy means the agent owns at least one unresolved task.
	StatusBusy StatusType = "busy"
)

// Agent represents a participant agent in a team or a running agent instance.
// It carries runtime identity and state, distinct from the static Definition.
type Agent struct {
	// ID is the unique identifier for this agent instance or team member.
	ID string
	// Name is the human-readable display name.
	Name string
	// Type is the agent type identifier (e.g., "explore", "verify", "plan").
	Type string
	// Status indicates whether the agent is idle or busy.
	Status StatusType
	// CreatedAt records when the agent was instantiated.
	CreatedAt time.Time
}
