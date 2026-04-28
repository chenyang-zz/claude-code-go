package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestInitVerifiersCommandMetadata verifies /init-verifiers is exposed as a hidden command.
func TestInitVerifiersCommandMetadata(t *testing.T) {
	meta := InitVerifiersCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "init-verifiers",
		Description: "Create verifier skill(s) for automated verification of code changes",
		Usage:       "/init-verifiers",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want init-verifiers metadata", meta)
	}
}

// TestInitVerifiersCommandExecute verifies /init-verifiers returns the stable fallback for no-arg execution.
func TestInitVerifiersCommandExecute(t *testing.T) {
	result, err := InitVerifiersCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != initVerifiersCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, initVerifiersCommandFallback)
	}
}

// TestInitVerifiersCommandExecuteRejectsArgs verifies /init-verifiers accepts no arguments.
func TestInitVerifiersCommandExecuteRejectsArgs(t *testing.T) {
	_, err := InitVerifiersCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /init-verifiers" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
