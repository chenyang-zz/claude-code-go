package repl

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
	servicecommands "github.com/sheepzhao/claude-code-go/internal/services/commands"
	"github.com/sheepzhao/claude-code-go/internal/ui/console"
)

type recordingEngine struct {
	lastRequest conversation.RunRequest
	stream      event.Stream
}

type stubWorktreeLister struct {
	paths []string
	err   error
}

type stubEditorModeStore struct {
	saved []string
}

type stubThemeStore struct {
	saved []string
}

type stubModelStore struct {
	saved []string
}

type stubAdditionalDirectoryStore struct {
	saved []string
}

type recordingSessionRepository struct {
	loadResult   coresession.Session
	loadErr      error
	latestResult coresession.Session
	latestErr    error
	listRecent   []coresession.Summary
	listErr      error
	searchResult []coresession.Summary
	searchErr    error
	titleResult  []coresession.Summary
	titleErr     error
	saved        []coresession.Session
}

func (e *recordingEngine) Run(ctx context.Context, req conversation.RunRequest) (event.Stream, error) {
	e.lastRequest = req
	return e.stream, nil
}

func (s stubWorktreeLister) ListWorktrees(ctx context.Context, cwd string) ([]string, error) {
	_ = ctx
	_ = cwd
	return append([]string(nil), s.paths...), s.err
}

func (s *stubEditorModeStore) SaveEditorMode(ctx context.Context, mode string) error {
	_ = ctx
	s.saved = append(s.saved, mode)
	return nil
}

func (s *stubThemeStore) SaveTheme(ctx context.Context, theme string) error {
	_ = ctx
	s.saved = append(s.saved, theme)
	return nil
}

func (s *stubModelStore) SaveModel(ctx context.Context, model string) error {
	_ = ctx
	s.saved = append(s.saved, model)
	return nil
}

func (s *stubAdditionalDirectoryStore) AddAdditionalDirectory(ctx context.Context, directory string) error {
	_ = ctx
	s.saved = append(s.saved, directory)
	return nil
}

func (r *recordingSessionRepository) Save(ctx context.Context, session coresession.Session) error {
	_ = ctx
	r.saved = append(r.saved, session.Clone())
	return nil
}

func (r *recordingSessionRepository) Load(ctx context.Context, id string) (coresession.Session, error) {
	_ = ctx
	if r.loadErr != nil {
		return coresession.Session{}, r.loadErr
	}
	for i := len(r.saved) - 1; i >= 0; i-- {
		if r.saved[i].ID == id {
			return r.saved[i].Clone(), nil
		}
	}
	if r.latestResult.ID == id {
		return r.latestResult.Clone(), nil
	}
	if r.loadResult.ID == id {
		return r.loadResult.Clone(), nil
	}
	return coresession.Session{}, coresession.ErrSessionNotFound
}

func (r *recordingSessionRepository) LoadLatest(ctx context.Context, lookup coresession.Lookup) (coresession.Session, error) {
	_ = ctx
	if r.latestErr != nil {
		return coresession.Session{}, r.latestErr
	}
	for i := len(r.saved) - 1; i >= 0; i-- {
		if r.saved[i].ProjectPath == lookup.ProjectPath {
			return r.saved[i].Clone(), nil
		}
	}
	if r.latestResult.ID != "" || len(r.latestResult.Messages) != 0 || r.latestResult.ProjectPath != "" {
		if r.latestResult.ProjectPath != lookup.ProjectPath {
			return coresession.Session{}, coresession.ErrSessionNotFound
		}
		return r.latestResult.Clone(), nil
	}
	if r.loadErr != nil {
		return coresession.Session{}, r.loadErr
	}
	return r.loadResult.Clone(), nil
}

func (r *recordingSessionRepository) ListRecent(ctx context.Context, lookup coresession.Lookup) ([]coresession.Summary, error) {
	_ = ctx
	if r.listErr != nil {
		return nil, r.listErr
	}
	var filtered []coresession.Summary
	for _, summary := range r.listRecent {
		if !lookup.AllProjects && summary.ProjectPath != lookup.ProjectPath {
			continue
		}
		filtered = append(filtered, summary)
		if lookup.Limit > 0 && len(filtered) == lookup.Limit {
			break
		}
	}
	return filtered, nil
}

func (r *recordingSessionRepository) Search(ctx context.Context, lookup coresession.Lookup) ([]coresession.Summary, error) {
	_ = ctx
	if r.searchErr != nil {
		return nil, r.searchErr
	}
	var filtered []coresession.Summary
	for _, summary := range r.searchResult {
		if !lookup.AllProjects && summary.ProjectPath != lookup.ProjectPath {
			continue
		}
		filtered = append(filtered, summary)
		if lookup.Limit > 0 && len(filtered) == lookup.Limit {
			break
		}
	}
	return filtered, nil
}

func (r *recordingSessionRepository) FindByCustomTitle(ctx context.Context, lookup coresession.Lookup) ([]coresession.Summary, error) {
	_ = ctx
	if r.titleErr != nil {
		return nil, r.titleErr
	}
	var filtered []coresession.Summary
	for _, summary := range r.titleResult {
		if !lookup.AllProjects && summary.ProjectPath != lookup.ProjectPath {
			continue
		}
		filtered = append(filtered, summary)
		if lookup.Limit > 0 && len(filtered) == lookup.Limit {
			break
		}
	}
	return filtered, nil
}

func (r *recordingSessionRepository) UpdateCustomTitle(ctx context.Context, id string, title string) error {
	_ = ctx
	for index := range r.saved {
		if r.saved[index].ID == id {
			r.saved[index].CustomTitle = title
			return nil
		}
	}
	if r.loadResult.ID == id {
		r.loadResult.CustomTitle = title
		return nil
	}
	return coresession.ErrSessionNotFound
}

var _ engine.Engine = (*recordingEngine)(nil)

type staticCommand struct {
	meta   command.Metadata
	result command.Result
}

func (c staticCommand) Metadata() command.Metadata {
	return c.meta
}

func (c staticCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args
	return c.result, nil
}

func registerSlashCommands(t *testing.T, runner *Runner, commands ...command.Command) {
	t.Helper()

	registry := command.NewInMemoryRegistry()
	for _, cmd := range commands {
		if err := registry.Register(cmd); err != nil {
			t.Fatalf("Register(%q) error = %v", cmd.Metadata().Name, err)
		}
	}
	runner.Commands = registry
}

// TestParseArgsParsesSlashCommand verifies slash input is split into command and body.
func TestParseArgsParsesSlashCommand(t *testing.T) {
	parsed, err := ParseArgs([]string{"/help", "topic"})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if !parsed.IsSlashCommand || parsed.Command != "help" || parsed.Body != "topic" {
		t.Fatalf("ParseArgs() = %#v, want slash command help with body topic", parsed)
	}
}

// TestParseArgsParsesContinueFlags verifies the minimum continue/fork flags are peeled off before prompt parsing.
func TestParseArgsParsesContinueFlags(t *testing.T) {
	parsed, err := ParseArgs([]string{"--fork-session", "--continue", "follow", "up"})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if !parsed.ContinueLatest {
		t.Fatal("ParseArgs() ContinueLatest = false, want true")
	}
	if !parsed.ForkSession {
		t.Fatal("ParseArgs() ForkSession = false, want true")
	}
	if parsed.Body != "follow up" {
		t.Fatalf("ParseArgs() body = %q, want %q", parsed.Body, "follow up")
	}
}

// TestParseArgsRejectsForkWithoutExplicitRecovery verifies fork mode is only accepted for continue/resume recovery flows.
func TestParseArgsRejectsForkWithoutExplicitRecovery(t *testing.T) {
	_, err := ParseArgs([]string{"--fork-session", "follow", "up"})
	if err == nil {
		t.Fatal("ParseArgs() error = nil, want fork usage error")
	}
	if err.Error() != forkUsageMessage {
		t.Fatalf("ParseArgs() error = %q, want %q", err.Error(), forkUsageMessage)
	}
}

