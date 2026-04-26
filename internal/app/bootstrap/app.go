package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/app/wiring"
	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	coretask "github.com/sheepzhao/claude-code-go/internal/core/task"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/platform/api/anthropic"
	"github.com/sheepzhao/claude-code-go/internal/platform/api/openai"
	platformconfig "github.com/sheepzhao/claude-code-go/internal/platform/config"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	platformgit "github.com/sheepzhao/claude-code-go/internal/platform/git"
	mcpbridge "github.com/sheepzhao/claude-code-go/internal/platform/mcp/bridge"
	mcpclient "github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
	platformremote "github.com/sheepzhao/claude-code-go/internal/platform/remote"
	platformsqlite "github.com/sheepzhao/claude-code-go/internal/platform/store/sqlite"
	platformteam "github.com/sheepzhao/claude-code-go/internal/platform/team"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	"github.com/sheepzhao/claude-code-go/internal/runtime/executor"
	runtimehooks "github.com/sheepzhao/claude-code-go/internal/runtime/hooks"
	"github.com/sheepzhao/claude-code-go/internal/runtime/repl"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
	servicecommands "github.com/sheepzhao/claude-code-go/internal/services/commands"
	"github.com/sheepzhao/claude-code-go/internal/services/prompts"
	agenttool "github.com/sheepzhao/claude-code-go/internal/services/tools/agent"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/agent/builtin"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/agent/loader"
	mcpproxy "github.com/sheepzhao/claude-code-go/internal/services/tools/mcp"
	"github.com/sheepzhao/claude-code-go/internal/ui/console"
	"github.com/sheepzhao/claude-code-go/internal/ui/jsonout"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// EngineFactory constructs the engine selected by the resolved runtime config together with the shared filesystem policy.
type EngineFactory func(cfg coreconfig.Config, backgroundTaskStore *runtimesession.BackgroundTaskStore, taskStore coretask.Store) (engine.Engine, *corepermission.FilesystemPolicy, error)

// App wires together the minimum batch-07 runtime needed by cmd/cc.
type App struct {
	// Config stores the resolved runtime configuration for observability and tests.
	Config coreconfig.Config
	// Runner owns the one-turn REPL execution flow.
	Runner *repl.Runner
}

// NewApp builds the production app wiring from the default config loader.
func NewApp() (*App, error) {
	return NewAppFromLoader(func() (*platformconfig.FileLoader, error) {
		return platformconfig.NewDefaultFileLoader()
	})
}

// NewAppFromLoader builds the production app wiring from one explicit default loader factory.
func NewAppFromLoader(factory func() (*platformconfig.FileLoader, error)) (*App, error) {
	loader, err := factory()
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
	configureConsoleLogging(cfg.OutputFormat)
	applyRuntimeEnvironment(cfg.Env)

	backgroundTaskStore := runtimesession.NewBackgroundTaskStore()
	taskStore := resolveTaskStore(loader, cfg.HomeDir)
	eng, policy, err := engineFactory(cfg, backgroundTaskStore, taskStore)
	if err != nil {
		return nil, err
	}

	var renderer console.EventRenderer
	if cfg.OutputFormat == "stream-json" {
		renderer = jsonout.NewWriter(os.Stdout)
	} else {
		renderer = console.NewStreamRenderer(console.NewPrinter(nil))
	}
	runner := repl.NewRunner(eng, renderer)
	runner.ProjectPath = cfg.ProjectPath
	runner.RemoteSession = cfg.RemoteSession
	if cfg.RemoteSession.Enabled && strings.TrimSpace(cfg.RemoteSession.StreamURL) != "" {
		runner.RemoteLifecycle = platformremote.NewLifecycleManager(nil, nil)
		if endpoint := platformremote.DeriveEndpoint(cfg.RemoteSession); endpoint != "" {
			headers := platformremote.AuthHeaders()
			var opts []platformremote.CCROption
			for k, v := range headers {
				opts = append(opts, platformremote.WithHeader(k, v))
			}
			tokenProvider := platformremote.NewEnvTokenProvider()
			opts = append(opts, platformremote.WithTokenProvider(tokenProvider))
			runner.RemoteSender = platformremote.NewCCRClient(endpoint, cfg.RemoteSession.SessionID, opts...)
		}
	}
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
	var localSettingsStore *platformconfig.LocalSettingsStore
	if fileLoader, ok := loader.(*platformconfig.FileLoader); ok {
		globalSettingsStore = platformconfig.NewGlobalSettingsStore(fileLoader.HomeDir)
		projectSettingsStore = platformconfig.NewProjectSettingsStore(fileLoader.CWD)
		localSettingsStore = platformconfig.NewLocalSettingsStore(fileLoader.CWD)
	}

	agentRegistry := resolveAgentRegistry(cfg)
	commandRegistry, err := newCommandRegistry(&cfg, runner, globalSettingsStore, projectSettingsStore, localSettingsStore, policy, backgroundTaskStore, taskStore, agentRegistry)
	if err != nil {
		return nil, err
	}
	runner.Commands = commandRegistry

	logger.DebugCF("bootstrap", "constructed application", map[string]any{
		"provider":            cfg.Provider,
		"model":               cfg.Model,
		"has_session_db_path": cfg.SessionDBPath != "",
		"remote_mode":         cfg.RemoteSession.Enabled,
		"output_format":       cfg.OutputFormat,
	})

	return &App{
		Config: cfg,
		Runner: runner,
	}, nil
}

