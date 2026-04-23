package remote

import (
	"sync"
	"time"
)

// RemoteSubagentStateProvider exposes known subagent states for /session reporting.
type RemoteSubagentStateProvider interface {
	// SubagentCount returns the number of known subagents.
	SubagentCount() int
	// SubagentList returns a snapshot of all known subagent states.
	SubagentList() []SubagentStateView
}

// SubagentStateView exposes one subagent's observable state.
type SubagentStateView struct {
	AgentID    string
	AgentType  string
	Status     string
	EventCount int
}

// SubagentRegistry tracks known subagent states keyed by agent ID.
// It is safe for concurrent use.
type SubagentRegistry struct {
	mu     sync.RWMutex
	states map[string]*SubagentState
}

// NewSubagentRegistry creates an empty subagent registry.
func NewSubagentRegistry() *SubagentRegistry {
	return &SubagentRegistry{
		states: make(map[string]*SubagentState),
	}
}

// UpdateFromEvent updates the registry state based on one internal event.
// If the event has no agent_id, it is ignored.
func (r *SubagentRegistry) UpdateFromEvent(evt InternalEvent) {
	if !evt.IsSubagentEvent() {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.states[evt.AgentID]
	if !ok {
		state = &SubagentState{
			AgentID: evt.AgentID,
			Status:  "active",
		}
		r.states[evt.AgentID] = state
	}

	state.EventCount++
	state.LastEventAt = time.Now()

	// Infer agent type from payload when available.
	if agentType, _ := evt.Payload["agent_type"].(string); agentType != "" {
		state.AgentType = agentType
	}

	// Infer status transitions from event type hints.
	switch evt.EventType {
	case "subagent_stopped", "agent_stopped":
		state.Status = "stopped"
	case "subagent_error", "agent_error":
		state.Status = "error"
	case "subagent_started", "agent_started":
		state.Status = "active"
	}
}

// Get returns the state for one subagent, or nil if unknown.
func (r *SubagentRegistry) Get(agentID string) *SubagentState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.states[agentID]
}

// List returns a snapshot of all known subagent states.
func (r *SubagentRegistry) List() []*SubagentState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*SubagentState, 0, len(r.states))
	for _, s := range r.states {
		result = append(result, s)
	}
	return result
}

// Count returns the number of known subagents.
func (r *SubagentRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.states)
}

// Remove deletes one subagent from the registry.
func (r *SubagentRegistry) Remove(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.states, agentID)
}

// Clear removes all tracked subagents.
func (r *SubagentRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states = make(map[string]*SubagentState)
}

// GroupInternalEventsByAgent partitions a slice of internal events by agent_id.
// Events without an agent_id are placed under the empty string key.
func GroupInternalEventsByAgent(events []InternalEvent) map[string][]InternalEvent {
	groups := make(map[string][]InternalEvent)
	for _, evt := range events {
		key := evt.AgentID
		groups[key] = append(groups[key], evt)
	}
	return groups
}
