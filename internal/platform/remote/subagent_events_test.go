package remote

import (
	"testing"
	"time"
)

func TestSubagentRegistry_UpdateFromEvent(t *testing.T) {
	r := NewSubagentRegistry()

	// First event for agent-a
	r.UpdateFromEvent(InternalEvent{AgentID: "agent-a", EventType: "test", Payload: map[string]any{"agent_type": "general-purpose"}})
	state := r.Get("agent-a")
	if state == nil {
		t.Fatal("expected state for agent-a")
	}
	if state.AgentID != "agent-a" {
		t.Errorf("expected agent-a, got %s", state.AgentID)
	}
	if state.AgentType != "general-purpose" {
		t.Errorf("expected general-purpose, got %s", state.AgentType)
	}
	if state.Status != "active" {
		t.Errorf("expected active, got %s", state.Status)
	}
	if state.EventCount != 1 {
		t.Errorf("expected event count 1, got %d", state.EventCount)
	}

	// Second event for agent-a
	r.UpdateFromEvent(InternalEvent{AgentID: "agent-a", EventType: "test"})
	state = r.Get("agent-a")
	if state.EventCount != 2 {
		t.Errorf("expected event count 2, got %d", state.EventCount)
	}

	// Event without agent_id is ignored
	r.UpdateFromEvent(InternalEvent{EventType: "test"})
	if r.Count() != 1 {
		t.Errorf("expected count 1, got %d", r.Count())
	}
}

func TestSubagentRegistry_StatusTransitions(t *testing.T) {
	r := NewSubagentRegistry()

	tests := []struct {
		EventType    string
		wantStatus   string
	}{
		{"subagent_started", "active"},
		{"agent_started", "active"},
		{"subagent_stopped", "stopped"},
		{"agent_stopped", "stopped"},
		{"subagent_error", "error"},
		{"agent_error", "error"},
	}

	for _, tt := range tests {
		t.Run(tt.EventType, func(t *testing.T) {
			r.Clear()
			r.UpdateFromEvent(InternalEvent{AgentID: "agent-1", EventType: tt.EventType})
			state := r.Get("agent-1")
			if state == nil {
				t.Fatal("expected state")
			}
			if state.Status != tt.wantStatus {
				t.Errorf("status = %s, want %s", state.Status, tt.wantStatus)
			}
		})
	}
}

func TestSubagentRegistry_List(t *testing.T) {
	r := NewSubagentRegistry()
	r.UpdateFromEvent(InternalEvent{AgentID: "agent-a"})
	r.UpdateFromEvent(InternalEvent{AgentID: "agent-b"})

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 states, got %d", len(list))
	}

	ids := make(map[string]bool)
	for _, s := range list {
		ids[s.AgentID] = true
	}
	if !ids["agent-a"] || !ids["agent-b"] {
		t.Errorf("expected both agents in list")
	}
}

func TestSubagentRegistry_Remove(t *testing.T) {
	r := NewSubagentRegistry()
	r.UpdateFromEvent(InternalEvent{AgentID: "agent-a"})
	if r.Count() != 1 {
		t.Fatalf("expected count 1")
	}
	r.Remove("agent-a")
	if r.Count() != 0 {
		t.Errorf("expected count 0 after remove, got %d", r.Count())
	}
}

func TestSubagentRegistry_Clear(t *testing.T) {
	r := NewSubagentRegistry()
	r.UpdateFromEvent(InternalEvent{AgentID: "agent-a"})
	r.UpdateFromEvent(InternalEvent{AgentID: "agent-b"})
	r.Clear()
	if r.Count() != 0 {
		t.Errorf("expected count 0 after clear, got %d", r.Count())
	}
}

func TestSubagentRegistry_Concurrent(t *testing.T) {
	r := NewSubagentRegistry()
	done := make(chan struct{})

	go func() {
		for i := 0; i < 100; i++ {
			r.UpdateFromEvent(InternalEvent{AgentID: "agent-a", EventType: "test"})
		}
		close(done)
	}()

	go func() {
		for i := 0; i < 100; i++ {
			r.UpdateFromEvent(InternalEvent{AgentID: "agent-b", EventType: "test"})
		}
	}()

	<-done
	time.Sleep(10 * time.Millisecond)

	if r.Count() != 2 {
		t.Errorf("expected count 2, got %d", r.Count())
	}
	stateA := r.Get("agent-a")
	if stateA == nil || stateA.EventCount != 100 {
		t.Errorf("expected 100 events for agent-a, got %d", stateA.EventCount)
	}
}

func TestSubagentState_LastEventAt(t *testing.T) {
	r := NewSubagentRegistry()
	before := time.Now()
	r.UpdateFromEvent(InternalEvent{AgentID: "agent-1"})
	after := time.Now()

	state := r.Get("agent-1")
	if state.LastEventAt.Before(before) || state.LastEventAt.After(after) {
		t.Errorf("LastEventAt %v not in expected range [%v, %v]", state.LastEventAt, before, after)
	}
}
