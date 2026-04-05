package bootstrap

import (
	"context"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/app/wiring"
	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	"github.com/sheepzhao/claude-code-go/internal/platform/api/anthropic"
	platformconfig "github.com/sheepzhao/claude-code-go/internal/platform/config"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	platformsqlite "github.com/sheepzhao/claude-code-go/internal/platform/store/sqlite"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	"github.com/sheepzhao/claude-code-go/internal/runtime/executor"
	"github.com/sheepzhao/claude-code-go/internal/runtime/repl"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
	servicecommands "github.com/sheepzhao/claude-code-go/internal/services/commands"
	"github.com/sheepzhao/claude-code-go/internal/ui/console"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// EngineFactory constructs the engine selected by the resolved runtime config.
type EngineFactory func(cfg coreconfig.Config) (engine.Engine, error)

// App wires together the minimum batch-07 runtime needed by cmd/cc.
type App struct {
	// Config stores the resolved runtime configuration for observability and tests.
	Config coreconfig.Config
	// Runner owns the one-turn REPL execution flow.
	Runner *repl.Runner
}

// NewApp builds the production app wiring from the default config loader.
func NewApp() (*App, error) {
	loader, err := platformconfig.NewDefaultFileLoader()
	if err != nil {
		return nil, err
	}

	return NewAppWithDependencies(loader, DefaultEngineFactory)
}

// NewAppWithDependencies builds the app from explicit loader and engine dependencies.
func NewAppWithDependencies(loader coreconfig.Loader, engineFactory EngineFactory) (*App, error) {
	cfg, err := loader.Load(context.Background())
	if err != nil {
		return nil, err
	}

	eng, err := engineFactory(cfg)
	if err != nil {
		return nil, err
	}

	renderer := console.NewStreamRenderer(console.NewPrinter(nil))
	runner := repl.NewRunner(eng, renderer)
	runner.ProjectPath = cfg.ProjectPath

	commandRegistry, err := newCommandRegistry(runner)
	if err != nil {
		return nil, err
	}
	runner.Commands = commandRegistry

	if cfg.SessionDBPath != "" {
		db, err := platformsqlite.Open(context.Background(), cfg.SessionDBPath)
		if err != nil {
			return nil, err
		}
		repository := platformsqlite.NewSessionRepository(db)
		manager := runtimesession.NewManager(repository)
		runner.SessionManager = manager
		runner.AutoSave = runtimesession.NewAutoSave(manager)
	}

	logger.DebugCF("bootstrap", "constructed application", map[string]any{
		"provider":            cfg.Provider,
		"model":               cfg.Model,
		"has_session_db_path": cfg.SessionDBPath != "",
	})

	return &App{
		Config: cfg,
		Runner: runner,
	}, nil
}

// newCommandRegistry wires the minimum slash commands available in the current migration stage.
func newCommandRegistry(runner *repl.Runner) (command.Registry, error) {
	registry := command.NewInMemoryRegistry()
	if err := registry.Register(servicecommands.HelpCommand{Registry: registry}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ClearCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(repl.NewResumeCommandAdapter(runner)); err != nil {
		return nil, err
	}
	return registry, nil
}

// DefaultEngineFactory selects the minimum provider implementation supported by batch-07.
func DefaultEngineFactory(cfg coreconfig.Config) (engine.Engine, error) {
	filesystem := platformfs.NewLocalFS()
	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		return nil, err
	}
	modules, err := wiring.NewBaseWorkspaceModules(filesystem, policy)
	if err != nil {
		return nil, err
	}
	toolCatalog := engine.DescribeTools(modules.Tools)
	toolExecutor := executor.NewToolExecutor(modules.Tools)

	switch cfg.Provider {
	case "", "anthropic":
		client := anthropic.NewClient(anthropic.Config{
			APIKey:     cfg.APIKey,
			BaseURL:    cfg.APIBaseURL,
			HTTPClient: nil,
		})
		runtime := engine.New(client, cfg.Model, toolExecutor, toolCatalog...)
		runtime.ApprovalService = approval.NewPromptingService(
			cfg.ApprovalMode,
			console.NewApprovalRenderer(console.NewPrinter(nil), nil),
		)
		return runtime, nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}

// Run forwards execution to the configured runner.
func (a *App) Run(ctx context.Context, args []string) error {
	return a.Runner.Run(ctx, args)
}
