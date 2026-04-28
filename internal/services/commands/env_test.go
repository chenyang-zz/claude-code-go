package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestEnvCommandMetadata verifies /env is exposed as a hidden command.
func TestEnvCommandMetadata(t *testing.T) {
	meta := EnvCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "env",
		Description: "Inspect internal environment state",
		Usage:       "/env",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want env metadata", meta)
	}
}

// TestEnvCommandExecute verifies /env returns the stable fallback for no-arg execution.
func TestEnvCommandExecute(t *testing.T) {
	result, err := EnvCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != envCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, envCommandFallback)
	}
}

// TestEnvCommandExecuteRejectsArgs verifies /env accepts no arguments.
func TestEnvCommandExecuteRejectsArgs(t *testing.T) {
	_, err := EnvCommand{}.Execute(context.Background(), command.Args{RawLine: "print"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /env" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
