package bootstrap

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
)

type stubLoader struct {
	cfg coreconfig.Config
}

// TestNewAppWithDependenciesSeedSessionsCommandUsesConfiguredStorage verifies /seed-sessions sees the repository once session storage is wired.
func TestNewAppWithDependenciesSeedSessionsCommandUsesConfiguredStorage(t *testing.T) {
	loader := stubLoader{
		cfg: coreconfig.Config{
			Provider:      "anthropic",
			Model:         "claude-sonnet-4-5",
			SessionDBPath: filepath.Join(t.TempDir(), "sessions.db"),
		},
	}

	app, err := NewAppWithDependencies(loader, func(cfg coreconfig.Config) (engine.Engine, *corepermission.FilesystemPolicy, error) {
		_ = cfg
		policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
		if err != nil {
			return nil, nil, err
		}
		return stubEngine{}, policy, nil
	})
	if err != nil {
		t.Fatalf("NewAppWithDependencies() error = %v", err)
	}

	cmd, ok := app.Runner.Commands.Get("seed-sessions")
	if !ok {
		t.Fatal("runner commands missing /seed-sessions command")
	}

	result, err := cmd.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("seed-sessions Execute() error = %v", err)
	}
	if result.Output == servicecommandsSeedNotConfiguredMessageForTest() {
		t.Fatalf("seed-sessions output = %q, want configured storage path", result.Output)
	}
}

func servicecommandsSeedNotConfiguredMessageForTest() string {
	return "Seed command is not available because session storage is not configured."
}

func (l stubLoader) Load(ctx context.Context) (coreconfig.Config, error) {
	return l.cfg, nil
}

type stubEngine struct{}

func (e stubEngine) Run(ctx context.Context, req conversation.RunRequest) (event.Stream, error) {
	ch := make(chan event.Event)
	close(ch)
	return ch, nil
}

var _ engine.Engine = stubEngine{}

// TestNewAppWithDependenciesLoadsConfig verifies bootstrap wires the runner from resolved config and selected engine.
func TestNewAppWithDependenciesLoadsConfig(t *testing.T) {
	loader := stubLoader{
		cfg: coreconfig.Config{
			Provider: "anthropic",
			Model:    "claude-sonnet-4-5",
			APIKey:   "test-key",
		},
	}

	called := false
	app, err := NewAppWithDependencies(loader, func(cfg coreconfig.Config) (engine.Engine, *corepermission.FilesystemPolicy, error) {
		called = true
		if cfg.APIKey != "test-key" {
			t.Fatalf("engine factory cfg = %#v, want api key", cfg)
		}
		policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
		if err != nil {
			return nil, nil, err
		}
		return stubEngine{}, policy, nil
	})
	if err != nil {
		t.Fatalf("NewAppWithDependencies() error = %v", err)
	}

	if !called {
		t.Fatal("engine factory was not called")
	}
	if app.Config.Provider != "anthropic" || app.Runner == nil {
		t.Fatalf("NewAppWithDependencies() = %#v, want anthropic config and runner", app)
	}
	if app.Runner.ProjectPath != "" {
		t.Fatalf("NewAppWithDependencies() runner project path = %q, want empty when loader does not supply one", app.Runner.ProjectPath)
	}
	if app.Runner.Commands == nil {
		t.Fatal("NewAppWithDependencies() runner commands = nil, want slash registry")
	}
	if _, ok := app.Runner.Commands.Get("help"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /help command")
	}
	if _, ok := app.Runner.Commands.Get("clear"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /clear command")
	}
	if _, ok := app.Runner.Commands.Get("compact"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /compact command")
	}
	if _, ok := app.Runner.Commands.Get("memory"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /memory command")
	}
	if _, ok := app.Runner.Commands.Get("resume"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /resume adapter")
	}
	if _, ok := app.Runner.Commands.Get("rename"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /rename adapter")
	}
	if _, ok := app.Runner.Commands.Get("config"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /config command")
	}
	if _, ok := app.Runner.Commands.Get("model"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /model command")
	}
	if _, ok := app.Runner.Commands.Get("fast"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /fast command")
	}
	if _, ok := app.Runner.Commands.Get("effort"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /effort command")
	}
	if _, ok := app.Runner.Commands.Get("output-style"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /output-style command")
	}
	if _, ok := app.Runner.Commands.Get("doctor"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /doctor command")
	}
	if _, ok := app.Runner.Commands.Get("permissions"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /permissions command")
	}
	if _, ok := app.Runner.Commands.Get("allowed-tools"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /allowed-tools alias")
	}
	if _, ok := app.Runner.Commands.Get("add-dir"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /add-dir command")
	}
	if _, ok := app.Runner.Commands.Get("login"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /login command")
	}
	if _, ok := app.Runner.Commands.Get("logout"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /logout command")
	}
	if _, ok := app.Runner.Commands.Get("cost"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /cost command")
	}
	if _, ok := app.Runner.Commands.Get("status"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /status command")
	}
	if _, ok := app.Runner.Commands.Get("mcp"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /mcp command")
	}
	if _, ok := app.Runner.Commands.Get("session"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /session command")
	}
	if _, ok := app.Runner.Commands.Get("files"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /files command")
	}
	if _, ok := app.Runner.Commands.Get("copy"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /copy command")
	}
	if _, ok := app.Runner.Commands.Get("export"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /export command")
	}
	if _, ok := app.Runner.Commands.Get("theme"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /theme command")
	}
	if _, ok := app.Runner.Commands.Get("vim"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /vim command")
	}
	if _, ok := app.Runner.Commands.Get("pr-comments"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /pr-comments command")
	}
	if _, ok := app.Runner.Commands.Get("security-review"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /security-review command")
	}
	if _, ok := app.Runner.Commands.Get("seed-sessions"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /seed-sessions command")
	}
}

