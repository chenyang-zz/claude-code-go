package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestMockLimitsCommandMetadata verifies /mock-limits is exposed as a hidden command.
func TestMockLimitsCommandMetadata(t *testing.T) {
	meta := MockLimitsCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "mock-limits",
		Description: "Mock internal usage limits",
		Usage:       "/mock-limits",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want mock-limits metadata", meta)
	}
}

// TestMockLimitsCommandExecute verifies /mock-limits returns the stable fallback for no-arg execution.
func TestMockLimitsCommandExecute(t *testing.T) {
	result, err := MockLimitsCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != mockLimitsCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, mockLimitsCommandFallback)
	}
}

// TestMockLimitsCommandExecuteRejectsArgs verifies /mock-limits accepts no arguments.
func TestMockLimitsCommandExecuteRejectsArgs(t *testing.T) {
	_, err := MockLimitsCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /mock-limits" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
