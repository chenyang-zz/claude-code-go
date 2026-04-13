package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
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
