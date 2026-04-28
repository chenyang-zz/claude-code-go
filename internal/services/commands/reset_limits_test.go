package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestResetLimitsCommandMetadata verifies /reset-limits is exposed as a hidden command.
func TestResetLimitsCommandMetadata(t *testing.T) {
	meta := ResetLimitsCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "reset-limits",
		Aliases:     []string{"reset-limits-non-interactive"},
		Description: "Reset internal usage limits",
		Usage:       "/reset-limits",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want reset-limits metadata", meta)
	}
}

// TestResetLimitsCommandExecute verifies /reset-limits returns the stable fallback for no-arg execution.
func TestResetLimitsCommandExecute(t *testing.T) {
	result, err := ResetLimitsCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != resetLimitsCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, resetLimitsCommandFallback)
	}
}

// TestResetLimitsCommandExecuteRejectsArgs verifies /reset-limits accepts no arguments.
func TestResetLimitsCommandExecuteRejectsArgs(t *testing.T) {
	_, err := ResetLimitsCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /reset-limits" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