// TestRunnerRunForwardsPrompt verifies the REPL forwards real CLI input into the engine request.
func TestRunnerRunForwardsPrompt(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "hello",
		},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("say hello")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("hello")}},
				},
			},
		},
	}
	close(stream)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))

	if err := runner.Run(context.Background(), []string{"say", "hello"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.Input != "" {
		t.Fatalf("Run() request.Input = %q, want empty input because REPL sends full message history", eng.lastRequest.Input)
	}
	if len(eng.lastRequest.Messages) != 1 || eng.lastRequest.Messages[0].Content[0].Text != "say hello" {
		t.Fatalf("Run() request.Messages = %#v, want one user message with say hello", eng.lastRequest.Messages)
	}

	if got := buf.String(); got != "hello" {
		t.Fatalf("Run() output = %q, want %q", got, "hello")
	}
}

// TestRunnerRunDispatchesRegisteredSlashCommand verifies slash commands are dispatched through the shared registry.
func TestRunnerRunDispatchesRegisteredSlashCommand(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, staticCommand{
		meta: command.Metadata{
			Name:        "help",
			Description: "show help",
			Usage:       "/help",
		},
		result: command.Result{Output: "available commands"},
	})

	if err := runner.Run(context.Background(), []string{"/help"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.Input != "" {
		t.Fatalf("engine should not be called for slash command, got request %#v", eng.lastRequest)
	}

	if got := buf.String(); got != "available commands\n" {
		t.Fatalf("Run() output = %q, want registered slash command output", got)
	}
}

// TestRunnerRunDispatchesSlashAlias verifies slash aliases resolve through the shared registry lookup path.
func TestRunnerRunDispatchesSlashAlias(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, staticCommand{
		meta: command.Metadata{
			Name:        "permissions",
			Aliases:     []string{"allowed-tools"},
			Description: "Manage allow & deny tool permission rules",
			Usage:       "/permissions",
		},
		result: command.Result{Output: "Permission settings:\n- Default mode: default"},
	})

	if err := runner.Run(context.Background(), []string{"/allowed-tools"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.Input != "" {
		t.Fatalf("engine should not be called for slash alias, got request %#v", eng.lastRequest)
	}
	if got := buf.String(); got != "Permission settings:\n- Default mode: default\n" {
		t.Fatalf("Run() output = %q, want registered slash alias output", got)
	}
}

// TestRunnerRunHelpCommandListsRegisteredCommands verifies the real /help command output matches the registry-backed support surface.
func TestRunnerRunHelpCommandListsRegisteredCommands(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))

	registry := command.NewInMemoryRegistry()
	if err := registry.Register(servicecommands.HelpCommand{Registry: registry}); err != nil {
		t.Fatalf("Register(help) error = %v", err)
	}
	if err := registry.Register(servicecommands.ClearCommand{}); err != nil {
		t.Fatalf("Register(clear) error = %v", err)
	}
	if err := registry.Register(servicecommands.CompactCommand{}); err != nil {
		t.Fatalf("Register(compact) error = %v", err)
	}
	if err := registry.Register(servicecommands.MemoryCommand{}); err != nil {
		t.Fatalf("Register(memory) error = %v", err)
	}
	if err := registry.Register(NewResumeCommandAdapter(runner)); err != nil {
		t.Fatalf("Register(resume) error = %v", err)
	}
	if err := registry.Register(NewRenameCommandAdapter(runner)); err != nil {
		t.Fatalf("Register(rename) error = %v", err)
	}
	if err := registry.Register(servicecommands.ConfigCommand{}); err != nil {
		t.Fatalf("Register(config) error = %v", err)
	}
	if err := registry.Register(servicecommands.ModelCommand{}); err != nil {
		t.Fatalf("Register(model) error = %v", err)
	}
	if err := registry.Register(servicecommands.FastCommand{}); err != nil {
		t.Fatalf("Register(fast) error = %v", err)
	}
	if err := registry.Register(servicecommands.EffortCommand{}); err != nil {
		t.Fatalf("Register(effort) error = %v", err)
	}
	if err := registry.Register(servicecommands.OutputStyleCommand{}); err != nil {
		t.Fatalf("Register(output-style) error = %v", err)
	}
	if err := registry.Register(servicecommands.DoctorCommand{}); err != nil {
		t.Fatalf("Register(doctor) error = %v", err)
	}
	if err := registry.Register(servicecommands.PermissionsCommand{}); err != nil {
		t.Fatalf("Register(permissions) error = %v", err)
	}
	if err := registry.Register(servicecommands.AddDirCommand{}); err != nil {
		t.Fatalf("Register(add-dir) error = %v", err)
	}
	if err := registry.Register(servicecommands.LoginCommand{}); err != nil {
		t.Fatalf("Register(login) error = %v", err)
	}
	if err := registry.Register(servicecommands.LogoutCommand{}); err != nil {
		t.Fatalf("Register(logout) error = %v", err)
	}
	if err := registry.Register(servicecommands.CostCommand{}); err != nil {
		t.Fatalf("Register(cost) error = %v", err)
	}
	if err := registry.Register(servicecommands.StatusCommand{}); err != nil {
		t.Fatalf("Register(status) error = %v", err)
	}
	if err := registry.Register(servicecommands.MCPCommand{}); err != nil {
		t.Fatalf("Register(mcp) error = %v", err)
	}
	if err := registry.Register(servicecommands.SessionCommand{}); err != nil {
		t.Fatalf("Register(session) error = %v", err)
	}
	if err := registry.Register(servicecommands.FilesCommand{}); err != nil {
		t.Fatalf("Register(files) error = %v", err)
	}
	if err := registry.Register(servicecommands.CopyCommand{}); err != nil {
		t.Fatalf("Register(copy) error = %v", err)
	}
	if err := registry.Register(servicecommands.ExportCommand{}); err != nil {
		t.Fatalf("Register(export) error = %v", err)
	}
	if err := registry.Register(servicecommands.VersionCommand{}); err != nil {
		t.Fatalf("Register(version) error = %v", err)
	}
	if err := registry.Register(servicecommands.ReleaseNotesCommand{}); err != nil {
		t.Fatalf("Register(release-notes) error = %v", err)
	}
	if err := registry.Register(servicecommands.UpgradeCommand{}); err != nil {
		t.Fatalf("Register(upgrade) error = %v", err)
	}
	if err := registry.Register(servicecommands.UsageCommand{}); err != nil {
		t.Fatalf("Register(usage) error = %v", err)
	}
	if err := registry.Register(servicecommands.StatsCommand{}); err != nil {
		t.Fatalf("Register(stats) error = %v", err)
	}
	if err := registry.Register(servicecommands.ExtraUsageCommand{}); err != nil {
		t.Fatalf("Register(extra-usage) error = %v", err)
	}
	if err := registry.Register(servicecommands.ThemeCommand{}); err != nil {
		t.Fatalf("Register(theme) error = %v", err)
	}
	if err := registry.Register(servicecommands.VimCommand{}); err != nil {
		t.Fatalf("Register(vim) error = %v", err)
	}
	if err := registry.Register(servicecommands.TerminalSetupCommand{}); err != nil {
		t.Fatalf("Register(terminal-setup) error = %v", err)
	}
	if err := registry.Register(servicecommands.KeybindingsCommand{}); err != nil {
		t.Fatalf("Register(keybindings) error = %v", err)
	}
	if err := registry.Register(servicecommands.PRCommentsCommand{}); err != nil {
		t.Fatalf("Register(pr-comments) error = %v", err)
	}
	if err := registry.Register(servicecommands.SecurityReviewCommand{}); err != nil {
		t.Fatalf("Register(security-review) error = %v", err)
	}
	if err := registry.Register(servicecommands.SeedSessionsCommand{}); err != nil {
		t.Fatalf("Register(seed-sessions) error = %v", err)
	}
	runner.Commands = registry

	if err := runner.Run(context.Background(), []string{"/help"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := "Available commands:\n/help - Show help and available commands\n/clear - Clear conversation history and start a new session\n/compact - Clear conversation history but keep a summary in context\n  Usage: /compact [instructions]\n/memory - Edit Claude memory files\n/resume - Resume a saved session by search or continue it with a new prompt\n  Aliases: /continue\n  Usage: /resume <search-term> | /resume <session-id> <prompt>\n/rename - Rename the current conversation for easier resume discovery\n  Usage: /rename <title>\n/config - Show the current runtime configuration\n  Aliases: /settings\n/model - Change the model\n  Usage: /model [model]\n/fast - Toggle fast mode (Opus 4.6 only)\n  Usage: /fast [on|off]\n/effort - Set effort level for model usage\n  Usage: /effort [low|medium|high|max|auto]\n/output-style - Deprecated: use /config to change output style\n/doctor - Diagnose the current Claude Code Go host setup\n/permissions - Manage allow & deny tool permission rules\n  Aliases: /allowed-tools\n/add-dir - Add a new working directory\n  Usage: /add-dir <path>\n/login - Sign in with your Anthropic account\n/logout - Sign out from your Anthropic account\n/cost - Show the total cost and duration of the current session\n/status - Show Claude Code status including version, model, account, API connectivity, and tool statuses\n/mcp - Manage MCP servers\n  Usage: /mcp [enable|disable <server-name>]\n/session - Show remote session URL and QR code\n/files - List all files currently in context\n/copy - Copy Claude's last response to clipboard (or /copy N for the Nth-latest)\n  Usage: /copy [N]\n/export - Export the current conversation to a file or clipboard\n  Usage: /export [filename]\n/version - Print the version this session is running (not what autoupdate downloaded)\n/release-notes - View release notes\n/upgrade - Upgrade to Max for higher rate limits and more Opus\n/usage - Show plan usage limits\n/stats - Show your Claude Code usage statistics and activity\n/extra-usage - Configure extra usage to keep working when limits are hit\n/theme - Change the theme\n  Usage: /theme <auto|dark|light|light-daltonized|dark-daltonized|light-ansi|dark-ansi>\n/vim - Toggle between Vim and Normal editing modes\n/terminal-setup - Install Shift+Enter key binding for newlines\n/keybindings - Open or create your keybindings configuration file\n/pr-comments - Get comments from a GitHub pull request\n/security-review - Complete a security review of the pending changes on the current branch\n/seed-sessions - Insert demo persisted sessions for /resume testing\nSend plain text without a leading slash to start a normal prompt.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run() output = %q, want %q", got, want)
	}
}

// TestRunnerRunMemoryCommandReportsFallback verifies /memory is routed through the shared registry and emits the stable memory fallback.
func TestRunnerRunMemoryCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.MemoryCommand{})

	if err := runner.Run(context.Background(), []string{"/memory"}); err != nil {
		t.Fatalf("Run(/memory) error = %v", err)
	}

	want := "Memory file editing is not available in Claude Code Go yet. Memory file discovery, interactive selection, file creation, and editor launch remain unmigrated.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/memory) output = %q, want %q", got, want)
	}
}

