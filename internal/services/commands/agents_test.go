package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/command"
	platformteam "github.com/sheepzhao/claude-code-go/internal/platform/team"
)

// fakeAgentRegistry is a test double for agent.Registry.
type fakeAgentRegistry struct {
	defs []agent.Definition
}

func (f fakeAgentRegistry) Register(def agent.Definition) error { return nil }
func (f fakeAgentRegistry) Get(agentType string) (agent.Definition, bool) {
	for _, d := range f.defs {
		if d.AgentType == agentType {
			return d, true
		}
	}
	return agent.Definition{}, false
}
func (f fakeAgentRegistry) List() []agent.Definition { return f.defs }
func (f fakeAgentRegistry) Remove(agentType string) bool { return false }

// TestAgentsCommandMetadata verifies /agents is exposed with the expected canonical descriptor.
func TestAgentsCommandMetadata(t *testing.T) {
	meta := AgentsCommand{}.Metadata()

	if meta.Name != "agents" {
		t.Fatalf("Metadata().Name = %q, want agents", meta.Name)
	}
	if meta.Description != "Manage agent configurations" {
		t.Fatalf("Metadata().Description = %q, want agents description", meta.Description)
	}
	if meta.Usage != "/agents" {
		t.Fatalf("Metadata().Usage = %q, want /agents", meta.Usage)
	}
}

// TestAgentsCommandExecute verifies /agents returns the stable settings fallback.
func TestAgentsCommandExecute(t *testing.T) {
	result, err := AgentsCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != agentsCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, agentsCommandFallback)
	}
}

// TestAgentsCommandExecuteWithProvider verifies /agents renders a live team summary when available.
func TestAgentsCommandExecuteWithProvider(t *testing.T) {
	cmd := AgentsCommand{
		StatusProvider: fakeTeamStatusProvider{
			status: &platformteam.Status{
				TeamName:    "alpha-team",
				LeadAgentID: "lead-1",
				Members: []agent.Status{
					{
						AgentID:      "lead-1",
						Name:         "team-lead",
						AgentType:    "lead",
						Status:       agent.StatusIdle,
						CurrentTasks: nil,
					},
					{
						AgentID:      "agent-2",
						Name:         "researcher",
						AgentType:    "research",
						Status:       agent.StatusBusy,
						CurrentTasks: []string{"1", "2"},
					},
				},
			},
		},
	}

	result, err := cmd.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output == agentsCommandFallback {
		t.Fatal("Execute() returned fallback output, want live summary")
	}
	for _, want := range []string{"Agent status summary:", "Team: alpha-team", "Lead agent ID: lead-1", "researcher (research): busy [1, 2]"} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("Execute() output missing %q:\n%s", want, result.Output)
		}
	}
}

// TestAgentsCommandExecuteWithRegistry verifies /agents renders registered agents with colors.
func TestAgentsCommandExecuteWithRegistry(t *testing.T) {
	cmd := AgentsCommand{
		Registry: fakeAgentRegistry{
			defs: []agent.Definition{
				{AgentType: "explore", WhenToUse: "Explore codebase", Color: "blue"},
				{AgentType: "plan", WhenToUse: "Plan implementation", Color: "purple"},
				{AgentType: "review", WhenToUse: "Review code", Color: ""},
			},
		},
	}

	result, err := cmd.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, want := range []string{
		"Registered agents:",
		"explore: Explore codebase [color: blue]",
		"plan: Plan implementation [color: purple]",
		"review: Review code",
	} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("Execute() output missing %q:\n%s", want, result.Output)
		}
	}
}

// TestAgentsCommandExecuteWithProviderAndRegistry verifies /agents renders both team status and agent definitions.
func TestAgentsCommandExecuteWithProviderAndRegistry(t *testing.T) {
	cmd := AgentsCommand{
		StatusProvider: fakeTeamStatusProvider{
			status: &platformteam.Status{
				TeamName: "alpha-team",
				Members: []agent.Status{
					{AgentID: "agent-1", Name: "researcher", AgentType: "research", Status: agent.StatusIdle},
				},
			},
		},
		Registry: fakeAgentRegistry{
			defs: []agent.Definition{
				{AgentType: "research", WhenToUse: "Research tasks", Color: "green"},
			},
		},
	}

	result, err := cmd.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, want := range []string{
		"Agent status summary:",
		"Team: alpha-team",
		"researcher (research): idle",
		"Registered agents:",
		"research: Research tasks [color: green]",
	} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("Execute() output missing %q:\n%s", want, result.Output)
		}
	}
}

// TestRenderAgentDefinitions verifies the agent definition rendering helper.
func TestRenderAgentDefinitions(t *testing.T) {
	defs := []agent.Definition{
		{AgentType: "explore", WhenToUse: "Explore codebase", Color: "blue"},
		{AgentType: "review", WhenToUse: "Review code", Color: ""},
	}

	output := renderAgentDefinitions(defs)
	for _, want := range []string{
		"Registered agents:",
		"explore: Explore codebase [color: blue]",
		"review: Review code",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("renderAgentDefinitions() missing %q:\n%s", want, output)
		}
	}
}

// TestRenderAgentDefinitionsEmpty verifies empty registry produces no output.
func TestRenderAgentDefinitionsEmpty(t *testing.T) {
	output := renderAgentDefinitions(nil)
	if output != "" {
		t.Fatalf("renderAgentDefinitions(nil) = %q, want empty string", output)
	}
	output = renderAgentDefinitions([]agent.Definition{})
	if output != "" {
		t.Fatalf("renderAgentDefinitions([]) = %q, want empty string", output)
	}
}

// fakeTeamStatusProvider returns a stable team summary for tests.
type fakeTeamStatusProvider struct {
	status *platformteam.Status
}

// CurrentTeamStatus returns the injected status summary.
func (f fakeTeamStatusProvider) CurrentTeamStatus(context.Context) (*platformteam.Status, error) {
	return f.status, nil
}
