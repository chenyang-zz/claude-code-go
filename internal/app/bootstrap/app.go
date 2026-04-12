package bootstrap

import (
	"context"
	"fmt"
	"os"

	"github.com/sheepzhao/claude-code-go/internal/app/wiring"
	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	"github.com/sheepzhao/claude-code-go/internal/platform/api/anthropic"
	platformconfig "github.com/sheepzhao/claude-code-go/internal/platform/config"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	platformgit "github.com/sheepzhao/claude-code-go/internal/platform/git"
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

// EngineFactory constructs the engine selected by the resolved runtime config together with the shared filesystem policy.
type EngineFactory func(cfg coreconfig.Config) (engine.Engine, *corepermission.FilesystemPolicy, error)

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

	eng, policy, err := engineFactory(cfg)
	if err != nil {
		return nil, err
	}

	renderer := console.NewStreamRenderer(console.NewPrinter(nil))
	runner := repl.NewRunner(eng, renderer)
	runner.ProjectPath = cfg.ProjectPath
	runner.Input = os.Stdin
	runner.WorktreeLister = platformgit.NewClient()

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

	var globalSettingsStore *platformconfig.GlobalSettingsStore
	var projectSettingsStore *platformconfig.ProjectSettingsStore
	if fileLoader, ok := loader.(*platformconfig.FileLoader); ok {
		globalSettingsStore = platformconfig.NewGlobalSettingsStore(fileLoader.HomeDir)
		projectSettingsStore = platformconfig.NewProjectSettingsStore(fileLoader.CWD)
	}

	commandRegistry, err := newCommandRegistry(&cfg, runner, globalSettingsStore, projectSettingsStore, policy)
	if err != nil {
		return nil, err
	}
	runner.Commands = commandRegistry

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
func newCommandRegistry(cfg *coreconfig.Config, runner *repl.Runner, globalSettingsStore *platformconfig.GlobalSettingsStore, projectSettingsStore *platformconfig.ProjectSettingsStore, policy *corepermission.FilesystemPolicy) (command.Registry, error) {
	registry := command.NewInMemoryRegistry()
	var sessionRepository coresession.Repository
	if runner != nil && runner.SessionManager != nil {
		sessionRepository = runner.SessionManager.Repository
	}
	if err := registry.Register(servicecommands.HelpCommand{Registry: registry}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ClearCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.CompactCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.MemoryCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(repl.NewResumeCommandAdapter(runner)); err != nil {
		return nil, err
	}
	if err := registry.Register(repl.NewRenameCommandAdapter(runner)); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ConfigCommand{Config: dereferenceConfig(cfg)}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ModelCommand{
		Config: cfg,
		Store:  globalSettingsStore,
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.FastCommand{
		Config: cfg,
		Store:  globalSettingsStore,
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.EffortCommand{
		Config: cfg,
		Store:  globalSettingsStore,
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.OutputStyleCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.DoctorCommand{Config: dereferenceConfig(cfg)}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.PermissionsCommand{Config: dereferenceConfig(cfg)}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.AddDirCommand{
		Config: cfg,
		Store:  projectSettingsStore,
		Policy: policy,
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.LoginCommand{Config: dereferenceConfig(cfg)}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.LogoutCommand{Config: dereferenceConfig(cfg)}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.CostCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.StatusCommand{Config: dereferenceConfig(cfg)}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.MCPCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.SessionCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.FilesCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.CopyCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ExportCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.VersionCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ReleaseNotesCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.UpgradeCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.UsageCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.StatsCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ExtraUsageCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ThemeCommand{
		Config: cfg,
		Store:  globalSettingsStore,
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.VimCommand{
		Config: cfg,
		Store:  globalSettingsStore,
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.PRCommentsCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.SecurityReviewCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.SeedSessionsCommand{
		Repository:  sessionRepository,
		ProjectPath: dereferenceConfig(cfg).ProjectPath,
	}); err != nil {
		return nil, err
	}
	return registry, nil
}

// dereferenceConfig copies the pointed runtime config or returns the zero value when unavailable.
func dereferenceConfig(cfg *coreconfig.Config) coreconfig.Config {
	if cfg == nil {
		return coreconfig.Config{}
	}
	return *cfg
}

// DefaultEngineFactory selects the minimum provider implementation supported by batch-07.
func DefaultEngineFactory(cfg coreconfig.Config) (engine.Engine, *corepermission.FilesystemPolicy, error) {
	filesystem := platformfs.NewLocalFS()
	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		return nil, nil, err
	}
	for _, configured := range cfg.Permissions.AdditionalDirectories {
		expanded, err := platformfs.ExpandPath(configured, cfg.ProjectPath)
		if err != nil {
			return nil, nil, fmt.Errorf("expand configured additional directory %q: %w", configured, err)
		}
		policy.AddReadRoot(expanded)
	}
	modules, err := wiring.NewBaseWorkspaceModules(filesystem, policy)
	if err != nil {
		return nil, nil, err
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
		return runtime, policy, nil
	default:
		return nil, nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}

// Run forwards execution to the configured runner.
func (a *App) Run(ctx context.Context, args []string) error {
	return a.Runner.Run(ctx, args)
}
