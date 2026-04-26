package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestBridgeKickCommandMetadata verifies /bridge-kick is exposed as a hidden command.
func TestBridgeKickCommandMetadata(t *testing.T) {
	meta := BridgeKickCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "bridge-kick",
		Description: "Inject bridge failure states for manual recovery testing",
		Usage:       "/bridge-kick <subcommand>",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want bridge-kick metadata", meta)
	}
}

// TestBridgeKickCommandExecute verifies /bridge-kick returns the stable fallback when a subcommand is provided.
func TestBridgeKickCommandExecute(t *testing.T) {
	result, err := BridgeKickCommand{}.Execute(context.Background(), command.Args{RawLine: "status"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != bridgeKickCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, bridgeKickCommandFallback)
	}
}

// TestBridgeKickCommandExecuteRejectsEmptyArgs verifies /bridge-kick requires a subcommand argument.
func TestBridgeKickCommandExecuteRejectsEmptyArgs(t *testing.T) {
	_, err := BridgeKickCommand{}.Execute(context.Background(), command.Args{})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /bridge-kick <subcommand>" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
