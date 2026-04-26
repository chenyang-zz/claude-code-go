package commands

import (
	"context"
	"strings"
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
		Name:        "branch",
		Aliases:     []string{"fork"},
		Description: "Create a branch of the current conversation at this point",
		Usage:       "/branch [name]",
	}}); err != nil {
		t.Fatalf("Register(branch) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "voice",
		Description: "Toggle voice mode",
		Usage:       "/voice",
	}}); err != nil {
		t.Fatalf("Register(voice) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "ide",
		Description: "Manage IDE integrations and show status",
		Usage:       "/ide [open]",
	}}); err != nil {
		t.Fatalf("Register(ide) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "init",
		Description: "Initialize a new CLAUDE.md file with codebase documentation",
		Usage:       "/init",
	}}); err != nil {
		t.Fatalf("Register(init) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "install-github-app",
		Description: "Set up Claude GitHub Actions for a repository",
		Usage:       "/install-github-app",
	}}); err != nil {
		t.Fatalf("Register(install-github-app) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "install-slack-app",
		Description: "Install the Claude Slack app",
		Usage:       "/install-slack-app",
	}}); err != nil {
		t.Fatalf("Register(install-slack-app) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "remote-env",
		Description: "Configure the default remote environment for teleport sessions",
		Usage:       "/remote-env",
	}}); err != nil {
		t.Fatalf("Register(remote-env) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "desktop",
		Aliases:     []string{"app"},
		Description: "Continue the current session in Claude Desktop",
		Usage:       "/desktop",
	}}); err != nil {
		t.Fatalf("Register(desktop) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "mobile",
		Aliases:     []string{"ios", "android"},
		Description: "Show QR code to download the Claude mobile app",
		Usage:       "/mobile",
	}}); err != nil {
		t.Fatalf("Register(mobile) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "feedback",
		Aliases:     []string{"bug"},
		Description: "Submit feedback about Claude Code",
		Usage:       "/feedback [report]",
	}}); err != nil {
		t.Fatalf("Register(feedback) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "exit",
		Aliases:     []string{"quit"},
		Description: "Exit the REPL",
		Usage:       "/exit",
	}}); err != nil {
		t.Fatalf("Register(exit) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "install",
		Description: "Install Claude Code native build",
		Usage:       "/install [options]",
	}}); err != nil {
		t.Fatalf("Register(install) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "context",
		Description: "Show current context usage",
		Usage:       "/context",
	}}); err != nil {
		t.Fatalf("Register(context) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "review",
		Description: "Review a pull request",
		Usage:       "/review [pr-number]",
	}}); err != nil {
		t.Fatalf("Register(review) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "rewind",
		Aliases:     []string{"checkpoint"},
		Description: "Restore the code and/or conversation to a previous point",
		Usage:       "/rewind",
	}}); err != nil {
		t.Fatalf("Register(rewind) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "skills",
		Description: "List available skills",
		Usage:       "/skills",
	}}); err != nil {
		t.Fatalf("Register(skills) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "tag",
		Description: "Toggle a searchable tag on the current session",
		Usage:       "/tag <tag-name>",
	}}); err != nil {
		t.Fatalf("Register(tag) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "color",
		Description: "Set the prompt bar color for this session",
		Usage:       "/color <color|default>",
	}}); err != nil {
		t.Fatalf("Register(color) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "passes",
		Description: "Share a free week of Claude Code with friends",
		Usage:       "/passes",
	}}); err != nil {
		t.Fatalf("Register(passes) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "rate-limit-options",
		Description: "Show options when rate limit is reached",
		Usage:       "/rate-limit-options",
		Hidden:      true,
	}}); err != nil {
		t.Fatalf("Register(rate-limit-options) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "sandbox",
		Description: "Configure sandbox settings",
		Usage:       "/sandbox [exclude <command-pattern>]",
	}}); err != nil {
		t.Fatalf("Register(sandbox) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "stickers",
		Description: "Order Claude Code stickers",
		Usage:       "/stickers",
	}}); err != nil {
		t.Fatalf("Register(stickers) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "privacy-settings",
		Description: "View and update your privacy settings",
		Usage:       "/privacy-settings",
	}}); err != nil {
		t.Fatalf("Register(privacy-settings) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "plan",
		Description: "Enable plan mode or view the current session plan",
		Usage:       "/plan [open|<description>]",
	}}); err != nil {
		t.Fatalf("Register(plan) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "tasks",
		Aliases:     []string{"bashes"},
		Description: "List and manage background tasks",
		Usage:       "/tasks",
	}}); err != nil {
		t.Fatalf("Register(tasks) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "diff",
		Description: "View uncommitted changes and per-turn diffs",
		Usage:       "/diff",
	}}); err != nil {
		t.Fatalf("Register(diff) error = %v", err)
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
		Name:        "terminal-setup",
		Description: "Install Shift+Enter key binding for newlines",
		Usage:       "/terminal-setup",
	}}); err != nil {
		t.Fatalf("Register(terminal-setup) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "keybindings",
		Description: "Open or create your keybindings configuration file",
		Usage:       "/keybindings",
	}}); err != nil {
		t.Fatalf("Register(keybindings) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "btw",
		Description: "Ask a quick side question without interrupting the main conversation",
		Usage:       "/btw <question>",
	}}); err != nil {
		t.Fatalf("Register(btw) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "chrome",
		Description: "Claude in Chrome (Beta) settings",
		Usage:       "/chrome",
	}}); err != nil {
		t.Fatalf("Register(chrome) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "think-back",
		Description: "Your 2025 Claude Code Year in Review",
		Usage:       "/think-back",
	}}); err != nil {
		t.Fatalf("Register(think-back) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "thinkback-play",
		Description: "Play the thinkback animation",
		Usage:       "/thinkback-play",
		Hidden:      true,
	}}); err != nil {
		t.Fatalf("Register(thinkback-play) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "reload-plugins",
		Description: "Activate pending plugin changes in the current session",
		Usage:       "/reload-plugins",
	}}); err != nil {
		t.Fatalf("Register(reload-plugins) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "advisor",
		Description: "Configure the advisor model",
		Usage:       "/advisor [<model>|off]",
	}}); err != nil {
		t.Fatalf("Register(advisor) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "statusline",
		Description: "Set up Claude Code's status line UI",
		Usage:       "/statusline [prompt]",
	}}); err != nil {
		t.Fatalf("Register(statusline) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "ultrareview",
		Description: "Find and verify bugs in your branch using Claude Code on the web",
		Usage:       "/ultrareview [pr-number]",
	}}); err != nil {
		t.Fatalf("Register(ultrareview) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "insights",
		Description: "Generate a report analyzing your Claude Code sessions",
		Usage:       "/insights [--homespaces]",
	}}); err != nil {
		t.Fatalf("Register(insights) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "remote-control",
		Aliases:     []string{"rc"},
		Description: "Connect this terminal for remote-control sessions",
		Usage:       "/remote-control [name]",
	}}); err != nil {
		t.Fatalf("Register(remote-control) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "bridge-kick",
		Description: "Inject bridge failure states for manual recovery testing",
		Usage:       "/bridge-kick <subcommand>",
		Hidden:      true,
	}}); err != nil {
		t.Fatalf("Register(bridge-kick) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "commit",
		Description: "Create a git commit",
		Usage:       "/commit",
	}}); err != nil {
		t.Fatalf("Register(commit) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "commit-push-pr",
		Description: "Commit, push, and open a PR",
		Usage:       "/commit-push-pr [instructions]",
	}}); err != nil {
		t.Fatalf("Register(commit-push-pr) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "heapdump",
		Description: "Dump the JS heap to ~/Desktop",
		Usage:       "/heapdump",
		Hidden:      true,
	}}); err != nil {
		t.Fatalf("Register(heapdump) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "web-setup",
		Description: "Setup Claude Code on the web (requires connecting your GitHub account)",
		Usage:       "/web-setup",
	}}); err != nil {
		t.Fatalf("Register(web-setup) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "seed-sessions",
		Description: "Insert demo persisted sessions for /resume testing",
		Usage:       "/seed-sessions",
	}}); err != nil {
		t.Fatalf("Register(seed-sessions) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "agents",
		Description: "Manage agent configurations",
		Usage:       "/agents",
	}}); err != nil {
		t.Fatalf("Register(agents) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "plugin",
		Aliases:     []string{"plugins", "marketplace"},
		Description: "Manage Claude Code plugins",
		Usage:       "/plugin [subcommand]",
	}}); err != nil {
		t.Fatalf("Register(plugin) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: command.Metadata{
		Name:        "hooks",
		Description: "View hook configurations for tool events",
		Usage:       "/hooks",
	}}); err != nil {
		t.Fatalf("Register(hooks) error = %v", err)
	}

	result, err := HelpCommand{Registry: registry}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Available commands:\n/help - Show help and available commands\n/clear - Clear conversation history and start a new session\n/compact - Clear conversation history but keep a summary in context\n  Usage: /compact [instructions]\n/memory - Edit Claude memory files\n/resume - Resume a saved session by search or continue it with a new prompt\n  Aliases: /continue\n  Usage: /resume <search-term> | /resume <session-id> <prompt>\n/config - Show the current runtime configuration\n  Aliases: /settings\n/model - Change the model\n  Usage: /model [model]\n/fast - Toggle fast mode (Opus 4.6 only)\n  Usage: /fast [on|off]\n/effort - Set effort level for model usage\n  Usage: /effort [low|medium|high|max|auto]\n/output-style - Deprecated: use /config to change output style\n/rename - Rename the current conversation for easier resume discovery\n  Usage: /rename <title>\n/doctor - Diagnose the current Claude Code Go host setup\n/permissions - Manage allow & deny tool permission rules\n  Aliases: /allowed-tools\n/add-dir - Add a new working directory\n  Usage: /add-dir <path>\n/login - Sign in with your Anthropic account\n/logout - Sign out from your Anthropic account\n/cost - Show the total cost and duration of the current session\n/status - Show Claude Code status including version, model, account, API connectivity, and tool statuses\n/mcp - Manage MCP servers\n  Usage: /mcp [enable|disable <server-name>]\n/session - Show remote session URL and QR code\n/branch - Create a branch of the current conversation at this point\n  Aliases: /fork\n  Usage: /branch [name]\n/voice - Toggle voice mode\n/ide - Manage IDE integrations and show status\n  Usage: /ide [open]\n/init - Initialize a new CLAUDE.md file with codebase documentation\n/install-github-app - Set up Claude GitHub Actions for a repository\n/install-slack-app - Install the Claude Slack app\n/remote-env - Configure the default remote environment for teleport sessions\n/desktop - Continue the current session in Claude Desktop\n  Aliases: /app\n/mobile - Show QR code to download the Claude mobile app\n  Aliases: /ios, /android\n/feedback - Submit feedback about Claude Code\n  Aliases: /bug\n  Usage: /feedback [report]\n/exit - Exit the REPL\n  Aliases: /quit\n/install - Install Claude Code native build\n  Usage: /install [options]\n/context - Show current context usage\n/review - Review a pull request\n  Usage: /review [pr-number]\n/rewind - Restore the code and/or conversation to a previous point\n  Aliases: /checkpoint\n/skills - List available skills\n/tag - Toggle a searchable tag on the current session\n  Usage: /tag <tag-name>\n/color - Set the prompt bar color for this session\n  Usage: /color <color|default>\n/passes - Share a free week of Claude Code with friends\n/sandbox - Configure sandbox settings\n  Usage: /sandbox [exclude <command-pattern>]\n/stickers - Order Claude Code stickers\n/privacy-settings - View and update your privacy settings\n/plan - Enable plan mode or view the current session plan\n  Usage: /plan [open|<description>]\n/tasks - List and manage background tasks\n  Aliases: /bashes\n/diff - View uncommitted changes and per-turn diffs\n/files - List all files currently in context\n/copy - Copy Claude's last response to clipboard (or /copy N for the Nth-latest)\n  Usage: /copy [N]\n/export - Export the current conversation to a file or clipboard\n  Usage: /export [filename]\n/version - Print the version this session is running (not what autoupdate downloaded)\n/release-notes - View release notes\n/upgrade - Upgrade to Max for higher rate limits and more Opus\n/usage - Show plan usage limits\n/stats - Show your Claude Code usage statistics and activity\n/extra-usage - Configure extra usage to keep working when limits are hit\n/theme - Change the theme\n  Usage: /theme <auto|dark|light|light-daltonized|dark-daltonized|light-ansi|dark-ansi>\n/vim - Toggle between Vim and Normal editing modes\n/terminal-setup - Install Shift+Enter key binding for newlines\n/keybindings - Open or create your keybindings configuration file\n/btw - Ask a quick side question without interrupting the main conversation\n  Usage: /btw <question>\n/chrome - Claude in Chrome (Beta) settings\n/think-back - Your 2025 Claude Code Year in Review\n/reload-plugins - Activate pending plugin changes in the current session\n/advisor - Configure the advisor model\n  Usage: /advisor [<model>|off]\n/statusline - Set up Claude Code's status line UI\n  Usage: /statusline [prompt]\n/ultrareview - Find and verify bugs in your branch using Claude Code on the web\n  Usage: /ultrareview [pr-number]\n/insights - Generate a report analyzing your Claude Code sessions\n  Usage: /insights [--homespaces]\n/remote-control - Connect this terminal for remote-control sessions\n  Aliases: /rc\n  Usage: /remote-control [name]\n/commit - Create a git commit\n/commit-push-pr - Commit, push, and open a PR\n  Usage: /commit-push-pr [instructions]\n/web-setup - Setup Claude Code on the web (requires connecting your GitHub account)\n/seed-sessions - Insert demo persisted sessions for /resume testing\n/agents - Manage agent configurations\n/plugin - Manage Claude Code plugins\n  Aliases: /plugins, /marketplace\n  Usage: /plugin [subcommand]\n/hooks - View hook configurations for tool events\nSend plain text without a leading slash to start a normal prompt."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
	if strings.Contains(result.Output, "/rate-limit-options -") {
		t.Fatalf("Execute() output unexpectedly includes hidden /rate-limit-options command: %q", result.Output)
	}
	if strings.Contains(result.Output, "/thinkback-play -") {
		t.Fatalf("Execute() output unexpectedly includes hidden /thinkback-play command: %q", result.Output)
	}
	if strings.Contains(result.Output, "/bridge-kick -") {
		t.Fatalf("Execute() output unexpectedly includes hidden /bridge-kick command: %q", result.Output)
	}
	if strings.Contains(result.Output, "/heapdump -") {
		t.Fatalf("Execute() output unexpectedly includes hidden /heapdump command: %q", result.Output)
	}
}
