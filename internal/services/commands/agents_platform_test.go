package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestAgentsPlatformCommandMetadata verifies /agents-platform is exposed as a hidden command.
func TestAgentsPlatformCommandMetadata(t *testing.T) {
	meta := AgentsPlatformCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "agents-platform",
		Description: "Run internal agents-platform workflow",
		Usage:       "/agents-platform",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want agents-platform metadata", meta)
	}
}

// TestAgentsPlatformCommandExecute verifies /agents-platform returns the stable fallback for no-arg execution.
func TestAgentsPlatformCommandExecute(t *testing.T) {
	result, err := AgentsPlatformCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != agentsPlatformCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, agentsPlatformCommandFallback)
	}
}

// TestAgentsPlatformCommandExecuteRejectsArgs verifies /agents-platform accepts no arguments.
func TestAgentsPlatformCommandExecuteRejectsArgs(t *testing.T) {
	_, err := AgentsPlatformCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /agents-platform" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