// TestRunnerRunTerminalSetupCommandReportsFallback verifies /terminal-setup is routed through the shared registry and emits the stable guidance fallback.
func TestRunnerRunTerminalSetupCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.TerminalSetupCommand{})

	if err := runner.Run(context.Background(), []string{"/terminal-setup"}); err != nil {
		t.Fatalf("Run(/terminal-setup) error = %v", err)
	}

	want := "Terminal setup automation is not available in Claude Code Go yet. Shift+Enter shortcut installation, terminal-specific config writes, backup/restore, remote-session guidance, and shell completion setup remain unmigrated.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/terminal-setup) output = %q, want %q", got, want)
	}
}

// TestRunnerRunKeybindingsCommandReportsFallback verifies /keybindings is routed through the shared registry and emits the stable guidance fallback.
func TestRunnerRunKeybindingsCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.KeybindingsCommand{})

	if err := runner.Run(context.Background(), []string{"/keybindings"}); err != nil {
		t.Fatalf("Run(/keybindings) error = %v", err)
	}

	want := "Keybindings file management is not available in Claude Code Go yet. Keybindings file discovery, template creation, file writes, and editor launch remain unmigrated.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/keybindings) output = %q, want %q", got, want)
	}
}

// TestRunnerRunCompactCommandReportsFallback verifies /compact is routed through the shared registry and emits the stable compaction fallback.
func TestRunnerRunCompactCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.CompactCommand{})

	if err := runner.Run(context.Background(), []string{"/compact", "keep", "latest", "decisions"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := "Conversation compaction is not available in Claude Code Go yet. Use /clear to start a new session; summary-preserving compact, custom instructions, and compact hooks remain unmigrated.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/compact) output = %q, want %q", got, want)
	}
}

// TestRunnerRunLoginCommandReportsAuthFallback verifies /login is routed through the shared registry and emits the stable auth fallback.
func TestRunnerRunLoginCommandReportsAuthFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.LoginCommand{})

	if err := runner.Run(context.Background(), []string{"/login"}); err != nil {
		t.Fatalf("Run(/login) error = %v", err)
	}

	want := "Interactive Anthropic account login is not supported in Claude Code Go yet. Configure an API key in settings or environment variables instead.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/login) output = %q, want %q", got, want)
	}
}

// TestRunnerRunPRCommentsCommandReportsPluginNotice verifies /pr-comments is routed through the shared registry and emits the stable plugin guidance.
func TestRunnerRunPRCommentsCommandReportsPluginNotice(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.PRCommentsCommand{})

	if err := runner.Run(context.Background(), []string{"/pr-comments"}); err != nil {
		t.Fatalf("Run(/pr-comments) error = %v", err)
	}

	want := "This command has moved to a plugin.\n\n1. Install it with: claude plugin install pr-comments@claude-code-marketplace\n2. Then run: /pr-comments:pr-comments\n3. More information: https://github.com/anthropics/claude-code-marketplace/blob/main/pr-comments/README.md\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/pr-comments) output = %q, want %q", got, want)
	}
}

// TestRunnerRunSecurityReviewCommandReportsPluginNotice verifies /security-review is routed through the shared registry and emits the stable plugin guidance.
func TestRunnerRunSecurityReviewCommandReportsPluginNotice(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.SecurityReviewCommand{})

	if err := runner.Run(context.Background(), []string{"/security-review"}); err != nil {
		t.Fatalf("Run(/security-review) error = %v", err)
	}

	want := "This command has moved to a plugin.\n\n1. Install it with: claude plugin install security-review@claude-code-marketplace\n2. Then run: /security-review:security-review\n3. More information: https://github.com/anthropics/claude-code-marketplace/blob/main/security-review/README.md\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/security-review) output = %q, want %q", got, want)
	}
}

// TestRunnerRunLogoutCommandReportsFallback verifies /logout is routed through the shared registry and emits the stable logout fallback.
func TestRunnerRunLogoutCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.LogoutCommand{})

	if err := runner.Run(context.Background(), []string{"/logout"}); err != nil {
		t.Fatalf("Run(/logout) error = %v", err)
	}

	want := "Interactive Anthropic account logout is not supported in Claude Code Go yet because account login is not available.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/logout) output = %q, want %q", got, want)
	}
}

// TestRunnerRunCostCommandReportsFallback verifies /cost is routed through the shared registry and emits the stable usage fallback.
func TestRunnerRunCostCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.CostCommand{})

	if err := runner.Run(context.Background(), []string{"/cost"}); err != nil {
		t.Fatalf("Run(/cost) error = %v", err)
	}

	want := "Session cost and duration tracking are not available in Claude Code Go yet.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/cost) output = %q, want %q", got, want)
	}
}

// TestRunnerRunStatusCommandReportsFallback verifies /status is routed through the shared registry and emits the stable status summary.
func TestRunnerRunStatusCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.StatusCommand{})

	if err := runner.Run(context.Background(), []string{"/status"}); err != nil {
		t.Fatalf("Run(/status) error = %v", err)
	}

	want := "Status summary:\n- Provider: (not set)\n- Model: (not set)\n- Project path: (not set)\n- Approval mode: (not set)\n- Session storage: not configured\n- Account auth: missing API key; interactive account status is not available\n- API base URL: default\n- API connectivity check: not available in Claude Code Go yet\n- Tool status checks: not available in Claude Code Go yet\n- Settings status UI: not available in Claude Code Go yet\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/status) output = %q, want %q", got, want)
	}
}

// TestRunnerRunMCPCommandReportsFallback verifies /mcp is routed through the shared registry and emits the stable MCP fallback.
func TestRunnerRunMCPCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.MCPCommand{})

	if err := runner.Run(context.Background(), []string{"/mcp"}); err != nil {
		t.Fatalf("Run(/mcp) error = %v", err)
	}

	want := "MCP server management is not available in Claude Code Go yet. Configure MCP servers before startup instead.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/mcp) output = %q, want %q", got, want)
	}
}

// TestRunnerRunSessionCommandReportsRemoteFallback verifies /session is routed through the shared registry and emits the stable non-remote message.
func TestRunnerRunSessionCommandReportsRemoteFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.SessionCommand{})

	if err := runner.Run(context.Background(), []string{"/session"}); err != nil {
		t.Fatalf("Run(/session) error = %v", err)
	}

	want := "Not in remote mode. Start with `claude --remote` to use this command.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/session) output = %q, want %q", got, want)
	}
}

// TestRunnerRunFilesCommandReportsFallback verifies /files is routed through the shared registry and emits the stable file-context fallback.
func TestRunnerRunFilesCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.FilesCommand{})

	if err := runner.Run(context.Background(), []string{"/files"}); err != nil {
		t.Fatalf("Run(/files) error = %v", err)
	}

	want := "File context listing is not available in Claude Code Go yet.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/files) output = %q, want %q", got, want)
	}
}

