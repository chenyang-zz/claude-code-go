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
		Description: "Resume a saved session by search or continue it with a new prompt",
		Usage:       "/resume <search-term> | /resume <session-id> <prompt>",
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
		Name:        "model",
		Description: "Change the model",
		Usage:       "/model [model]",
	}}); err != nil {
		t.Fatalf("Register(model) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "rename",
		Description: "Rename the current conversation for easier resume discovery",
		Usage:       "/rename <title>",
	}}); err != nil {
		t.Fatalf("Register(rename) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "doctor",
		Description: "Diagnose the current Claude Code Go host setup",
		Usage:       "/doctor",
	}}); err != nil {
		t.Fatalf("Register(doctor) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "permissions",
		Aliases:     []string{"allowed-tools"},
		Description: "Manage allow & deny tool permission rules",
		Usage:       "/permissions",
	}}); err != nil {
		t.Fatalf("Register(permissions) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "add-dir",
		Description: "Add a new working directory",
		Usage:       "/add-dir <path>",
	}}); err != nil {
		t.Fatalf("Register(add-dir) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "login",
		Description: "Sign in with your Anthropic account",
		Usage:       "/login",
	}}); err != nil {
		t.Fatalf("Register(login) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "logout",
		Description: "Sign out from your Anthropic account",
		Usage:       "/logout",
	}}); err != nil {
		t.Fatalf("Register(logout) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "cost",
		Description: "Show the total cost and duration of the current session",
		Usage:       "/cost",
	}}); err != nil {
		t.Fatalf("Register(cost) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "status",
		Description: "Show Claude Code status including version, model, account, API connectivity, and tool statuses",
		Usage:       "/status",
	}}); err != nil {
		t.Fatalf("Register(status) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "mcp",
		Description: "Manage MCP servers",
		Usage:       "/mcp [enable|disable <server-name>]",
	}}); err != nil {
		t.Fatalf("Register(mcp) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "session",
		Description: "Show remote session URL and QR code",
		Usage:       "/session",
	}}); err != nil {
		t.Fatalf("Register(session) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "theme",
		Description: "Change the theme",
		Usage:       "/theme <auto|dark|light|light-daltonized|dark-daltonized|light-ansi|dark-ansi>",
	}}); err != nil {
		t.Fatalf("Register(theme) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "vim",
		Description: "Toggle between Vim and Normal editing modes",
		Usage:       "/vim",
	}}); err != nil {
		t.Fatalf("Register(vim) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "seed-sessions",
		Description: "Insert demo persisted sessions for /resume testing",
		Usage:       "/seed-sessions",
	}}); err != nil {
		t.Fatalf("Register(seed-sessions) error = %v", err)
	}

	result, err := HelpCommand{Registry: registry}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Available commands:\n/help - Show help and available commands\n/clear - Clear conversation history and start a new session\n/resume - Resume a saved session by search or continue it with a new prompt\n  Aliases: /continue\n  Usage: /resume <search-term> | /resume <session-id> <prompt>\n/config - Show the current runtime configuration\n  Aliases: /settings\n/model - Change the model\n  Usage: /model [model]\n/rename - Rename the current conversation for easier resume discovery\n  Usage: /rename <title>\n/doctor - Diagnose the current Claude Code Go host setup\n/permissions - Manage allow & deny tool permission rules\n  Aliases: /allowed-tools\n/add-dir - Add a new working directory\n  Usage: /add-dir <path>\n/login - Sign in with your Anthropic account\n/logout - Sign out from your Anthropic account\n/cost - Show the total cost and duration of the current session\n/status - Show Claude Code status including version, model, account, API connectivity, and tool statuses\n/mcp - Manage MCP servers\n  Usage: /mcp [enable|disable <server-name>]\n/session - Show remote session URL and QR code\n/theme - Change the theme\n  Usage: /theme <auto|dark|light|light-daltonized|dark-daltonized|light-ansi|dark-ansi>\n/vim - Toggle between Vim and Normal editing modes\n/seed-sessions - Insert demo persisted sessions for /resume testing\nSend plain text without a leading slash to start a normal prompt."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
