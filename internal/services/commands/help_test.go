package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

type stubCommand struct {
	meta command.Metadata
}

func (c stubCommand) Metadata() command.Metadata {
	return c.meta
}

func (c stubCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args
	return command.Result{}, nil
}

// TestHelpCommandExecuteRendersRegisteredCommands verifies /help reflects the currently registered minimum command catalog.
func TestHelpCommandExecuteRendersRegisteredCommands(t *testing.T) {
	registry := command.NewInMemoryRegistry()
	if err := registry.Register(HelpCommand{Registry: registry}); err != nil {
		t.Fatalf("Register(help) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "clear",
		Description: "Clear conversation history and start a new session",
		Usage:       "/clear",
	}}); err != nil {
		t.Fatalf("Register(clear) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "resume",
		Aliases:     []string{"continue"},
		Description: "Resume a saved session and continue it with a new prompt",
		Usage:       "/resume <session-id> <prompt>",
	}}); err != nil {
		t.Fatalf("Register(resume) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "config",
		Aliases:     []string{"settings"},
		Description: "Show the current runtime configuration",
		Usage:       "/config",
	}}); err != nil {
		t.Fatalf("Register(config) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "doctor",
		Description: "Diagnose the current Claude Code Go host setup",
		Usage:       "/doctor",
	}}); err != nil {
		t.Fatalf("Register(doctor) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "session",
		Description: "Show remote session URL and QR code",
		Usage:       "/session",
	}}); err != nil {
		t.Fatalf("Register(session) error = %v", err)
	}

	result, err := HelpCommand{Registry: registry}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Available commands:\n/help - Show help and available commands\n/clear - Clear conversation history and start a new session\n/resume - Resume a saved session and continue it with a new prompt\n  Aliases: /continue\n  Usage: /resume <session-id> <prompt>\n/config - Show the current runtime configuration\n  Aliases: /settings\n/doctor - Diagnose the current Claude Code Go host setup\n/session - Show remote session URL and QR code\nSend plain text without a leading slash to start a normal prompt."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