func configureConsoleLogging(outputFormat string) {
	if outputFormat == "stream-json" {
		logger.SetConsoleOutput(os.Stderr)
		return
	}
	logger.SetConsoleOutput(os.Stdout)
}

// newCommandRegistry wires the minimum slash commands available in the current migration stage.
func newCommandRegistry(cfg *coreconfig.Config, runner *repl.Runner, globalSettingsStore *platformconfig.GlobalSettingsStore, projectSettingsStore *platformconfig.ProjectSettingsStore, localSettingsStore *platformconfig.LocalSettingsStore, policy *corepermission.FilesystemPolicy, backgroundTaskStore *runtimesession.BackgroundTaskStore, taskStore coretask.Store, agentRegistry agent.Registry) (command.Registry, error) {
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
		Probe:  buildUsageLimitsProbe(cfg),
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
	statusToolRegistry, err := wiring.NewModules(wiring.BaseWorkspaceTools(platformfs.NewLocalFS(), policy, cfg.Permissions, backgroundTaskStore, taskStore)...)
	if err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.DoctorCommand{
		Config:       dereferenceConfig(cfg),
		ToolRegistry: statusToolRegistry.Tools,
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.PermissionsCommand{Config: dereferenceConfig(cfg)}); err != nil {
		return nil, err
	}
	_ = projectSettingsStore
	if err := registry.Register(repl.NewAddDirCommandAdapter(runner, servicecommands.AddDirCommand{
		Config:     cfg,
		LocalStore: localSettingsStore,
		Policy:     policy,
	})); err != nil {
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
	if err := registry.Register(servicecommands.StatusCommand{
		Config:       dereferenceConfig(cfg),
		ToolRegistry: statusToolRegistry.Tools,
		APIProbe:     buildStatusProbe(cfg),
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.MCPCommand{}); err != nil {
		return nil, err
	}
	var stateProvider servicecommands.RemoteStateProvider
	var sendStateProvider servicecommands.RemoteSendStateProvider
	var subagentStateProvider platformremote.RemoteSubagentStateProvider
	var authStateProvider platformremote.AuthStateProvider
	if runner != nil {
		stateProvider = runner.RemoteLifecycle
		if sender, ok := runner.RemoteSender.(servicecommands.RemoteSendStateProvider); ok {
			sendStateProvider = sender
		}
		if asp, ok := runner.RemoteSender.(platformremote.AuthStateProvider); ok {
			authStateProvider = asp
		}
	}
	if err := registry.Register(servicecommands.SessionCommand{
		RemoteSession:         cfg.RemoteSession,
		StateProvider:         stateProvider,
		SendStateProvider:     sendStateProvider,
		SubagentStateProvider: subagentStateProvider,
		AuthStateProvider:     authStateProvider,
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.BranchCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.VoiceCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.IdeCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.InitCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.InstallGitHubAppCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.InstallSlackAppCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.RemoteEnvCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.PrivacySettingsCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.PlanCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.TasksCommand{TaskStore: backgroundTaskStore}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.DiffCommand{}); err != nil {
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
	if err := registry.Register(servicecommands.UsageCommand{
		Config: dereferenceConfig(cfg),
		Probe:  buildUsageLimitsProbe(cfg),
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.StatsCommand{
		Config: dereferenceConfig(cfg),
		Probe:  buildUsageLimitsProbe(cfg),
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ExtraUsageCommand{
		Config: dereferenceConfig(cfg),
		Probe:  buildUsageLimitsProbe(cfg),
	}); err != nil {
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
	if err := registry.Register(servicecommands.TerminalSetupCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.KeybindingsCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.PRCommentsCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.SecurityReviewCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.AgentsCommand{
		StatusProvider: platformteam.NewReader(cfg.HomeDir, taskStore),
		Registry:       agentRegistry,
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.PluginCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.HooksCommand{}); err != nil {
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

// resolveTaskStore builds a task store rooted under the effective home directory.
// By default it uses the stable "default" task list directory so tasks remain
// visible across separate CLI invocations. An explicit task list override still
// opts into an isolated directory.
func resolveTaskStore(loader coreconfig.Loader, homeDir string) coretask.Store {
	if homeDir == "" {
		if fl, ok := loader.(*platformconfig.FileLoader); ok {
			homeDir = fl.HomeDir
		}
	}
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	if homeDir == "" {
		return nil
	}
	taskListID := "default"
	if os.Getenv("CLAUDE_CODE_TASK_LIST_ID") != "" {
		taskListID = coretask.ResolveTaskListID()
	}
	return coretask.NewFileStore(filepath.Join(homeDir, ".claude", "tasks", taskListID))
}

// DefaultEngineFactory selects the minimum provider implementation supported by batch-07.
func DefaultEngineFactory(cfg coreconfig.Config, backgroundTaskStore *runtimesession.BackgroundTaskStore, taskStore coretask.Store) (engine.Engine, *corepermission.FilesystemPolicy, error) {
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

	hookRunner := runtimehooks.NewRunner()
	modules, err := wiring.NewBaseWorkspaceModulesWithHooks(filesystem, policy, cfg.Permissions, backgroundTaskStore, taskStore, hookRunner, cfg.Hooks, cfg.DisableAllHooks)
	if err != nil {
		return nil, nil, err
	}

	// Connect MCP servers and register their tools into the workspace registry.
	var mcpServerRegistry *mcpregistry.ServerRegistry
	mcpConfigs := loadMCPConfigs()
	if len(mcpConfigs) > 0 {
		mcpServerRegistry = mcpregistry.NewServerRegistry()
		mcpServerRegistry.SetAuthToken(cfg.AuthToken)
		mcpServerRegistry.LoadConfigs(mcpConfigs)
		ctx, cancel := context.WithTimeout(context.Background(), 30*1000000000) // 30s
		mcpServerRegistry.ConnectAll(ctx)
		cancel()

		mcpregistry.SetLastRegistry(mcpServerRegistry)
		for _, entry := range mcpServerRegistry.Connected() {
			toolsResult, err := entry.Client.ListTools(context.Background())
			if err != nil {
				logger.WarnCF("bootstrap", "mcp listTools failed", map[string]any{
					"server": entry.Name,
					"error":  err.Error(),
				})
				continue
			}
			for _, t := range toolsResult.Tools {
				proxyTool := mcpproxy.AdaptTool(entry.Name, t, entry.Client)
				if regErr := modules.Tools.Register(proxyTool); regErr != nil {
					logger.WarnCF("bootstrap", "mcp tool registration failed", map[string]any{
						"server": entry.Name,
						"tool":   t.Name,
						"error":  regErr.Error(),
					})
				}
			}
		}
		registerMCPAuthTools(modules.Tools, mcpServerRegistry)
		for _, entry := range mcpServerRegistry.Connected() {
			if err := mcpbridge.RegisterElicitationHandlers(entry.Client, entry.Name, hookRunner, cfg.Hooks, cfg.DisableAllHooks); err != nil {
				logger.WarnCF("bootstrap", "failed to register mcp elicitation handlers", map[string]any{
					"server": entry.Name,
					"error":  err.Error(),
				})
			}
		}
	}

	toolCatalog := engine.DescribeTools(modules.Tools)
	toolExecutor := executor.NewToolExecutor(modules.Tools)
	agentRegistry := resolveAgentRegistry(cfg)
	promptBuilder := newPromptBuilder(cfg, agentRegistry)

	switch coreconfig.NormalizeProvider(cfg.Provider) {
	case coreconfig.ProviderAnthropic:
		client := anthropic.NewClient(anthropic.Config{
			APIKey:       cfg.APIKey,
			AuthToken:    cfg.AuthToken,
			BaseURL:      cfg.APIBaseURL,
			HTTPClient:   nil,
			IsFirstParty: true,
		})
		runtime := engine.New(client, cfg.Model, toolExecutor, toolCatalog...)
		runtime.Hooks = cfg.Hooks
		runtime.DisableAllHooks = cfg.DisableAllHooks
		runtime.HookRunner = hookRunner
		runtime.EnablePromptCaching = cfg.EnablePromptCaching
		runtime.CacheBreakDetector = anthropic.NewCacheBreakDetector()
		runtime.Source = "repl_main_thread"
		runtime.ApprovalService = approval.NewPromptingService(
			cfg.ApprovalMode,
			console.NewApprovalRenderer(approvalPrinterForConfig(cfg), nil),
		)
		runtime.PromptBuilder = promptBuilder
		runtime.AgentRegistry = agentRegistry
		runtime.SessionConfig = buildSessionConfigSnapshot(cfg, agentRegistry, mcpServerRegistry)
		configureMainThreadAgent(runtime, cfg, agentRegistry)

		// Register the Agent tool after the runtime is created so the runner can use it as parent.
		if agentRegistry != nil {
			agentTool := agenttool.NewTool(agentRegistry, runtime, mcpServerRegistry, modules.Tools)
			if regErr := modules.Tools.Register(agentTool); regErr != nil {
				logger.WarnCF("bootstrap", "failed to register agent tool", map[string]any{"error": regErr.Error()})
			} else {
				// Re-describe tools so the runtime catalog includes the Agent tool.
				runtime.ToolCatalog = engine.DescribeTools(modules.Tools)
			}
		}

		return runtime, policy, nil
	case coreconfig.ProviderOpenAICompatible, coreconfig.ProviderGLM:
		var client model.Client
		if cfg.Provider == "openai" && openai.UseResponsesAPI(cfg.Model) {
			client = openai.NewResponsesClient(openai.Config{
				APIKey:     cfg.APIKey,
				BaseURL:    cfg.APIBaseURL,
				HTTPClient: nil,
			})
		} else {
			client = openai.NewClient(openai.Config{
				Provider:   cfg.Provider,
				APIKey:     cfg.APIKey,
				BaseURL:    cfg.APIBaseURL,
				HTTPClient: nil,
			})
		}
		runtime := engine.New(client, cfg.Model, toolExecutor, toolCatalog...)
		runtime.Hooks = cfg.Hooks
		runtime.DisableAllHooks = cfg.DisableAllHooks
		runtime.HookRunner = hookRunner
		runtime.EnablePromptCaching = cfg.EnablePromptCaching
		runtime.ApprovalService = approval.NewPromptingService(
			cfg.ApprovalMode,
			console.NewApprovalRenderer(approvalPrinterForConfig(cfg), nil),
		)
		applyOpenAIAdvancedDefaults(runtime)
		runtime.PromptBuilder = promptBuilder
		runtime.AgentRegistry = agentRegistry
		runtime.SessionConfig = buildSessionConfigSnapshot(cfg, agentRegistry, mcpServerRegistry)
		configureMainThreadAgent(runtime, cfg, agentRegistry)

		// Register the Agent tool after the runtime is created so the runner can use it as parent.
		if agentRegistry != nil {
			agentTool := agenttool.NewTool(agentRegistry, runtime, mcpServerRegistry, modules.Tools)
			if regErr := modules.Tools.Register(agentTool); regErr != nil {
				logger.WarnCF("bootstrap", "failed to register agent tool", map[string]any{"error": regErr.Error()})
			} else {
				// Re-describe tools so the runtime catalog includes the Agent tool.
				runtime.ToolCatalog = engine.DescribeTools(modules.Tools)
			}
		}

		return runtime, policy, nil
	default:
		return nil, nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}

// buildSessionConfigSnapshot assembles the current session configuration visible to guide agents.
func buildSessionConfigSnapshot(cfg coreconfig.Config, agentRegistry agent.Registry, mcpRegistry *mcpregistry.ServerRegistry) coretool.SessionConfigSnapshot {
	var snapshot coretool.SessionConfigSnapshot

	// Custom agents: source != "built-in"
	if agentRegistry != nil {
		for _, def := range agentRegistry.List() {
			if def.Source != "built-in" {
				snapshot.CustomAgents = append(snapshot.CustomAgents, coretool.AgentInfo{
					AgentType: def.AgentType,
					WhenToUse: def.WhenToUse,
					Color:     def.Color,
				})
			}
		}
	}

	// MCP servers: connected entries only
	if mcpRegistry != nil {
		for _, entry := range mcpRegistry.Connected() {
			snapshot.MCPServers = append(snapshot.MCPServers, entry.Name)
		}
	}

	// User settings: expose a filtered, non-sensitive subset.
	snapshot.UserSettings = filterSettingsForSnapshot(cfg)

	return snapshot
}

// filterSettingsForSnapshot returns a safe subset of runtime configuration for guide agent prompts.
func filterSettingsForSnapshot(cfg coreconfig.Config) map[string]any {
	safe := map[string]any{
		"model":                    cfg.Model,
		"provider":                 cfg.Provider,
		"effortLevel":              cfg.EffortLevel,
		"theme":                    cfg.Theme,
		"editorMode":               cfg.EditorMode,
		"approvalMode":             cfg.ApprovalMode,
		"outputFormat":             cfg.OutputFormat,
		"outputStyle":              cfg.OutputStyle,
		"language":                 cfg.Language,
		"fastMode":                 cfg.FastMode,
		"enablePromptCaching":      cfg.EnablePromptCaching,
		"autoUpdatesChannel":       cfg.AutoUpdatesChannel,
		"plansDirectory":           cfg.PlansDirectory,
		"skipWebFetchPreflight":    cfg.SkipWebFetchPreflight,
		"disableAllHooks":          cfg.DisableAllHooks,
		"allowManagedHooksOnly":    cfg.AllowManagedHooksOnly,
		"allowedHttpHookUrls":      cfg.AllowedHttpHookUrls,
		"httpHookAllowedEnvVars":   cfg.HttpHookAllowedEnvVars,
		"channelsEnabled":          cfg.ChannelsEnabled,
		"claudeMdExcludes":         cfg.ClaudeMdExcludes,
		"additionalDirectories":    cfg.Permissions.AdditionalDirectories,
		"loadedSettingSources":     cfg.LoadedSettingSources,
		"policySettings": map[string]any{
			"origin":     cfg.PolicySettings.Origin,
			"hasBaseFile": cfg.PolicySettings.HasBaseFile,
			"hasDropIns":  cfg.PolicySettings.HasDropIns,
		},
	}

	if cfg.Agent != "" {
		safe["agent"] = cfg.Agent
	}
	if cfg.RemoteSession.Enabled {
		safe["remoteSession"] = map[string]any{
			"enabled": cfg.RemoteSession.Enabled,
			"url":     cfg.RemoteSession.URL,
		}
	}
	if len(cfg.EnabledPlugins) > 0 {
		safe["enabledPlugins"] = cfg.EnabledPlugins
	}
	if len(cfg.Sandbox) > 0 {
		safe["sandbox"] = cfg.Sandbox
	}
	if cfg.StatusLine.Type != "" {
		safe["statusLine"] = cfg.StatusLine
	}
	if len(cfg.SSHConfigs) > 0 {
		safe["sshConfigs"] = cfg.SSHConfigs
	}
	if cfg.MinimumVersion != "" {
		safe["minimumVersion"] = cfg.MinimumVersion
	}

	return safe
}

// newPromptBuilder creates a PromptBuilder with the standard system prompt sections.
func newPromptBuilder(cfg coreconfig.Config, registry agent.Registry) *prompts.PromptBuilder {
	return prompts.NewPromptBuilder(
		prompts.CoordinatorSection{},
		prompts.IdentitySection{},
		prompts.EnvironmentSection{Model: cfg.Model},
		prompts.PermissionSection{},
		prompts.ToolGuidelinesSection{},
		prompts.FileReadPromptSection{},
		prompts.FileWritePromptSection{},
		prompts.FileEditPromptSection{},
		prompts.GlobPromptSection{},
		prompts.GrepPromptSection{},
		prompts.BashPromptSection{},
		prompts.WebFetchGuidelinesSection{},
		prompts.AgentListingSection{Registry: registry},
		prompts.SessionGuidanceSection{},
		prompts.MemorySection{},
		prompts.MCPInstructionsSection{},
		prompts.ScratchpadSection{},
		prompts.FunctionResultClearingSection{},
		prompts.ToolResultsReminderSection{},
		prompts.BriefSection{},
		prompts.ProactiveSection{},
	)
}

// applyOpenAIAdvancedDefaults reads optional environment variables for OpenAI
// Responses API advanced parameters and applies them to the runtime.
func applyOpenAIAdvancedDefaults(r *engine.Runtime) {
	if v := os.Getenv("CLAUDE_CODE_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			r.DefaultTemperature = &f
		}
	}
	if v := os.Getenv("CLAUDE_CODE_TOP_P"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			r.DefaultTopP = &f
		}
	}
	if v := os.Getenv("CLAUDE_CODE_STORE"); v != "" {
		b := v == "true" || v == "1"
		r.DefaultStore = &b
	}
	if v := os.Getenv("CLAUDE_CODE_REASONING_EFFORT"); v != "" {
		r.DefaultReasoningEffort = &v
	}
	if v := os.Getenv("CLAUDE_CODE_TOOL_CHOICE"); v != "" {
		r.DefaultToolChoice = &v
	}
	if v := os.Getenv("CLAUDE_CODE_USER"); v != "" {
		r.DefaultUser = &v
	}
	if v := os.Getenv("CLAUDE_CODE_INSTRUCTIONS"); v != "" {
		r.DefaultInstructions = &v
	}
}

// resolveAgentRegistry returns the agent registry used by the engine runtime.
// It creates an in-memory registry, registers built-in agents, and loads custom
// agents from user, project, local, and managed directories. Agents are loaded
// in priority order (built-in → user → project → local → managed) so that later
// sources override earlier ones when names collide, matching TypeScript
// getActiveAgentsFromList.
func resolveAgentRegistry(cfg coreconfig.Config) agent.Registry {
	registry := agent.NewInMemoryRegistry()

	// Register built-in agents first.
	if err := builtin.RegisterBuiltInAgents(registry); err != nil {
		logger.WarnCF("bootstrap", "failed to register built-in agents", map[string]any{"error": err.Error()})
	}

	// Load user-scoped agents if the source is enabled.
	if isAgentSourceEnabled("userSettings", cfg.LoadedSettingSources) {
		userAgents, loadErrors, err := loader.LoadUserAgents(cfg.HomeDir)
		if err != nil {
			logger.WarnCF("bootstrap", "failed to load user agents", map[string]any{"error": err.Error()})
		}
		for _, loadErr := range loadErrors {
			logger.WarnCF("bootstrap", "user agent load error", map[string]any{
				"path":  loadErr.Path,
				"error": loadErr.Error,
			})
		}
		for _, def := range userAgents {
			registry.Remove(def.AgentType)
			if regErr := registry.Register(def); regErr != nil {
				logger.WarnCF("bootstrap", "failed to register user agent", map[string]any{
					"agent": def.AgentType,
					"error": regErr.Error(),
				})
			}
		}
	}

	// Load project-scoped agents if the source is enabled.
	if isAgentSourceEnabled("projectSettings", cfg.LoadedSettingSources) {
		projectAgents, loadErrors, err := loader.LoadProjectAgents(cfg.ProjectPath)
		if err != nil {
			logger.WarnCF("bootstrap", "failed to load project agents", map[string]any{"error": err.Error()})
		}
		for _, loadErr := range loadErrors {
			logger.WarnCF("bootstrap", "project agent load error", map[string]any{
				"path":  loadErr.Path,
				"error": loadErr.Error,
			})
		}
		for _, def := range projectAgents {
			registry.Remove(def.AgentType)
			if regErr := registry.Register(def); regErr != nil {
				logger.WarnCF("bootstrap", "failed to register project agent", map[string]any{
					"agent": def.AgentType,
					"error": regErr.Error(),
				})
			}
		}
	}

	// Load local-scoped agents if the source is enabled.
	// Local agents share the same physical directory as project agents
	// (.claude/agents/) but use source "localSettings" for priority tracking.
	if isAgentSourceEnabled("localSettings", cfg.LoadedSettingSources) {
		localAgents, loadErrors, err := loader.LoadLocalAgents(cfg.ProjectPath)
		if err != nil {
			logger.WarnCF("bootstrap", "failed to load local agents", map[string]any{"error": err.Error()})
		}
		for _, loadErr := range loadErrors {
			logger.WarnCF("bootstrap", "local agent load error", map[string]any{
				"path":  loadErr.Path,
				"error": loadErr.Error,
			})
		}
		for _, def := range localAgents {
			registry.Remove(def.AgentType)
			if regErr := registry.Register(def); regErr != nil {
				logger.WarnCF("bootstrap", "failed to register local agent", map[string]any{
					"agent": def.AgentType,
					"error": regErr.Error(),
				})
			}
		}
	}

	// Load managed (policySettings) agents if the source is enabled.
	// Managed agents have the highest priority and are loaded last so they
	// override all other sources, matching TypeScript behavior.
	if isAgentSourceEnabled("policySettings", cfg.LoadedSettingSources) {
		managedAgents, loadErrors, err := loader.LoadManagedAgents(cfg.ManagedSettingsDir)
		if err != nil {
			logger.WarnCF("bootstrap", "failed to load managed agents", map[string]any{"error": err.Error()})
		}
		for _, loadErr := range loadErrors {
			logger.WarnCF("bootstrap", "managed agent load error", map[string]any{
				"path":  loadErr.Path,
				"error": loadErr.Error,
			})
		}
		for _, def := range managedAgents {
			registry.Remove(def.AgentType)
			if regErr := registry.Register(def); regErr != nil {
				logger.WarnCF("bootstrap", "failed to register managed agent", map[string]any{
					"agent": def.AgentType,
					"error": regErr.Error(),
				})
			}
		}
	}

	return registry
}

// isAgentSourceEnabled reports whether a given agent source is present in the
// loaded setting sources list. An empty list is treated as "all sources enabled"
// for backward compatibility.
func isAgentSourceEnabled(source string, loaded []string) bool {
	if len(loaded) == 0 {
		return true
	}
	return slices.Contains(loaded, source)
}

// configureMainThreadAgent wires the selected main-thread agent into runtime state.
func configureMainThreadAgent(runtime *engine.Runtime, cfg coreconfig.Config, registry agent.Registry) {
	if runtime == nil {
		return
	}

	agentType := strings.TrimSpace(cfg.Agent)
	runtime.MainThreadAgentType = agentType
	if agentType == "" || registry == nil {
		return
	}
	if _, ok := registry.Get(agentType); !ok {
		logger.WarnCF("bootstrap", "main-thread agent not found in registry", map[string]any{
			"agent_type": agentType,
		})
	}
}

// registerMCPAuthTools exposes pseudo auth tools for MCP servers that need manual authentication.
func registerMCPAuthTools(toolRegistry coretool.Registry, registry *mcpregistry.ServerRegistry) {
	if toolRegistry == nil || registry == nil {
		return
	}

	for _, entry := range registry.List() {
		if entry.Status != mcpregistry.StatusNeedsAuth {
			continue
		}
		authTool := mcpproxy.NewAuthTool(entry.Name, entry.Config)
		if err := toolRegistry.Register(authTool); err != nil {
			logger.WarnCF("bootstrap", "mcp auth tool registration failed", map[string]any{
				"server": entry.Name,
				"error":  err.Error(),
			})
		}
	}
}

// loadMCPConfigs reads MCP server configurations from the CLAUDE_CODE_MCP_SERVERS
// environment variable (JSON object mapping server names to ServerConfig).
func loadMCPConfigs() map[string]mcpclient.ServerConfig {
	raw := os.Getenv("CLAUDE_CODE_MCP_SERVERS")
	if raw == "" {
		return nil
	}
	var configs map[string]mcpclient.ServerConfig
	if err := json.Unmarshal([]byte(raw), &configs); err != nil {
		logger.WarnCF("bootstrap", "failed to parse MCP server configs", map[string]any{
			"error": err.Error(),
		})
		return nil
	}
	return configs
}

// approvalPrinterForConfig returns a printer directed at stderr when stream-json
// mode is active so that approval prompts do not pollute the NDJSON stdout stream.
func approvalPrinterForConfig(cfg coreconfig.Config) *console.Printer {
	if cfg.OutputFormat == "stream-json" {
		return console.NewPrinter(os.Stderr)
	}
	return console.NewPrinter(nil)
}

// applyRuntimeEnvironment writes the filtered settings-sourced runtime environment variables into the current process.
func applyRuntimeEnvironment(values map[string]string) {
	if len(values) == 0 {
		return
	}
	for key, value := range values {
		if err := os.Setenv(key, value); err != nil {
			logger.WarnCF("bootstrap", "failed to set runtime environment variable", map[string]any{
				"key":   key,
				"error": err.Error(),
			})
		}
	}
	logger.DebugCF("bootstrap", "applied runtime environment variables", map[string]any{
		"count": len(values),
	})
}

// buildStatusProbe selects the provider-specific connectivity probe used by /status.
func buildStatusProbe(cfg *coreconfig.Config) servicecommands.APIConnectivityProber {
	if cfg == nil {
		return nil
	}

	switch coreconfig.NormalizeProvider(cfg.Provider) {
	case coreconfig.ProviderAnthropic:
		return anthropic.NewStatusProbe(anthropic.StatusProbeConfig{})
	case coreconfig.ProviderOpenAICompatible, coreconfig.ProviderGLM:
		return openai.NewStatusProbe(openai.StatusProbeConfig{
			Provider: cfg.Provider,
		})
	default:
		return nil
	}
}

// buildUsageLimitsProbe selects the provider-specific quota probe used by usage-related slash commands.
func buildUsageLimitsProbe(cfg *coreconfig.Config) servicecommands.UsageLimitsProber {
	if cfg == nil {
		return nil
	}

	switch coreconfig.NormalizeProvider(cfg.Provider) {
	case coreconfig.ProviderAnthropic:
		return anthropic.NewQuotaProbe(anthropic.QuotaProbeConfig{})
	default:
		return nil
	}
}

// Run forwards execution to the configured runner.
func (a *App) Run(ctx context.Context, args []string) error {
	return a.Runner.Run(ctx, args)
}