// TestDefaultEngineFactoryInjectsApprovalService verifies the production engine wiring now carries a minimal approval service.
func TestDefaultEngineFactoryInjectsApprovalService(t *testing.T) {
	eng, _, err := DefaultEngineFactory(coreconfig.Config{
		Provider:     "anthropic",
		Model:        "claude-sonnet-4-5",
		ApprovalMode: approval.ModeBypassPermissions,
	})
	if err != nil {
		t.Fatalf("DefaultEngineFactory() error = %v", err)
	}

	runtime, ok := eng.(*engine.Runtime)
	if !ok {
		t.Fatalf("DefaultEngineFactory() engine = %T, want *engine.Runtime", eng)
	}
	if runtime.ApprovalService == nil {
		t.Fatal("DefaultEngineFactory() approval service = nil, want injected service")
	}
}

// TestNewCommandRegistryRegistersResume verifies batch-12 bootstrap wiring exposes the minimum resume command through the registry.
func TestNewCommandRegistryRegistersResume(t *testing.T) {
	registry, err := newCommandRegistry(&coreconfig.Config{}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newCommandRegistry() error = %v", err)
	}

	cmds := registry.List()
	if len(cmds) != 28 {
		t.Fatalf("newCommandRegistry() list len = %d, want 28", len(cmds))
	}
	if got := cmds[0].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "help",
		Description: "Show help and available commands",
		Usage:       "/help",
	}) {
		t.Fatalf("newCommandRegistry() first metadata = %#v, want help metadata", got)
	}
	if got := cmds[1].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "clear",
		Description: "Clear conversation history and start a new session",
		Usage:       "/clear",
	}) {
		t.Fatalf("newCommandRegistry() second metadata = %#v, want clear metadata", got)
	}
	if got := cmds[2].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "compact",
		Description: "Clear conversation history but keep a summary in context",
		Usage:       "/compact [instructions]",
	}) {
		t.Fatalf("newCommandRegistry() third metadata = %#v, want compact metadata", got)
	}
	if got := cmds[3].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "memory",
		Description: "Edit Claude memory files",
		Usage:       "/memory",
	}) {
		t.Fatalf("newCommandRegistry() fourth metadata = %#v, want memory metadata", got)
	}
	if got := cmds[4].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "resume",
		Aliases:     []string{"continue"},
		Description: "Resume a saved session by search or continue it with a new prompt",
		Usage:       "/resume <search-term> | /resume <session-id> <prompt>",
	}) {
		t.Fatalf("newCommandRegistry() fifth metadata = %#v, want resume metadata", got)
	}
	if got := cmds[5].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "rename",
		Description: "Rename the current conversation for easier resume discovery",
		Usage:       "/rename <title>",
	}) {
		t.Fatalf("newCommandRegistry() sixth metadata = %#v, want rename metadata", got)
	}
	if got := cmds[6].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "config",
		Aliases:     []string{"settings"},
		Description: "Show the current runtime configuration",
		Usage:       "/config",
	}) {
		t.Fatalf("newCommandRegistry() seventh metadata = %#v, want config metadata", got)
	}
	if got := cmds[7].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "model",
		Description: "Change the model",
		Usage:       "/model [model]",
	}) {
		t.Fatalf("newCommandRegistry() eighth metadata = %#v, want model metadata", got)
	}
	if got := cmds[8].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "fast",
		Description: "Toggle fast mode (Opus 4.6 only)",
		Usage:       "/fast [on|off]",
	}) {
		t.Fatalf("newCommandRegistry() ninth metadata = %#v, want fast metadata", got)
	}
	if got := cmds[9].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "effort",
		Description: "Set effort level for model usage",
		Usage:       "/effort [low|medium|high|max|auto]",
	}) {
		t.Fatalf("newCommandRegistry() tenth metadata = %#v, want effort metadata", got)
	}
	if got := cmds[10].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "output-style",
		Description: "Deprecated: use /config to change output style",
		Usage:       "/output-style",
	}) {
		t.Fatalf("newCommandRegistry() eleventh metadata = %#v, want output-style metadata", got)
	}
	if got := cmds[11].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "doctor",
		Description: "Diagnose the current Claude Code Go host setup",
		Usage:       "/doctor",
	}) {
		t.Fatalf("newCommandRegistry() twelfth metadata = %#v, want doctor metadata", got)
	}
	if got := cmds[12].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "permissions",
		Aliases:     []string{"allowed-tools"},
		Description: "Manage allow & deny tool permission rules",
		Usage:       "/permissions",
	}) {
		t.Fatalf("newCommandRegistry() thirteenth metadata = %#v, want permissions metadata", got)
	}
	if got := cmds[13].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "add-dir",
		Description: "Add a new working directory",
		Usage:       "/add-dir <path>",
	}) {
		t.Fatalf("newCommandRegistry() fourteenth metadata = %#v, want add-dir metadata", got)
	}
	if got := cmds[14].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "login",
		Description: "Sign in with your Anthropic account",
		Usage:       "/login",
	}) {
		t.Fatalf("newCommandRegistry() fifteenth metadata = %#v, want login metadata", got)
	}
	if got := cmds[15].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "logout",
		Description: "Sign out from your Anthropic account",
		Usage:       "/logout",
	}) {
		t.Fatalf("newCommandRegistry() sixteenth metadata = %#v, want logout metadata", got)
	}
	if got := cmds[16].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "cost",
		Description: "Show the total cost and duration of the current session",
		Usage:       "/cost",
	}) {
		t.Fatalf("newCommandRegistry() seventeenth metadata = %#v, want cost metadata", got)
	}
	if got := cmds[17].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "status",
		Description: "Show Claude Code status including version, model, account, API connectivity, and tool statuses",
		Usage:       "/status",
	}) {
		t.Fatalf("newCommandRegistry() eighteenth metadata = %#v, want status metadata", got)
	}
	if got := cmds[18].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "mcp",
		Description: "Manage MCP servers",
		Usage:       "/mcp [enable|disable <server-name>]",
	}) {
		t.Fatalf("newCommandRegistry() nineteenth metadata = %#v, want mcp metadata", got)
	}
	if got := cmds[19].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "session",
		Description: "Show remote session URL and QR code",
		Usage:       "/session",
	}) {
		t.Fatalf("newCommandRegistry() twentieth metadata = %#v, want session metadata", got)
	}
	if got := cmds[20].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "files",
		Description: "List all files currently in context",
		Usage:       "/files",
	}) {
		t.Fatalf("newCommandRegistry() twenty-first metadata = %#v, want files metadata", got)
	}
	if got := cmds[21].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "copy",
		Description: "Copy Claude's last response to clipboard (or /copy N for the Nth-latest)",
		Usage:       "/copy [N]",
	}) {
		t.Fatalf("newCommandRegistry() twenty-second metadata = %#v, want copy metadata", got)
	}
	if got := cmds[22].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "export",
		Description: "Export the current conversation to a file or clipboard",
		Usage:       "/export [filename]",
	}) {
		t.Fatalf("newCommandRegistry() twenty-third metadata = %#v, want export metadata", got)
	}
	if got := cmds[23].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "theme",
		Description: "Change the theme",
		Usage:       "/theme <auto|dark|light|light-daltonized|dark-daltonized|light-ansi|dark-ansi>",
	}) {
		t.Fatalf("newCommandRegistry() twenty-fourth metadata = %#v, want theme metadata", got)
	}
	if got := cmds[24].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "vim",
		Description: "Toggle between Vim and Normal editing modes",
		Usage:       "/vim",
	}) {
		t.Fatalf("newCommandRegistry() twenty-fifth metadata = %#v, want vim metadata", got)
	}
	if got := cmds[25].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "pr-comments",
		Description: "Get comments from a GitHub pull request",
		Usage:       "/pr-comments",
	}) {
		t.Fatalf("newCommandRegistry() twenty-sixth metadata = %#v, want pr-comments metadata", got)
	}
	if got := cmds[26].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "security-review",
		Description: "Complete a security review of the pending changes on the current branch",
		Usage:       "/security-review",
	}) {
		t.Fatalf("newCommandRegistry() twenty-seventh metadata = %#v, want security-review metadata", got)
	}
	if got := cmds[27].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "seed-sessions",
		Description: "Insert demo persisted sessions for /resume testing",
		Usage:       "/seed-sessions",
	}) {
		t.Fatalf("newCommandRegistry() twenty-eighth metadata = %#v, want seed-sessions metadata", got)
	}
}
