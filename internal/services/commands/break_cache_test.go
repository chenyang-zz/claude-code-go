package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestBreakCacheCommandMetadata verifies /break-cache is exposed as a hidden command.
func TestBreakCacheCommandMetadata(t *testing.T) {
	meta := BreakCacheCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "break-cache",
		Description: "Run internal cache-break workflow",
		Usage:       "/break-cache",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want break-cache metadata", meta)
	}
}

// TestBreakCacheCommandExecute verifies /break-cache returns the stable fallback for no-arg execution.
func TestBreakCacheCommandExecute(t *testing.T) {
	result, err := BreakCacheCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != breakCacheCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, breakCacheCommandFallback)
	}
}

// TestBreakCacheCommandExecuteRejectsArgs verifies /break-cache accepts no arguments.
func TestBreakCacheCommandExecuteRejectsArgs(t *testing.T) {
	_, err := BreakCacheCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /break-cache" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
