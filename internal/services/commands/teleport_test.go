package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestTeleportCommandMetadata verifies /teleport is exposed as a hidden command.
func TestTeleportCommandMetadata(t *testing.T) {
	meta := TeleportCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "teleport",
		Description: "Teleport the current session to remote runtime",
		Usage:       "/teleport",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want teleport metadata", meta)
	}
}

// TestTeleportCommandExecute verifies /teleport returns the stable fallback for no-arg execution.
func TestTeleportCommandExecute(t *testing.T) {
	result, err := TeleportCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != teleportCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, teleportCommandFallback)
	}
}

// TestTeleportCommandExecuteRejectsArgs verifies /teleport accepts no arguments.
func TestTeleportCommandExecuteRejectsArgs(t *testing.T) {
	_, err := TeleportCommand{}.Execute(context.Background(), command.Args{RawLine: "session-1"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /teleport" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