// TestRunnerRunCopyCommandReportsFallback verifies /copy is routed through the shared registry and emits the stable clipboard fallback.
func TestRunnerRunCopyCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.CopyCommand{})

	if err := runner.Run(context.Background(), []string{"/copy"}); err != nil {
		t.Fatalf("Run(/copy) error = %v", err)
	}

	want := "Copying Claude's last response is not available in Claude Code Go yet.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/copy) output = %q, want %q", got, want)
	}
}

// TestRunnerRunExportCommandReportsFallback verifies /export is routed through the shared registry and emits the stable export fallback.
func TestRunnerRunExportCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.ExportCommand{})

	if err := runner.Run(context.Background(), []string{"/export"}); err != nil {
		t.Fatalf("Run(/export) error = %v", err)
	}

	want := "Conversation export is not available in Claude Code Go yet.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/export) output = %q, want %q", got, want)
	}
}

// TestRunnerRunVersionCommandReportsBuildInfo verifies /version is routed through the shared registry and emits non-empty build metadata.
func TestRunnerRunVersionCommandReportsBuildInfo(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.VersionCommand{})

	if err := runner.Run(context.Background(), []string{"/version"}); err != nil {
		t.Fatalf("Run(/version) error = %v", err)
	}

	if strings.TrimSpace(buf.String()) == "" {
		t.Fatal("Run(/version) output = empty, want non-empty version string")
	}
}

// TestRunnerRunReleaseNotesCommandReportsFallback verifies /release-notes is routed through the shared registry and emits the stable changelog fallback.
func TestRunnerRunReleaseNotesCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.ReleaseNotesCommand{})

	if err := runner.Run(context.Background(), []string{"/release-notes"}); err != nil {
		t.Fatalf("Run(/release-notes) error = %v", err)
	}

	want := "Release notes fetching is not available in Claude Code Go yet. See the full changelog at: https://github.com/anthropics/claude-code/blob/main/CHANGELOG.md\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/release-notes) output = %q, want %q", got, want)
	}
}

// TestRunnerRunUpgradeCommandReportsFallback verifies /upgrade is routed through the shared registry and emits the stable upgrade fallback.
func TestRunnerRunUpgradeCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.UpgradeCommand{})

	if err := runner.Run(context.Background(), []string{"/upgrade"}); err != nil {
		t.Fatalf("Run(/upgrade) error = %v", err)
	}

	want := "Interactive upgrade flow is not available in Claude Code Go yet. Review Claude Max plans at https://claude.ai/upgrade/max. Browser launch, subscription detection, and post-upgrade login handoff remain unmigrated.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/upgrade) output = %q, want %q", got, want)
	}
}

// TestRunnerRunUsageCommandReportsFallback verifies /usage is routed through the shared registry and emits the stable usage-limit fallback.
func TestRunnerRunUsageCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.UsageCommand{})

	if err := runner.Run(context.Background(), []string{"/usage"}); err != nil {
		t.Fatalf("Run(/usage) error = %v", err)
	}

	want := "Plan usage limits UI is not available in Claude Code Go yet. Settings Usage tab rendering, account plan limit lookup, and consumer subscription state detection remain unmigrated.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/usage) output = %q, want %q", got, want)
	}
}

// TestRunnerRunStatsCommandReportsFallback verifies /stats is routed through the shared registry and emits the stable usage-statistics fallback.
func TestRunnerRunStatsCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.StatsCommand{})

	if err := runner.Run(context.Background(), []string{"/stats"}); err != nil {
		t.Fatalf("Run(/stats) error = %v", err)
	}

	want := "Usage statistics and activity views are not available in Claude Code Go yet. Aggregated usage history, activity panels, and analytics-backed summaries remain unmigrated.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/stats) output = %q, want %q", got, want)
	}
}

// TestRunnerRunExtraUsageCommandReportsFallback verifies /extra-usage is routed through the shared registry and emits the stable browser-flow fallback.
func TestRunnerRunExtraUsageCommandReportsFallback(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.ExtraUsageCommand{})

	if err := runner.Run(context.Background(), []string{"/extra-usage"}); err != nil {
		t.Fatalf("Run(/extra-usage) error = %v", err)
	}

	want := "Extra usage enrollment is not available in Claude Code Go yet. Browser launch, account overage management, and post-enrollment login handoff remain unmigrated.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/extra-usage) output = %q, want %q", got, want)
	}
}

// TestRunnerRunVimCommandPersistsEditorMode verifies /vim is routed through the shared registry and updates the stored mode.
// TestRunnerRunThemeCommandPersistsTheme verifies /theme is routed through the shared registry and updates the stored theme.
// TestRunnerRunModelCommandPersistsModel verifies /model is routed through the shared registry and updates the stored model.
func TestRunnerRunModelCommandPersistsModel(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	cfg := &coreconfig.Config{Model: coreconfig.DefaultConfig().Model}
	store := &stubModelStore{}
	registerSlashCommands(t, runner, servicecommands.ModelCommand{
		Config: cfg,
		Store:  store,
	})

	if err := runner.Run(context.Background(), []string{"/model", "claude-opus-4-1"}); err != nil {
		t.Fatalf("Run(/model) error = %v", err)
	}

	want := "Model set to claude-opus-4-1. Claude Code Go stores the preference now, but the interactive model picker and model availability checks are not implemented yet.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/model) output = %q, want %q", got, want)
	}
	if len(store.saved) != 1 || store.saved[0] != "claude-opus-4-1" {
		t.Fatalf("saved models = %#v, want []string{\"claude-opus-4-1\"}", store.saved)
	}
	if cfg.Model != "claude-opus-4-1" {
		t.Fatalf("config model = %q, want claude-opus-4-1", cfg.Model)
	}
}

// TestRunnerRunAddDirCommandPersistsDirectory verifies /add-dir is routed through the shared registry and updates read scope immediately.
func TestRunnerRunAddDirCommandPersistsDirectory(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))

	rootDir := t.TempDir()
	projectDir := filepath.Join(rootDir, "project")
	extraDir := filepath.Join(rootDir, "shared")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(extraDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	store := &stubAdditionalDirectoryStore{}
	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}
	cfg := &coreconfig.Config{ProjectPath: projectDir}
	registerSlashCommands(t, runner, servicecommands.AddDirCommand{
		Config: cfg,
		Store:  store,
		Policy: policy,
	})

	if err := runner.Run(context.Background(), []string{"/add-dir", "../shared"}); err != nil {
		t.Fatalf("Run(/add-dir) error = %v", err)
	}

	want := "Added " + extraDir + " as a working directory. Claude Code Go persists it to project settings now, but the interactive add-dir flow and session-only directory mode are not implemented yet.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/add-dir) output = %q, want %q", got, want)
	}
	if len(store.saved) != 1 || store.saved[0] != extraDir {
		t.Fatalf("saved directories = %#v, want %#v", store.saved, []string{extraDir})
	}
	if got := policy.CheckReadPermissionForTool(context.Background(), "file_read", filepath.Join(extraDir, "README.md"), projectDir).Decision; got != corepermission.DecisionAllow {
		t.Fatalf("policy decision = %q, want allow", got)
	}
}

func TestRunnerRunThemeCommandPersistsTheme(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	cfg := &coreconfig.Config{Theme: coreconfig.ThemeSettingDark}
	store := &stubThemeStore{}
	registerSlashCommands(t, runner, servicecommands.ThemeCommand{
		Config: cfg,
		Store:  store,
	})

	if err := runner.Run(context.Background(), []string{"/theme", "light"}); err != nil {
		t.Fatalf("Run(/theme) error = %v", err)
	}

	want := "Theme set to light. Claude Code Go stores the preference now, but the interactive theme picker and TUI theme rendering are not implemented yet.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/theme) output = %q, want %q", got, want)
	}
	if len(store.saved) != 1 || store.saved[0] != coreconfig.ThemeSettingLight {
		t.Fatalf("saved themes = %#v, want []string{\"light\"}", store.saved)
	}
}

// TestRunnerRunVimCommandPersistsEditorMode verifies /vim is routed through the shared registry and updates the stored mode.
func TestRunnerRunVimCommandPersistsEditorMode(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	cfg := &coreconfig.Config{EditorMode: coreconfig.EditorModeNormal}
	store := &stubEditorModeStore{}
	registerSlashCommands(t, runner, servicecommands.VimCommand{
		Config: cfg,
		Store:  store,
	})

	if err := runner.Run(context.Background(), []string{"/vim"}); err != nil {
		t.Fatalf("Run(/vim) error = %v", err)
	}

	want := "Editor mode set to vim. Claude Code Go stores the setting now, but prompt-editor Vim keybindings are not implemented yet.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/vim) output = %q, want %q", got, want)
	}
	if len(store.saved) != 1 || store.saved[0] != coreconfig.EditorModeVim {
		t.Fatalf("saved modes = %#v, want []string{\"vim\"}", store.saved)
	}
	if cfg.EditorMode != coreconfig.EditorModeVim {
		t.Fatalf("config editor mode = %q, want %q", cfg.EditorMode, coreconfig.EditorModeVim)
	}
}

