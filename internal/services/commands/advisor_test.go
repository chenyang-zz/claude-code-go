package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestAdvisorCommandMetadata verifies /advisor is exposed with the expected canonical descriptor.
func TestAdvisorCommandMetadata(t *testing.T) {
	meta := AdvisorCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "advisor",
		Description: "Configure the advisor model",
		Usage:       "/advisor [<model>|off]",
	}) {
		t.Fatalf("Metadata() = %#v, want advisor metadata", meta)
	}
}

// TestAdvisorCommandExecute verifies /advisor returns the stable fallback for no-arg execution.
func TestAdvisorCommandExecute(t *testing.T) {
	result, err := AdvisorCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != advisorCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, advisorCommandFallback)
	}
}

// TestAdvisorCommandExecuteAcceptsArgs verifies /advisor accepts optional argument text.
func TestAdvisorCommandExecuteAcceptsArgs(t *testing.T) {
	result, err := AdvisorCommand{}.Execute(context.Background(), command.Args{RawLine: "opus"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != advisorCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, advisorCommandFallback)
	}
}
