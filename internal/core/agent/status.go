package agent

// Status holds the runtime status of an agent within a team,
// derived from task ownership.
type Status struct {
	// AgentID is the unique identifier of the agent.
	AgentID string
	// Name is the human-readable display name.
	Name string
	// AgentType is the agent type identifier.
	AgentType string
	// Status indicates whether the agent is idle or busy.
	Status StatusType
	// CurrentTasks lists the task IDs this agent currently owns.
	CurrentTasks []string
}