// TestRunnerRunSettingsAliasDispatchesConfig verifies /settings resolves through the shared registry alias table.
func TestRunnerRunSettingsAliasDispatchesConfig(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, servicecommands.ConfigCommand{})

	if err := runner.Run(context.Background(), []string{"/settings"}); err != nil {
		t.Fatalf("Run(/settings) error = %v", err)
	}

	want := "Current configuration:\n- Provider: (not set)\n- Model: (not set)\n- Effort level: auto\n- Fast mode: off\n- Theme: dark\n- Editor mode: normal\n- Project path: (not set)\n- Approval mode: (not set)\n- Session DB path: (not set)\n- API key: missing\n- API base URL: default\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/settings) output = %q, want %q", got, want)
	}
}

// TestRunnerRunRenamePersistsCustomTitle verifies `/rename` stores the title on the active session.
func TestRunnerRunRenamePersistsCustomTitle(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{loadErr: coresession.ErrSessionNotFound}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewRenameCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/rename", "Deploy", "fix"}); err != nil {
		t.Fatalf("Run(/rename) error = %v", err)
	}

	if len(repo.saved) != 1 {
		t.Fatalf("saved count = %d, want 1", len(repo.saved))
	}
	if repo.saved[0].CustomTitle != "Deploy fix" {
		t.Fatalf("saved custom title = %q, want Deploy fix", repo.saved[0].CustomTitle)
	}
	if got := buf.String(); got != "Renamed conversation to \"Deploy fix\".\n" {
		t.Fatalf("Run(/rename) output = %q, want rename confirmation", got)
	}
}

// TestRunnerRunResumeWithoutArgsListsRecentSessions verifies bare /resume prints recent project-scoped sessions instead of only a usage error.
func TestRunnerRunResumeWithoutArgsListsRecentSessions(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{
		listRecent: []coresession.Summary{
			{ID: "session-3", ProjectPath: "/repo", Preview: "latest prompt", UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
			{ID: "session-2", ProjectPath: "/repo", UpdatedAt: time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC)},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume"}); err != nil {
		t.Fatalf("Run(/resume) error = %v", err)
	}

	want := "Recent conversations:\n- 2026-04-05 12:00 UTC | latest prompt [repo] | session-3\n- 2026-04-05 11:00 UTC | Previous conversation [repo] | session-2\nUse /resume <session-id> <prompt> to continue one.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/resume) output = %q, want %q", got, want)
	}
}

// TestRunnerRunResumeWithoutArgsIncludesOtherProjects verifies bare /resume now surfaces cross-project sessions with a stable hint.
func TestRunnerRunResumeWithoutArgsIncludesOtherProjects(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{
		listRecent: []coresession.Summary{
			{ID: "session-3", ProjectPath: "/repo", Preview: "latest prompt", UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
			{ID: "session-9", ProjectPath: "/other", Preview: "other repo prompt", UpdatedAt: time.Date(2026, 4, 5, 11, 30, 0, 0, time.UTC)},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume"}); err != nil {
		t.Fatalf("Run(/resume) error = %v", err)
	}

	want := "Recent conversations:\n- 2026-04-05 12:00 UTC | latest prompt [repo] | session-3\nOther projects:\n- 2026-04-05 11:30 UTC | other repo prompt [other] | session-9\nUse /resume <session-id> <prompt> to continue one.\nFor another project, change to that directory and use /resume <session-id> <prompt> there.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/resume) output = %q, want cross-project recent list", got)
	}
}

// TestRunnerRunResumeWithoutArgsSelectsConversation verifies bare /resume can read one numbered selection and switch sessions immediately.
func TestRunnerRunResumeWithoutArgsSelectsConversation(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{
		listRecent: []coresession.Summary{
			{ID: "session-3", ProjectPath: "/repo", Preview: "latest prompt", UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
			{ID: "session-2", ProjectPath: "/repo", Preview: "older prompt", UpdatedAt: time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC)},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.Input = strings.NewReader("2\n")
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume"}); err != nil {
		t.Fatalf("Run(/resume) error = %v", err)
	}

	if runner.SessionID != "session-2" {
		t.Fatalf("Run(/resume) session id = %q, want session-2", runner.SessionID)
	}
	want := "Recent conversations:\n1. 2026-04-05 12:00 UTC | latest prompt [repo] | session-3\n2. 2026-04-05 11:00 UTC | older prompt [repo] | session-2\nSelect a conversation number to resume, or press Enter to cancel.\nResumed conversation session-2.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/resume) output = %q, want picker selection confirmation", got)
	}
}

// TestRunnerRunResumeWithoutArgsCancelsSelection verifies an empty picker response exits without switching sessions.
func TestRunnerRunResumeWithoutArgsCancelsSelection(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{
		listRecent: []coresession.Summary{
			{ID: "session-3", ProjectPath: "/repo", Preview: "latest prompt", UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.Input = strings.NewReader("\n")
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume"}); err != nil {
		t.Fatalf("Run(/resume) error = %v", err)
	}

	if runner.SessionID != "" {
		t.Fatalf("Run(/resume) session id = %q, want empty after cancel", runner.SessionID)
	}
	want := "Recent conversations:\n1. 2026-04-05 12:00 UTC | latest prompt [repo] | session-3\nSelect a conversation number to resume, or press Enter to cancel.\nResume cancelled.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/resume) output = %q, want cancel confirmation", got)
	}
}

// TestRunnerRunResumeWithoutArgsSelectsSameRepoWorktree verifies picker selections reuse the same-repo worktree direct-resume path.
func TestRunnerRunResumeWithoutArgsSelectsSameRepoWorktree(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{
		listRecent: []coresession.Summary{
			{ID: "session-3", ProjectPath: "/repo", Preview: "latest prompt", UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
			{ID: "session-wt", ProjectPath: "/repo-wt/feature", Preview: "worktree prompt", UpdatedAt: time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC)},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.Input = strings.NewReader("2\n")
	runner.WorktreeLister = stubWorktreeLister{paths: []string{"/repo", "/repo-wt"}}
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume"}); err != nil {
		t.Fatalf("Run(/resume) error = %v", err)
	}

	if runner.SessionID != "session-wt" {
		t.Fatalf("Run(/resume) session id = %q, want session-wt", runner.SessionID)
	}
	if runner.ProjectPath != "/repo-wt/feature" {
		t.Fatalf("Run(/resume) project path = %q, want /repo-wt/feature", runner.ProjectPath)
	}
	want := "Recent conversations:\n1. 2026-04-05 12:00 UTC | latest prompt [repo] | session-3\nOther projects:\n2. 2026-04-05 11:00 UTC | worktree prompt [feature] | session-wt\nSelect a conversation number to resume, or press Enter to cancel.\nResumed conversation session-wt.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/resume) output = %q, want same-repo worktree resume", got)
	}
}

// TestRunnerRunResumeWithoutArgsSelectsOtherProjectShowsHint verifies picker selections still emit the stable cross-project command hint.
func TestRunnerRunResumeWithoutArgsSelectsOtherProjectShowsHint(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{
		listRecent: []coresession.Summary{
			{ID: "session-3", ProjectPath: "/repo", Preview: "latest prompt", UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
			{ID: "session-9", ProjectPath: "/other/repo", Preview: "other repo prompt", UpdatedAt: time.Date(2026, 4, 5, 11, 30, 0, 0, time.UTC)},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.Input = strings.NewReader("2\n")
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume"}); err != nil {
		t.Fatalf("Run(/resume) error = %v", err)
	}

	if runner.SessionID != "" {
		t.Fatalf("Run(/resume) session id = %q, want empty because cross-project selection should not switch session", runner.SessionID)
	}
	want := "Recent conversations:\n1. 2026-04-05 12:00 UTC | latest prompt [repo] | session-3\nOther projects:\n2. 2026-04-05 11:30 UTC | other repo prompt [repo] | session-9\nSelect a conversation number to resume, or press Enter to cancel.\nFound conversation session-9 in another project.\n- 2026-04-05 11:30 UTC | other repo prompt [repo] | session-9\nRun it from that project directory:\n  cd '/other/repo' && cc /resume session-9 <prompt>\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/resume) output = %q, want cross-project picker hint", got)
	}
}

