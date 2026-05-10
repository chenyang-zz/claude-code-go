package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestSandboxCommandMetadata verifies /sandbox is exposed with the expected canonical descriptor.
func TestSandboxCommandMetadata(t *testing.T) {
	cmd := NewSandboxCommand(nil)
	meta := cmd.Metadata()

	if meta.Name != "sandbox" {
		t.Fatalf("Metadata().Name = %q, want %q", meta.Name, "sandbox")
	}
	if meta.Description == "" {
		t.Fatal("Metadata().Description = empty, want non-empty")
	}
}

// TestSandboxCommandExecuteNilManager verifies /sandbox handles a nil manager gracefully.
func TestSandboxCommandExecuteNilManager(t *testing.T) {
	cmd := NewSandboxCommand(nil)
	result, err := cmd.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output == "" {
		t.Fatal("Execute() output = empty")
	}
}

// TestSandboxCommandExclude verifies /sandbox exclude subcommand.
func TestSandboxCommandExclude(t *testing.T) {
	cmd := NewSandboxCommand(nil)
	result, err := cmd.Execute(context.Background(), command.Args{RawLine: `exclude "npm run test:*"`})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output == "" {
		t.Fatal("Execute() output = empty, want pattern acknowledgment")
	}
}

// TestSandboxCommandRejectsUnknownSubcommand verifies /sandbox validates subcommands.
func TestSandboxCommandRejectsUnknownSubcommand(t *testing.T) {
	cmd := NewSandboxCommand(nil)
	_, err := cmd.Execute(context.Background(), command.Args{RawLine: "enable"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
}

// TestSandboxCommandRejectsEmptyExclude verifies /sandbox exclude requires one pattern argument.
func TestSandboxCommandRejectsEmptyExclude(t *testing.T) {
	cmd := NewSandboxCommand(nil)
	_, err := cmd.Execute(context.Background(), command.Args{RawLine: "exclude"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
}
