package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/command"
	platformteam "github.com/sheepzhao/claude-code-go/internal/platform/team"
)

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

// fakeTeamStatusProvider returns a stable team summary for tests.
type fakeTeamStatusProvider struct {
	status *platformteam.Status
}

// CurrentTeamStatus returns the injected status summary.
func (f fakeTeamStatusProvider) CurrentTeamStatus(context.Context) (*platformteam.Status, error) {
	return f.status, nil
}