// TestRunnerRunResumeSearchResolvesUniqueMatch verifies `/resume <search-term>` can switch the active session without requiring a prompt.
func TestRunnerRunResumeSearchResolvesUniqueMatch(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{
		searchResult: []coresession.Summary{
			{ID: "session-3", ProjectPath: "/repo", Preview: "deploy failure", UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume", "deploy"}); err != nil {
		t.Fatalf("Run(/resume deploy) error = %v", err)
	}

	if runner.SessionID != "session-3" {
		t.Fatalf("Run(/resume deploy) session id = %q, want session-3", runner.SessionID)
	}
	if got := buf.String(); got != "Resumed conversation session-3.\n" {
		t.Fatalf("Run(/resume deploy) output = %q, want resumed confirmation", got)
	}
}

// TestRunnerRunResumeSearchPrefersExactCustomTitle verifies exact title matches resume before preview-based search.
func TestRunnerRunResumeSearchPrefersExactCustomTitle(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{
		titleResult: []coresession.Summary{
			{ID: "session-7", ProjectPath: "/repo", CustomTitle: "Deploy fix", Preview: "unrelated preview", UpdatedAt: time.Date(2026, 4, 5, 13, 0, 0, 0, time.UTC)},
		},
		searchResult: []coresession.Summary{
			{ID: "session-3", ProjectPath: "/repo", Preview: "deploy failure", UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume", "Deploy", "fix"}); err != nil {
		t.Fatalf("Run(/resume title) error = %v", err)
	}

	if runner.SessionID != "session-7" {
		t.Fatalf("Run(/resume title) session id = %q, want session-7", runner.SessionID)
	}
	if got := buf.String(); got != "Resumed conversation session-7.\n" {
		t.Fatalf("Run(/resume title) output = %q, want resumed confirmation", got)
	}
}

// TestRunnerRunResumeSearchMatchesFuzzyCustomTitle verifies non-exact title queries can still restore a session through the shared search path.
func TestRunnerRunResumeSearchMatchesFuzzyCustomTitle(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{
		searchResult: []coresession.Summary{
			{ID: "session-8", ProjectPath: "/repo", CustomTitle: "Deploy retrospective", Preview: "unrelated preview", UpdatedAt: time.Date(2026, 4, 5, 13, 30, 0, 0, time.UTC)},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume", "deploy"}); err != nil {
		t.Fatalf("Run(/resume fuzzy title) error = %v", err)
	}

	if runner.SessionID != "session-8" {
		t.Fatalf("Run(/resume fuzzy title) session id = %q, want session-8", runner.SessionID)
	}
	if got := buf.String(); got != "Resumed conversation session-8.\n" {
		t.Fatalf("Run(/resume fuzzy title) output = %q, want resumed confirmation", got)
	}
}

// TestRunnerRunResumeSearchShowsCrossProjectHint verifies unique matches in another project produce a stable resume hint instead of switching projects implicitly.
func TestRunnerRunResumeSearchShowsCrossProjectHint(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{
		searchResult: []coresession.Summary{
			{ID: "session-9", ProjectPath: "/other/repo", Preview: "deploy failure", UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume", "deploy"}); err != nil {
		t.Fatalf("Run(/resume deploy) error = %v", err)
	}

	if runner.SessionID != "" {
		t.Fatalf("Run(/resume deploy) session id = %q, want empty because cross-project matches should not switch sessions", runner.SessionID)
	}
	want := "Found conversation session-9 in another project.\n- 2026-04-05 12:00 UTC | deploy failure [repo] | session-9\nRun it from that project directory:\n  cd '/other/repo' && cc /resume session-9 <prompt>\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/resume deploy) output = %q, want cross-project resume hint", got)
	}
}

// TestRunnerRunResumeSearchResumesSameRepoWorktree verifies unique same-repo worktree matches resume directly without a cd hint.
func TestRunnerRunResumeSearchResumesSameRepoWorktree(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{
		searchResult: []coresession.Summary{
			{ID: "session-wt", ProjectPath: "/repo-wt/feature", Preview: "deploy failure", UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = runtimesession.NewManager(repo)
	runner.WorktreeLister = stubWorktreeLister{paths: []string{"/repo", "/repo-wt"}}
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume", "deploy"}); err != nil {
		t.Fatalf("Run(/resume deploy) error = %v", err)
	}

	if runner.SessionID != "session-wt" {
		t.Fatalf("Run(/resume deploy) session id = %q, want session-wt", runner.SessionID)
	}
	if runner.ProjectPath != "/repo-wt/feature" {
		t.Fatalf("Run(/resume deploy) project path = %q, want /repo-wt/feature", runner.ProjectPath)
	}
	if got := buf.String(); got != "Resumed conversation session-wt.\n" {
		t.Fatalf("Run(/resume deploy) output = %q, want resumed confirmation", got)
	}
}

// TestRunnerRunResumeSearchShowsDisambiguation verifies multiple text matches are rendered as a stable text-only candidate list.
func TestRunnerRunResumeSearchShowsDisambiguation(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{
		searchResult: []coresession.Summary{
			{ID: "session-3", ProjectPath: "/repo", Preview: "deploy failure", UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
			{ID: "session-2", ProjectPath: "/repo", Preview: "deploy checklist", UpdatedAt: time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC)},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume", "deploy"}); err != nil {
		t.Fatalf("Run(/resume deploy) error = %v", err)
	}

	want := "Found 2 conversations matching deploy.\nMatching conversations:\n- 2026-04-05 12:00 UTC | deploy failure [repo] | session-3\n- 2026-04-05 11:00 UTC | deploy checklist [repo] | session-2\nUse /resume <session-id> <prompt> to continue one of them.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/resume deploy) output = %q, want search disambiguation list", got)
	}
}

// TestRunnerRunResumeSearchShowsCrossProjectDisambiguation verifies mixed-project multiple matches append the stable cross-project hint.
func TestRunnerRunResumeSearchShowsCrossProjectDisambiguation(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{
		searchResult: []coresession.Summary{
			{ID: "session-3", ProjectPath: "/repo", Preview: "deploy failure", UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
			{ID: "session-9", ProjectPath: "/other", Preview: "deploy checklist", UpdatedAt: time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC)},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume", "deploy"}); err != nil {
		t.Fatalf("Run(/resume deploy) error = %v", err)
	}

	want := "Found 2 conversations matching deploy.\nMatching conversations:\n- 2026-04-05 12:00 UTC | deploy failure [repo] | session-3\n- 2026-04-05 11:00 UTC | deploy checklist [other] | session-9\nUse /resume <session-id> <prompt> to continue one of them.\nFor another project, change to that directory and use /resume <session-id> <prompt> there.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/resume deploy) output = %q, want mixed-project disambiguation list", got)
	}
}

// TestRunnerRunContinueAliasDispatchesResume verifies /continue resolves through the shared registry alias table.
func TestRunnerRunContinueAliasDispatchesResume(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload:   event.MessageDeltaPayload{Text: "continued"},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("before")}},
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("new prompt")}},
				},
			},
		},
	}
	close(stream)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	repo := &recordingSessionRepository{
		loadResult: coresession.Session{
			ID:          "session-2",
			ProjectPath: "/repo",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("before")}},
			},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/continue", "session-2", "new", "prompt"}); err != nil {
		t.Fatalf("Run(/continue) error = %v", err)
	}
	if eng.lastRequest.SessionID != "session-2" {
		t.Fatalf("Run(/continue) session id = %q, want session-2", eng.lastRequest.SessionID)
	}
}

