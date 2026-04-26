package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestSandboxCommandMetadata verifies /sandbox is exposed with the expected canonical descriptor.
func TestSandboxCommandMetadata(t *testing.T) {
	meta := SandboxCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "sandbox",
		Description: "Configure sandbox settings",
		Usage:       "/sandbox [exclude <command-pattern>]",
	}) {
		t.Fatalf("Metadata() = %#v, want sandbox metadata", meta)
	}
}

// TestSandboxCommandExecute verifies /sandbox returns the stable fallback for no-arg execution.
func TestSandboxCommandExecute(t *testing.T) {
	result, err := SandboxCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != sandboxCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, sandboxCommandFallback)
	}
}

// TestSandboxCommandExecuteWithExclude verifies /sandbox accepts the exclude subcommand.
func TestSandboxCommandExecuteWithExclude(t *testing.T) {
	result, err := SandboxCommand{}.Execute(context.Background(), command.Args{RawLine: `exclude "npm run test:*"`})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != sandboxCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, sandboxCommandFallback)
	}
}

// TestSandboxCommandExecuteRejectsUnknownSubcommand verifies /sandbox validates subcommands.
func TestSandboxCommandExecuteRejectsUnknownSubcommand(t *testing.T) {
	_, err := SandboxCommand{}.Execute(context.Background(), command.Args{RawLine: "enable"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /sandbox [exclude <command-pattern>]" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}

// TestSandboxCommandExecuteRejectsEmptyExclude verifies /sandbox exclude requires one pattern argument.
func TestSandboxCommandExecuteRejectsEmptyExclude(t *testing.T) {
	_, err := SandboxCommand{}.Execute(context.Background(), command.Args{RawLine: "exclude"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /sandbox [exclude <command-pattern>]" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
