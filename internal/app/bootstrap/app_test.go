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

	app, err := NewAppWithDependencies(loader, func(cfg coreconfig.Config) (engine.Engine, error) {
		_ = cfg
		return stubEngine{}, nil
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
	app, err := NewAppWithDependencies(loader, func(cfg coreconfig.Config) (engine.Engine, error) {
		called = true
		if cfg.APIKey != "test-key" {
			t.Fatalf("engine factory cfg = %#v, want api key", cfg)
		}
		return stubEngine{}, nil
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
	if _, ok := app.Runner.Commands.Get("resume"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /resume adapter")
	}
	if _, ok := app.Runner.Commands.Get("rename"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /rename adapter")
	}
	if _, ok := app.Runner.Commands.Get("config"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /config command")
	}
	if _, ok := app.Runner.Commands.Get("doctor"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /doctor command")
	}
	if _, ok := app.Runner.Commands.Get("session"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /session command")
	}
	if _, ok := app.Runner.Commands.Get("seed-sessions"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /seed-sessions command")
	}
}

// TestDefaultEngineFactoryInjectsApprovalService verifies the production engine wiring now carries a minimal approval service.
func TestDefaultEngineFactoryInjectsApprovalService(t *testing.T) {
	eng, err := DefaultEngineFactory(coreconfig.Config{
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
	registry, err := newCommandRegistry(coreconfig.Config{}, nil)
	if err != nil {
		t.Fatalf("newCommandRegistry() error = %v", err)
	}

	cmds := registry.List()
	if len(cmds) != 8 {
		t.Fatalf("newCommandRegistry() list len = %d, want 8", len(cmds))
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
		Name:        "resume",
		Aliases:     []string{"continue"},
		Description: "Resume a saved session by search or continue it with a new prompt",
		Usage:       "/resume <search-term> | /resume <session-id> <prompt>",
	}) {
		t.Fatalf("newCommandRegistry() third metadata = %#v, want resume metadata", got)
	}
	if got := cmds[3].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "rename",
		Description: "Rename the current conversation for easier resume discovery",
		Usage:       "/rename <title>",
	}) {
		t.Fatalf("newCommandRegistry() fourth metadata = %#v, want rename metadata", got)
	}
	if got := cmds[4].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "config",
		Aliases:     []string{"settings"},
		Description: "Show the current runtime configuration",
		Usage:       "/config",
	}) {
		t.Fatalf("newCommandRegistry() fifth metadata = %#v, want config metadata", got)
	}
	if got := cmds[5].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "doctor",
		Description: "Diagnose the current Claude Code Go host setup",
		Usage:       "/doctor",
	}) {
		t.Fatalf("newCommandRegistry() sixth metadata = %#v, want doctor metadata", got)
	}
	if got := cmds[6].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "session",
		Description: "Show remote session URL and QR code",
		Usage:       "/session",
	}) {
		t.Fatalf("newCommandRegistry() seventh metadata = %#v, want session metadata", got)
	}
	if got := cmds[7].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "seed-sessions",
		Description: "Insert demo persisted sessions for /resume testing",
		Usage:       "/seed-sessions",
	}) {
		t.Fatalf("newCommandRegistry() eighth metadata = %#v, want seed-sessions metadata", got)
	}
}