// TestRunnerRunClearStartsFreshSession verifies /clear switches the runner to a fresh session so the next prompt does not reuse old history.
func TestRunnerRunClearStartsFreshSession(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "fresh reply",
		},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("fresh prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("fresh reply")}},
				},
			},
		},
	}
	close(stream)

	repo := &recordingSessionRepository{
		loadResult: coresession.Session{
			ID: "session-old",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
			},
		},
	}
	manager := runtimesession.NewManager(repo)
	autosave := runtimesession.NewAutoSave(manager)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.SessionID = "session-old"
	runner.ProjectPath = "/repo"
	runner.SessionManager = manager
	runner.AutoSave = autosave
	registerSlashCommands(t, runner, servicecommands.ClearCommand{})

	if err := runner.Run(context.Background(), []string{"/clear"}); err != nil {
		t.Fatalf("Run(/clear) error = %v", err)
	}

	oldSessionID := "session-old"
	if runner.SessionID == "" || runner.SessionID == oldSessionID {
		t.Fatalf("Run(/clear) session id = %q, want fresh session id", runner.SessionID)
	}
	if got := buf.String(); got != "Started a new session.\n" {
		t.Fatalf("Run(/clear) output = %q, want clear confirmation", got)
	}

	buf.Reset()
	if err := runner.Run(context.Background(), []string{"fresh", "prompt"}); err != nil {
		t.Fatalf("Run(prompt after clear) error = %v", err)
	}

	if eng.lastRequest.SessionID != runner.SessionID {
		t.Fatalf("Run(prompt after clear) session id = %q, want %q", eng.lastRequest.SessionID, runner.SessionID)
	}
	if len(eng.lastRequest.Messages) != 1 || eng.lastRequest.Messages[0].Content[0].Text != "fresh prompt" {
		t.Fatalf("Run(prompt after clear) messages = %#v, want fresh prompt only", eng.lastRequest.Messages)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("autosave saved count = %d, want 1", len(repo.saved))
	}
	if repo.saved[0].ID != runner.SessionID {
		t.Fatalf("autosave saved session id = %q, want %q", repo.saved[0].ID, runner.SessionID)
	}
}

// TestRunnerRunClearUpdatesLatestSessionForContinue verifies /clear followed by autosave makes explicit --continue recover the fresh session instead of the old one.
func TestRunnerRunClearUpdatesLatestSessionForContinue(t *testing.T) {
	clearStream := make(chan event.Event, 2)
	clearStream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "fresh reply",
		},
	}
	clearStream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("fresh prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("fresh reply")}},
				},
			},
		},
	}
	close(clearStream)

	repo := &recordingSessionRepository{
		latestResult: coresession.Session{
			ID:          "session-old",
			ProjectPath: "/repo",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
			},
		},
	}
	manager := runtimesession.NewManager(repo)
	autosave := runtimesession.NewAutoSave(manager)

	var clearBuf bytes.Buffer
	clearEngine := &recordingEngine{stream: clearStream}
	clearRunner := NewRunner(clearEngine, console.NewStreamRenderer(console.NewPrinter(&clearBuf)))
	clearRunner.ProjectPath = "/repo"
	clearRunner.SessionID = "session-old"
	clearRunner.SessionManager = manager
	clearRunner.AutoSave = autosave
	registerSlashCommands(t, clearRunner, servicecommands.ClearCommand{})

	if err := clearRunner.Run(context.Background(), []string{"/clear"}); err != nil {
		t.Fatalf("Run(/clear) error = %v", err)
	}
	freshSessionID := clearRunner.SessionID
	if freshSessionID == "" || freshSessionID == "session-old" {
		t.Fatalf("Run(/clear) session id = %q, want fresh session id", freshSessionID)
	}

	clearBuf.Reset()
	if err := clearRunner.Run(context.Background(), []string{"fresh", "prompt"}); err != nil {
		t.Fatalf("Run(prompt after clear) error = %v", err)
	}

	continueStream := make(chan event.Event, 2)
	continueStream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "continued fresh reply",
		},
	}
	continueStream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("fresh prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("fresh reply")}},
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("follow-up")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("continued fresh reply")}},
				},
			},
		},
	}
	close(continueStream)

	var continueBuf bytes.Buffer
	continueEngine := &recordingEngine{stream: continueStream}
	continueRunner := NewRunner(continueEngine, console.NewStreamRenderer(console.NewPrinter(&continueBuf)))
	continueRunner.ProjectPath = "/repo"
	continueRunner.SessionManager = manager
	continueRunner.AutoSave = autosave

	if err := continueRunner.Run(context.Background(), []string{"--continue", "follow-up"}); err != nil {
		t.Fatalf("Run(--continue) error = %v", err)
	}

	if continueEngine.lastRequest.SessionID != freshSessionID {
		t.Fatalf("Run(--continue) session id = %q, want latest fresh session %q", continueEngine.lastRequest.SessionID, freshSessionID)
	}
	if len(continueEngine.lastRequest.Messages) != 3 {
		t.Fatalf("Run(--continue) request message count = %d, want 3", len(continueEngine.lastRequest.Messages))
	}
	if continueEngine.lastRequest.Messages[0].Content[0].Text != "fresh prompt" {
		t.Fatalf("Run(--continue) first message = %#v, want fresh prompt history", continueEngine.lastRequest.Messages[0])
	}
	if continueEngine.lastRequest.Messages[2].Content[0].Text != "follow-up" {
		t.Fatalf("Run(--continue) last message = %#v, want follow-up", continueEngine.lastRequest.Messages[2])
	}
}

// TestRunnerRunClearPreservesResumeOfOlderSession verifies /clear does not break explicit /resume recovery of an older saved session.
func TestRunnerRunClearPreservesResumeOfOlderSession(t *testing.T) {
	clearStream := make(chan event.Event, 2)
	clearStream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "fresh reply",
		},
	}
	clearStream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("fresh prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("fresh reply")}},
				},
			},
		},
	}
	close(clearStream)

	repo := &recordingSessionRepository{
		loadResult: coresession.Session{
			ID:          "session-old",
			ProjectPath: "/repo",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
			},
		},
	}
	manager := runtimesession.NewManager(repo)
	autosave := runtimesession.NewAutoSave(manager)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: clearStream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionID = "session-old"
	runner.SessionManager = manager
	runner.AutoSave = autosave
	registerSlashCommands(t, runner, servicecommands.ClearCommand{}, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/clear"}); err != nil {
		t.Fatalf("Run(/clear) error = %v", err)
	}
	if err := runner.Run(context.Background(), []string{"fresh", "prompt"}); err != nil {
		t.Fatalf("Run(prompt after clear) error = %v", err)
	}

	resumeStream := make(chan event.Event, 2)
	resumeStream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "resumed reply",
		},
	}
	resumeStream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("resume prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("resumed reply")}},
				},
			},
		},
	}
	close(resumeStream)
	eng.stream = resumeStream

	buf.Reset()
	if err := runner.Run(context.Background(), []string{"/resume", "session-old", "resume", "prompt"}); err != nil {
		t.Fatalf("Run(/resume after clear) error = %v", err)
	}

	if eng.lastRequest.SessionID != "session-old" {
		t.Fatalf("Run(/resume after clear) session id = %q, want session-old", eng.lastRequest.SessionID)
	}
	if len(eng.lastRequest.Messages) != 3 {
		t.Fatalf("Run(/resume after clear) request message count = %d, want 3", len(eng.lastRequest.Messages))
	}
	if eng.lastRequest.Messages[0].Content[0].Text != "old prompt" {
		t.Fatalf("Run(/resume after clear) first message = %#v, want old prompt history", eng.lastRequest.Messages[0])
	}
	if eng.lastRequest.Messages[2].Content[0].Text != "resume prompt" {
		t.Fatalf("Run(/resume after clear) last message = %#v, want resume prompt", eng.lastRequest.Messages[2])
	}
}

// TestRunnerRunResumeRestoresSessionAndRunsPrompt verifies /resume restores an existing session and continues it with the trailing prompt text.
func TestRunnerRunResumeRestoresSessionAndRunsPrompt(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "continued reply",
		},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("new prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("continued reply")}},
				},
			},
		},
	}
	close(stream)

	repo := &recordingSessionRepository{
		loadResult: coresession.Session{
			ID: "session-2",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
			},
		},
	}
	manager := runtimesession.NewManager(repo)
	autosave := runtimesession.NewAutoSave(manager)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.SessionManager = manager
	runner.AutoSave = autosave
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume", "session-2", "new", "prompt"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID != "session-2" {
		t.Fatalf("Run() request.SessionID = %q, want session-2", eng.lastRequest.SessionID)
	}
	if len(eng.lastRequest.Messages) != 3 {
		t.Fatalf("Run() request message count = %d, want 3", len(eng.lastRequest.Messages))
	}
	if eng.lastRequest.Messages[2].Content[0].Text != "new prompt" {
		t.Fatalf("Run() request last message = %#v, want new prompt", eng.lastRequest.Messages[2])
	}
	if len(repo.saved) != 1 {
		t.Fatalf("autosave saved count = %d, want 1", len(repo.saved))
	}
	if repo.saved[0].ID != "session-2" {
		t.Fatalf("autosave saved session id = %q, want session-2", repo.saved[0].ID)
	}
	if got := buf.String(); got != "continued reply" {
		t.Fatalf("Run() output = %q, want %q", got, "continued reply")
	}
}

