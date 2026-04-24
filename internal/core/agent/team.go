package agent

// Team represents a group of agents working together.
type Team struct {
	// ID is the unique identifier for the team.
	ID string
	// Name is the human-readable team name (also used as taskListId).
	Name string
	// Members are the agents participating in this team.
	Members []Agent
	// LeadAgentID identifies the lead agent of the team.
	LeadAgentID string
	// TaskListID is the identifier used for task list operations.
	TaskListID string
}
