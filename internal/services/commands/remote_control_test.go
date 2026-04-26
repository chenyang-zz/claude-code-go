package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestRemoteControlCommandMetadata verifies /remote-control is exposed with the expected canonical descriptor.
func TestRemoteControlCommandMetadata(t *testing.T) {
	meta := RemoteControlCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "remote-control",
		Aliases:     []string{"rc"},
		Description: "Connect this terminal for remote-control sessions",
		Usage:       "/remote-control [name]",
	}) {
		t.Fatalf("Metadata() = %#v, want remote-control metadata", meta)
	}
}

// TestRemoteControlCommandExecute verifies /remote-control returns the stable fallback for no-arg execution.
func TestRemoteControlCommandExecute(t *testing.T) {
	result, err := RemoteControlCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != remoteControlCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, remoteControlCommandFallback)
	}
}

// TestRemoteControlCommandExecuteAcceptsArgs verifies /remote-control accepts optional name.
func TestRemoteControlCommandExecuteAcceptsArgs(t *testing.T) {
	result, err := RemoteControlCommand{}.Execute(context.Background(), command.Args{RawLine: "my-terminal"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != remoteControlCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, remoteControlCommandFallback)
	}
}
