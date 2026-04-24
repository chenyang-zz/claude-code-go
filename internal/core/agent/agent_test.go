package agent

import (
	"testing"
	"time"
)

func TestStatusTypeConstants(t *testing.T) {
	if StatusIdle != "idle" {
		t.Errorf("StatusIdle = %q, want %q", StatusIdle, "idle")
	}
	if StatusBusy != "busy" {
		t.Errorf("StatusBusy = %q, want %q", StatusBusy, "busy")
	}
}

func TestAgentStruct(t *testing.T) {
	now := time.Now()
	a := Agent{
		ID:        "agent-1",
		Name:      "Explore Agent",
		Type:      "explore",
		Status:    StatusIdle,
		CreatedAt: now,
	}
	if a.ID != "agent-1" {
		t.Errorf("ID = %q, want %q", a.ID, "agent-1")
	}
	if a.Status != StatusIdle {
		t.Errorf("Status = %q, want %q", a.Status, StatusIdle)
	}
}

func TestDefinitionTypeGuards(t *testing.T) {
	builtIn := Definition{AgentType: "explore", Source: "built-in"}
	custom := Definition{AgentType: "my-agent", Source: "userSettings"}
	plugin := Definition{AgentType: "plugin-agent", Source: "plugin", Plugin: "my-plugin"}

	if !builtIn.IsBuiltIn() {
		t.Error("expected built-in agent")
	}
	if builtIn.IsCustom() {
		t.Error("built-in should not be custom")
	}
	if builtIn.IsPlugin() {
		t.Error("built-in should not be plugin")
	}

	if !custom.IsCustom() {
		t.Error("expected custom agent")
	}
	if custom.IsBuiltIn() {
		t.Error("custom should not be built-in")
	}

	if !plugin.IsPlugin() {
		t.Error("expected plugin agent")
	}
	if plugin.IsCustom() {
		t.Error("plugin should not be custom")
	}
}

func TestStatusStruct(t *testing.T) {
	s := Status{
		AgentID:      "a1",
		Name:         "Agent One",
		AgentType:    "verify",
		Status:       StatusBusy,
		CurrentTasks: []string{"t1", "t2"},
	}
	if len(s.CurrentTasks) != 2 {
		t.Errorf("len(CurrentTasks) = %d, want 2", len(s.CurrentTasks))
	}
}

func TestTeamStruct(t *testing.T) {
	team := Team{
		ID:          "team-1",
		Name:        "migration-team",
		Members:     []Agent{{ID: "a1", Name: "Lead"}},
		LeadAgentID: "a1",
		TaskListID:  "migration-team",
	}
	if len(team.Members) != 1 {
		t.Errorf("len(Members) = %d, want 1", len(team.Members))
	}
	if team.LeadAgentID != "a1" {
		t.Errorf("LeadAgentID = %q, want %q", team.LeadAgentID, "a1")
	}
}

func TestMessageStruct(t *testing.T) {
	m := Message{
		From:      "a1",
		To:        "a2",
		Content:   "hello",
		Timestamp: time.Now(),
	}
	if m.From != "a1" {
		t.Errorf("From = %q, want %q", m.From, "a1")
	}
}

func TestDefinitionFields(t *testing.T) {
	d := Definition{
		AgentType:       "plan",
		WhenToUse:       "when planning is needed",
		Source:          "projectSettings",
		Tools:           []string{"BashTool", "FileReadTool"},
		DisallowedTools: []string{"FileWriteTool"},
		Skills:          []string{"git"},
		Model:           "sonnet",
		Effort:          "high",
		PermissionMode:  "plan",
		MaxTurns:        10,
		Background:      true,
		Memory:          "project",
		Isolation:       "worktree",
		OmitClaudeMd:    true,
		Plugin:          "",
		Filename:        "plan-agent",
		BaseDir:         "/agents",
		InitialPrompt:   "Start planning",
		CriticalSystemReminder: "Be careful",
	}

	if d.AgentType != "plan" {
		t.Errorf("AgentType = %q, want %q", d.AgentType, "plan")
	}
	if len(d.Tools) != 2 {
		t.Errorf("len(Tools) = %d, want 2", len(d.Tools))
	}
	if !d.Background {
		t.Error("expected Background to be true")
	}
}
