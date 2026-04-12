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
		Name:        "compact",
		Description: "Clear conversation history but keep a summary in context",
		Usage:       "/compact [instructions]",
	}}); err != nil {
		t.Fatalf("Register(compact) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "memory",
		Description: "Edit Claude memory files",
		Usage:       "/memory",
	}}); err != nil {
		t.Fatalf("Register(memory) error = %v", err)
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
		Name:        "fast",
		Description: "Toggle fast mode (Opus 4.6 only)",
		Usage:       "/fast [on|off]",
	}}); err != nil {
		t.Fatalf("Register(fast) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "effort",
		Description: "Set effort level for model usage",
		Usage:       "/effort [low|medium|high|max|auto]",
	}}); err != nil {
		t.Fatalf("Register(effort) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "output-style",
		Description: "Deprecated: use /config to change output style",
		Usage:       "/output-style",
	}}); err != nil {
		t.Fatalf("Register(output-style) error = %v", err)
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
		Name:        "files",
		Description: "List all files currently in context",
		Usage:       "/files",
	}}); err != nil {
		t.Fatalf("Register(files) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "copy",
		Description: "Copy Claude's last response to clipboard (or /copy N for the Nth-latest)",
		Usage:       "/copy [N]",
	}}); err != nil {
		t.Fatalf("Register(copy) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "export",
		Description: "Export the current conversation to a file or clipboard",
		Usage:       "/export [filename]",
	}}); err != nil {
		t.Fatalf("Register(export) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "version",
		Description: "Print the version this session is running (not what autoupdate downloaded)",
		Usage:       "/version",
	}}); err != nil {
		t.Fatalf("Register(version) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "release-notes",
		Description: "View release notes",
		Usage:       "/release-notes",
	}}); err != nil {
		t.Fatalf("Register(release-notes) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "upgrade",
		Description: "Upgrade to Max for higher rate limits and more Opus",
		Usage:       "/upgrade",
	}}); err != nil {
		t.Fatalf("Register(upgrade) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "usage",
		Description: "Show plan usage limits",
		Usage:       "/usage",
	}}); err != nil {
		t.Fatalf("Register(usage) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "stats",
		Description: "Show your Claude Code usage statistics and activity",
		Usage:       "/stats",
	}}); err != nil {
		t.Fatalf("Register(stats) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "extra-usage",
		Description: "Configure extra usage to keep working when limits are hit",
		Usage:       "/extra-usage",
	}}); err != nil {
		t.Fatalf("Register(extra-usage) error = %v", err)
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

	want := "Available commands:\n/help - Show help and available commands\n/clear - Clear conversation history and start a new session\n/compact - Clear conversation history but keep a summary in context\n  Usage: /compact [instructions]\n/memory - Edit Claude memory files\n/resume - Resume a saved session by search or continue it with a new prompt\n  Aliases: /continue\n  Usage: /resume <search-term> | /resume <session-id> <prompt>\n/config - Show the current runtime configuration\n  Aliases: /settings\n/model - Change the model\n  Usage: /model [model]\n/fast - Toggle fast mode (Opus 4.6 only)\n  Usage: /fast [on|off]\n/effort - Set effort level for model usage\n  Usage: /effort [low|medium|high|max|auto]\n/output-style - Deprecated: use /config to change output style\n/rename - Rename the current conversation for easier resume discovery\n  Usage: /rename <title>\n/doctor - Diagnose the current Claude Code Go host setup\n/permissions - Manage allow & deny tool permission rules\n  Aliases: /allowed-tools\n/add-dir - Add a new working directory\n  Usage: /add-dir <path>\n/login - Sign in with your Anthropic account\n/logout - Sign out from your Anthropic account\n/cost - Show the total cost and duration of the current session\n/status - Show Claude Code status including version, model, account, API connectivity, and tool statuses\n/mcp - Manage MCP servers\n  Usage: /mcp [enable|disable <server-name>]\n/session - Show remote session URL and QR code\n/files - List all files currently in context\n/copy - Copy Claude's last response to clipboard (or /copy N for the Nth-latest)\n  Usage: /copy [N]\n/export - Export the current conversation to a file or clipboard\n  Usage: /export [filename]\n/version - Print the version this session is running (not what autoupdate downloaded)\n/release-notes - View release notes\n/upgrade - Upgrade to Max for higher rate limits and more Opus\n/usage - Show plan usage limits\n/stats - Show your Claude Code usage statistics and activity\n/extra-usage - Configure extra usage to keep working when limits are hit\n/theme - Change the theme\n  Usage: /theme <auto|dark|light|light-daltonized|dark-daltonized|light-ansi|dark-ansi>\n/vim - Toggle between Vim and Normal editing modes\n/seed-sessions - Insert demo persisted sessions for /resume testing\nSend plain text without a leading slash to start a normal prompt."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