// TestRunnerRunResumeSessionNotFound verifies /resume reports a stable error when the target session does not exist.
func TestRunnerRunResumeSessionNotFound(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	manager := runtimesession.NewManager(&recordingSessionRepository{loadErr: coresession.ErrSessionNotFound})
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.SessionManager = manager
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume", "missing-session", "hello"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID != "" || len(eng.lastRequest.Messages) != 0 {
		t.Fatalf("engine should not be called for missing session, got request %#v", eng.lastRequest)
	}
	if got := buf.String(); got != "Session missing-session was not found.\n" {
		t.Fatalf("Run() output = %q, want stable missing-session error", got)
	}
}

// TestRunnerRunResumeSearchRequiresStorage verifies search-style `/resume <term>` still reports the stable storage prerequisite.
func TestRunnerRunResumeSearchRequiresStorage(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume", "session-3"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID != "" || len(eng.lastRequest.Messages) != 0 {
		t.Fatalf("engine should not be called for invalid resume input, got request %#v", eng.lastRequest)
	}
	if got := buf.String(); got != resumeNotConfiguredMessage+"\n" {
		t.Fatalf("Run() output = %q, want resume storage error", got)
	}
}

// TestRunnerRunRestoresAndAutosavesHistory verifies the REPL bridges persisted session history into the engine and saves the completed history afterwards.
func TestRunnerRunRestoresAndAutosavesHistory(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "new reply",
		},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("new prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("new reply")}},
				},
			},
		},
	}
	close(stream)

	repo := &recordingSessionRepository{
		loadResult: coresession.Session{
			ID: "session-1",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
			},
		},
	}
	manager := runtimesession.NewManager(repo)
	autosave := runtimesession.NewAutoSave(manager)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.SessionID = "session-1"
	runner.SessionManager = manager
	runner.AutoSave = autosave

	if err := runner.Run(context.Background(), []string{"new", "prompt"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(eng.lastRequest.Messages) != 3 {
		t.Fatalf("Run() request message count = %d, want 3", len(eng.lastRequest.Messages))
	}
	if eng.lastRequest.Messages[2].Content[0].Text != "new prompt" {
		t.Fatalf("Run() request last message = %#v, want new prompt", eng.lastRequest.Messages[2])
	}
	if len(repo.saved) != 1 {
		t.Fatalf("autosave saved count = %d, want 1", len(repo.saved))
	}
	if len(repo.saved[0].Messages) != 4 {
		t.Fatalf("autosave saved message count = %d, want 4", len(repo.saved[0].Messages))
	}
}

// TestRunnerRunResumesLatestSessionForProject verifies normal prompt execution restores the latest session in the current project.
func TestRunnerRunResumesLatestSessionForProject(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "latest reply",
		},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("follow-up")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("latest reply")}},
				},
			},
		},
	}
	close(stream)

	repo := &recordingSessionRepository{
		latestResult: coresession.Session{
			ID:          "session-latest",
			ProjectPath: "/repo",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
			},
		},
	}
	manager := runtimesession.NewManager(repo)
	autosave := runtimesession.NewAutoSave(manager)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = manager
	runner.AutoSave = autosave

	if err := runner.Run(context.Background(), []string{"follow-up"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID != "session-latest" {
		t.Fatalf("Run() request.SessionID = %q, want session-latest", eng.lastRequest.SessionID)
	}
	if len(eng.lastRequest.Messages) != 3 {
		t.Fatalf("Run() request message count = %d, want 3", len(eng.lastRequest.Messages))
	}
	if len(repo.saved) != 1 {
		t.Fatalf("autosave saved count = %d, want 1", len(repo.saved))
	}
	if repo.saved[0].ID != "session-latest" {
		t.Fatalf("autosave saved session id = %q, want session-latest", repo.saved[0].ID)
	}
	if repo.saved[0].ProjectPath != "/repo" {
		t.Fatalf("autosave saved project path = %q, want /repo", repo.saved[0].ProjectPath)
	}
}

// TestRunnerRunStartsFreshProjectScopedSession verifies a missing latest session falls back to a new project-scoped session.
func TestRunnerRunStartsFreshProjectScopedSession(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "fresh reply",
		},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("fresh prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("fresh reply")}},
				},
			},
		},
	}
	close(stream)

	repo := &recordingSessionRepository{latestErr: coresession.ErrSessionNotFound}
	manager := runtimesession.NewManager(repo)
	autosave := runtimesession.NewAutoSave(manager)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = manager
	runner.AutoSave = autosave

	if err := runner.Run(context.Background(), []string{"fresh", "prompt"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID == "" {
		t.Fatal("Run() request.SessionID = empty, want generated session id")
	}
	if len(repo.saved) != 1 {
		t.Fatalf("autosave saved count = %d, want 1", len(repo.saved))
	}
	if repo.saved[0].ProjectPath != "/repo" {
		t.Fatalf("autosave saved project path = %q, want /repo", repo.saved[0].ProjectPath)
	}
}

// TestRunnerRunContinueRequiresExistingSession verifies explicit --continue reports a stable error instead of creating a new session.
func TestRunnerRunContinueRequiresExistingSession(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	manager := runtimesession.NewManager(&recordingSessionRepository{latestErr: coresession.ErrSessionNotFound})
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = manager

	if err := runner.Run(context.Background(), []string{"--continue", "follow-up"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID != "" || len(eng.lastRequest.Messages) != 0 {
		t.Fatalf("engine should not be called for missing continue session, got request %#v", eng.lastRequest)
	}
	if got := buf.String(); got != continueNotFoundMessage+"\n" {
		t.Fatalf("Run() output = %q, want continue missing-session error", got)
	}
}

// TestRunnerRunContinueForksLatestSession verifies explicit continue can fork the latest session into a new target id.
func TestRunnerRunContinueForksLatestSession(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "forked reply",
		},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("follow-up")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("forked reply")}},
				},
			},
		},
	}
	close(stream)

	repo := &recordingSessionRepository{
		latestResult: coresession.Session{
			ID:          "session-latest",
			ProjectPath: "/repo",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
			},
		},
	}
	manager := runtimesession.NewManager(repo)
	autosave := runtimesession.NewAutoSave(manager)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = manager
	runner.AutoSave = autosave

	if err := runner.Run(context.Background(), []string{"--fork-session", "--continue", "follow-up"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID == "" {
		t.Fatal("Run() request.SessionID = empty, want forked session id")
	}
	if eng.lastRequest.SessionID == "session-latest" {
		t.Fatalf("Run() request.SessionID = %q, want new forked session id", eng.lastRequest.SessionID)
	}
	if len(repo.saved) != 2 {
		t.Fatalf("saved count = %d, want 2 (fork + autosave)", len(repo.saved))
	}
	if repo.saved[0].ID != eng.lastRequest.SessionID {
		t.Fatalf("fork saved session id = %q, want %q", repo.saved[0].ID, eng.lastRequest.SessionID)
	}
	if repo.saved[1].ID != eng.lastRequest.SessionID {
		t.Fatalf("autosave saved session id = %q, want %q", repo.saved[1].ID, eng.lastRequest.SessionID)
	}
	if repo.saved[1].ProjectPath != "/repo" {
		t.Fatalf("autosave saved project path = %q, want /repo", repo.saved[1].ProjectPath)
	}
}

// TestRunnerRunResumeForksSession verifies /resume can clone history into a new session before continuing.
func TestRunnerRunResumeForksSession(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "forked resume reply",
		},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("new prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("forked resume reply")}},
				},
			},
		},
	}
	close(stream)

	repo := &recordingSessionRepository{
		loadResult: coresession.Session{
			ID:          "session-2",
			ProjectPath: "/repo",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
			},
		},
	}
	manager := runtimesession.NewManager(repo)
	autosave := runtimesession.NewAutoSave(manager)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.SessionManager = manager
	runner.AutoSave = autosave
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"--fork-session", "/resume", "session-2", "new", "prompt"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID == "" {
		t.Fatal("Run() request.SessionID = empty, want forked session id")
	}
	if eng.lastRequest.SessionID == "session-2" {
		t.Fatalf("Run() request.SessionID = %q, want new forked session id", eng.lastRequest.SessionID)
	}
	if len(repo.saved) != 2 {
		t.Fatalf("saved count = %d, want 2 (fork + autosave)", len(repo.saved))
	}
	if repo.saved[0].ID != eng.lastRequest.SessionID {
		t.Fatalf("fork saved session id = %q, want %q", repo.saved[0].ID, eng.lastRequest.SessionID)
	}
	if repo.saved[1].ID != eng.lastRequest.SessionID {
		t.Fatalf("autosave saved session id = %q, want %q", repo.saved[1].ID, eng.lastRequest.SessionID)
	}
}
