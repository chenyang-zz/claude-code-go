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
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
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
	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
	"github.com/sheepzhao/claude-code-go/internal/platform/plugin"
	platformremote "github.com/sheepzhao/claude-code-go/internal/platform/remote"
	platformsqlite "github.com/sheepzhao/claude-code-go/internal/platform/store/sqlite"
	platformteam "github.com/sheepzhao/claude-code-go/internal/platform/team"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
	"github.com/sheepzhao/claude-code-go/internal/runtime/cron"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	"github.com/sheepzhao/claude-code-go/internal/runtime/executor"
	runtimehooks "github.com/sheepzhao/claude-code-go/internal/runtime/hooks"
	"github.com/sheepzhao/claude-code-go/internal/runtime/repl"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
	"github.com/sheepzhao/claude-code-go/internal/services/autodream"
	"github.com/sheepzhao/claude-code-go/internal/services/awaysummary"
	"github.com/sheepzhao/claude-code-go/internal/services/claudeailimits"
	servicecommands "github.com/sheepzhao/claude-code-go/internal/services/commands"
	"github.com/sheepzhao/claude-code-go/internal/services/policylimits"
	"github.com/sheepzhao/claude-code-go/internal/services/extractmemories"
	"github.com/sheepzhao/claude-code-go/internal/services/datetimeparser"
	"github.com/sheepzhao/claude-code-go/internal/services/haiku"
	"github.com/sheepzhao/claude-code-go/internal/services/internallogging"
	"github.com/sheepzhao/claude-code-go/internal/services/rename"
	"github.com/sheepzhao/claude-code-go/internal/services/sessiontitle"
	"github.com/sheepzhao/claude-code-go/internal/services/magicdocs"
	"github.com/sheepzhao/claude-code-go/internal/services/notifier"
	"github.com/sheepzhao/claude-code-go/internal/services/preventsleep"
	"github.com/sheepzhao/claude-code-go/internal/services/prompts"
	"github.com/sheepzhao/claude-code-go/internal/services/promptsuggestion"
	"github.com/sheepzhao/claude-code-go/internal/services/settingssync"
	"github.com/sheepzhao/claude-code-go/internal/services/teammemsync"
	"github.com/sheepzhao/claude-code-go/internal/services/tips"
	agenttool "github.com/sheepzhao/claude-code-go/internal/services/tools/agent"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/agent/builtin"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/agent/loader"
	mcpproxy "github.com/sheepzhao/claude-code-go/internal/services/tools/mcp"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill/bundled"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/tool_search"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/web_search"
	"github.com/sheepzhao/claude-code-go/internal/services/toolusesummary"
	"github.com/sheepzhao/claude-code-go/internal/ui/console"
	"github.com/sheepzhao/claude-code-go/internal/ui/jsonout"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// EngineAssembly bundles the engine runtime together with the subsystems created
// inside the engine factory so that bootstrap can wire them into downstream
// components such as the PluginRegistrar.
type EngineAssembly struct {
	Engine         engine.Engine
	Policy         *corepermission.FilesystemPolicy
	McpRegistry    *mcpregistry.ServerRegistry
	HookRunner     engine.HookRunner
	Hooks          *hook.HooksConfig
	StatsCollector *model.RuntimeStats
}

// EngineFactory constructs the engine selected by the resolved runtime config together with the shared filesystem policy.
type EngineFactory func(cfg coreconfig.Config, backgroundTaskStore *runtimesession.BackgroundTaskStore, taskStore coretask.Store) (*EngineAssembly, error)

// App wires together the minimum batch-07 runtime needed by cmd/cc.
type App struct {
	// Config stores the resolved runtime configuration for observability and tests.
	Config coreconfig.Config
	// Runner owns the one-turn REPL execution flow.
	Runner *repl.Runner
	// Scheduler runs cron scheduled tasks.
	Scheduler *cron.Scheduler
	// PluginWatcher monitors plugin directories for changes.
	PluginWatcher *plugin.Watcher
	// PromptSuggestionCleanup aborts in-progress suggestion/speculation work on shutdown.
	PromptSuggestionCleanup func()
	// PreventSleepCleanup tears down the macOS sleep-prevention service
	// (caffeinate subprocess + restart loop) on shutdown. nil when the
	// service is disabled by FlagPreventSleep.
	PreventSleepCleanup func()
	// Notifier dispatches terminal notifications (iTerm2 / Kitty / Ghostty
	// / bell / auto-detect). nil when FlagNotifier is disabled.
	Notifier *notifier.Service
	// Haiku is the single-prompt Haiku query helper used by downstream
	// services (e.g. tool use summary). nil when the Anthropic provider
	// is not selected or FlagHaikuQuery is disabled.
	Haiku *haiku.Service
	// ToolUseSummary generates ~30-character labels for completed tool
	// batches via the Haiku helper. nil when FlagToolUseSummary is
	// disabled or Haiku is unavailable.
	ToolUseSummary *toolusesummary.Service
	// SessionTitle generates a concise sentence-case title for the
	// current session via the Haiku helper. nil when FlagSessionTitle is
	// disabled or Haiku is unavailable.
	SessionTitle *sessiontitle.Service
	// Rename generates a kebab-case session name suggestion via the
	// Haiku helper. nil when FlagRenameSuggestion is disabled or Haiku
	// is unavailable.
	Rename *rename.Service
	// DateTimeParser converts natural-language dates and times into
	// ISO 8601 format via the Haiku helper. nil when FlagDateTimeParser
	// is disabled or Haiku is unavailable.
	DateTimeParser *datetimeparser.Service
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

	// Workspace trust dialog: prompt the user in interactive sessions when the
	// current directory has not been explicitly trusted. Untrusted directories
	// receive only the safe-env subset from project/local settings.
	if fileLoader, ok := loader.(*platformconfig.FileLoader); ok {
		trustState, err := platformconfig.LoadTrustState(fileLoader.HomeDir)
		if err != nil {
			logger.WarnF("failed to load trust state", map[string]any{"error": err})
			trustState = platformconfig.NewTrustState()
		}

		if !platformconfig.IsTrustAccepted(trustState, fileLoader.CWD, fileLoader.HomeDir) {
			result, shown, err := console.TrustDialog(fileLoader.CWD)
			if err != nil {
				return nil, fmt.Errorf("trust dialog: %w", err)
			}
			if shown && result == console.TrustResultRejected {
				return nil, fmt.Errorf("workspace trust declined")
			}
			if result == console.TrustResultAccepted {
				platformconfig.AcceptTrust(trustState, fileLoader.CWD)
				if err := platformconfig.SaveTrustState(fileLoader.HomeDir, trustState); err != nil {
					logger.WarnF("failed to save trust state", map[string]any{"error": err})
				}
				fileLoader.TrustAccepted = true
				// Re-load configuration so that project/local env vars are no longer
				// restricted to the safe-only allowlist.
				cfg, err = loader.Load(context.Background())
				if err != nil {
					return nil, err
				}
			}
		} else {
			fileLoader.TrustAccepted = true
		}
	}

	configureConsoleLogging(cfg.OutputFormat)
	applyRuntimeEnvironment(cfg.Env)

	backgroundTaskStore := runtimesession.NewBackgroundTaskStore()
	taskStore := resolveTaskStore(loader, cfg.HomeDir)
	assembly, err := engineFactory(cfg, backgroundTaskStore, taskStore)
	if err != nil {
		return nil, err
	}
	eng := assembly.Engine
	policy := assembly.Policy

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
	pluginLoader := plugin.NewPluginLoader(plugin.NewInstalledPluginsStore())
	commandRegistry, err := newCommandRegistry(&cfg, runner, globalSettingsStore, projectSettingsStore, localSettingsStore, policy, backgroundTaskStore, taskStore, agentRegistry, pluginLoader, assembly.StatsCollector)
	if err != nil {
		return nil, err
	}
	runner.Commands = commandRegistry

	// Two-phase assembly: build the full PluginRegistrar now that engine
	// subsystems are available, then replace the stub reload-plugins command.
	registrar := plugin.NewPluginRegistrarWithConfigs(
		agentRegistry,
		commandRegistry,
		assembly.Hooks,
		assembly.McpRegistry,
		nil, // LspManager not wired in bootstrap yet
		cfg.PluginConfigs,
	)

	// Build the reloader pipeline for both manual /reload-plugins and
	// automatic file-watcher-triggered reloads.
	var reloader *plugin.Reloader
	if pluginLoader != nil && registrar != nil {
		reloader = plugin.NewReloader(pluginLoader, registrar, cfg.Hooks)
	}

	if err := commandRegistry.Unregister("reload-plugins"); err != nil {
		logger.WarnCF("bootstrap", "failed to unregister reload-plugins", map[string]any{"error": err.Error()})
	}
	if err := commandRegistry.Register(servicecommands.ReloadPluginsCommand{
		Reloader:  reloader,
		Loader:    pluginLoader,
		Registrar: registrar,
	}); err != nil {
		logger.WarnCF("bootstrap", "failed to register reload-plugins with reloader", map[string]any{"error": err.Error()})
	}

	// Auto-load plugins at startup.
	if reloader != nil {
		if summary, err := reloader.Reload(); err != nil {
			logger.WarnCF("bootstrap", "failed to reload plugins at startup", map[string]any{"error": err.Error()})
		} else if summary != nil {
			logger.InfoCF("bootstrap", "plugins auto-loaded at startup", map[string]any{
				"agents":   summary.AgentsRegistered,
				"commands": summary.CommandsRegistered,
				"mcp":      summary.McpServersLoaded,
				"lsp":      summary.LspServersRegistered,
				"errors":   len(summary.Errors),
			})
		}
	}

	// Start the plugin directory watcher for automatic hot-reload.
	var pluginWatcher *plugin.Watcher
	if reloader != nil {
		pluginWatcher = plugin.NewWatcher(func() {
			logger.DebugCF("bootstrap", "plugin directory change detected, triggering reload", nil)
			_, _ = reloader.Reload()
		})
		var watchDirs []string
		if cfg.HomeDir != "" {
			watchDirs = append(watchDirs, filepath.Join(cfg.HomeDir, ".claude", "plugins"))
		}
		if cfg.ProjectPath != "" {
			watchDirs = append(watchDirs, filepath.Join(cfg.ProjectPath, ".claude", "plugins"))
		}
		if err := pluginWatcher.Start(watchDirs); err != nil {
			logger.WarnCF("bootstrap", "failed to start plugin watcher", map[string]any{"error": err.Error()})
		} else {
			logger.DebugCF("bootstrap", "plugin watcher started", nil)
		}
	}

	logger.DebugCF("bootstrap", "constructed application", map[string]any{
		"provider":            cfg.Provider,
		"model":               cfg.Model,
		"has_session_db_path": cfg.SessionDBPath != "",
		"remote_mode":         cfg.RemoteSession.Enabled,
		"output_format":       cfg.OutputFormat,
	})

	// Initialize Magic Docs system (detection and tracking; subagent execution wired later).
	magicdocs.InitMagicDocs(nil, func(hook magicdocs.PostTurnHookFunc) {
		engine.RegisterPostTurnHook(engine.PostTurnHook(hook))
	})

	// Initialize Extract Memories system (background memory extraction via forked subagent).
	extractmemories.InitExtractMemories(nil, func(hook extractmemories.PostTurnHookFunc) {
		engine.RegisterPostTurnHook(engine.PostTurnHook(hook))
	}, cfg.ProjectPath)

	// Initialize autoDream system (background memory consolidation via forked subagent).
	autodream.InitAutoDream(nil, func(hook autodream.PostTurnHookFunc) {
		engine.RegisterPostTurnHook(engine.PostTurnHook(hook))
	}, cfg.ProjectPath)


	// Initialize PromptSuggestion system (post-sampling suggestion generation).
	_, psCleanup := promptsuggestion.Init(nil, func(hook promptsuggestion.PostSamplingHookFunc) {
		engine.RegisterPostSamplingHook(engine.PostSamplingHook(hook))
	}, cfg.ProjectPath)

	// Initialize Tips system (spinner contextual usage tips).
	tips.Init(globalSettingsStore)

	// Initialize Claude.ai rate limit observation system. Bound to the
	// global settings store so the persisted snapshot survives restarts,
	// and to a credential-store-backed loader so subscription gating
	// (used by `messages.go` upsell branches) can read SubscriptionType
	// without an extra round trip.
	var claudeAILimitsLoader claudeailimits.SubscriptionLoader
	if homeDir := strings.TrimSpace(cfg.HomeDir); homeDir != "" {
		if store, storeErr := oauth.NewOAuthCredentialStore(homeDir); storeErr == nil {
			claudeAILimitsLoader = claudeailimits.LoadOAuthTokensFromStore(store)
		}
	}
	claudeailimits.Init(claudeailimits.InitOptions{
		Store:              globalSettingsStore,
		SubscriptionLoader: claudeAILimitsLoader,
	})

	// Initialize organizational policy limits system.
	policylimits.Init(policylimits.InitOptions{
		HomeDir:            strings.TrimSpace(cfg.HomeDir),
		Config:             &cfg,
		SubscriptionLoader: claudeAILimitsLoader,
	})

	// Initialize settings sync system (cross-device settings synchronization).
	settingssync.Init(settingssync.InitOptions{
		HomeDir:            strings.TrimSpace(cfg.HomeDir),
		Config:             &cfg,
		SubscriptionLoader: claudeAILimitsLoader,
	})

	// Initialize team memory sync system (team memory file synchronization).
	teammemsync.Init(teammemsync.InitOptions{
		HomeDir:     strings.TrimSpace(cfg.HomeDir),
		Config:      &cfg,
		ProjectRoot: cfg.ProjectPath,
	})

	// Initialize OS platform services (notifier / preventSleep / internalLogging).
	// Each service is gated by an independent feature flag (off by default)
	// so deployments can opt into terminal notifications, macOS sleep
	// prevention, and Ant-internal diagnostic logging without changing the
	// wiring topology.
	var notifierService *notifier.Service
	if notifier.IsNotifierEnabled() {
		// engine.HookRunner does not expose RunNotificationHooks (it is a
		// method on *engine.Runtime, not the abstract HookRunner interface).
		// Both DefaultEngineFactory branches store *engine.Runtime in
		// EngineAssembly.Engine, so a type assertion recovers the concrete
		// dispatcher; if a future factory returns a different Engine impl
		// the notifier simply skips the hook step.
		var hookRunner notifier.HookRunner
		if rt, ok := eng.(*engine.Runtime); ok {
			hookRunner = rt
		}
		projectPath := cfg.ProjectPath
		notifierService = notifier.Init(notifier.InitOptions{
			HookRunner: hookRunner,
			ChannelGetter: func() string {
				return strings.TrimSpace(os.Getenv("CLAUDE_NOTIFIER_CHANNEL"))
			},
			CWDGetter: func() string { return projectPath },
		})
	}

	var preventSleepCleanup func()
	if preventsleep.IsPreventSleepEnabled() {
		preventSleepCleanup = preventsleep.Init(preventsleep.InitOptions{})
	}

	if internallogging.IsInternalLoggingEnabled() {
		internallogging.Init(internallogging.InitOptions{})
	}

	scheduler := cron.NewScheduler(cron.SchedulerOptions{
		ProjectRoot: cfg.ProjectPath,
	})
	scheduler.Start()

	return &App{
		Config:                  cfg,
		Runner:                  runner,
		Scheduler:               scheduler,
		PluginWatcher:           pluginWatcher,
		PromptSuggestionCleanup: psCleanup,
		PreventSleepCleanup:     preventSleepCleanup,
		Notifier:                notifierService,
		Haiku:                   haiku.CurrentService(),
		ToolUseSummary:          toolusesummary.CurrentService(),
		SessionTitle:            sessiontitle.CurrentService(),
		Rename:                  rename.CurrentService(),
		DateTimeParser:          datetimeparser.CurrentService(),
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
func newCommandRegistry(cfg *coreconfig.Config, runner *repl.Runner, globalSettingsStore *platformconfig.GlobalSettingsStore, projectSettingsStore *platformconfig.ProjectSettingsStore, localSettingsStore *platformconfig.LocalSettingsStore, policy *corepermission.FilesystemPolicy, backgroundTaskStore *runtimesession.BackgroundTaskStore, taskStore coretask.Store, agentRegistry agent.Registry, pluginLoader *plugin.PluginLoader, statsCollector *model.RuntimeStats) (command.Registry, error) {
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
	statusToolRegistry, err := wiring.NewModules(wiring.BaseWorkspaceTools(platformfs.NewLocalFS(), policy, cfg.Permissions, backgroundTaskStore, taskStore, cfg.HomeDir, cfg.ProjectPath)...)
	if err != nil {
		return nil, err
	}
	tool_search.SharedRegistry = statusToolRegistry.Tools
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
	loginRunner, loginRunnerErr := servicecommands.NewLoginRunner(servicecommands.LoginRunnerDeps{
		HomeDir:    dereferenceConfig(cfg).HomeDir,
		ProjectDir: dereferenceConfig(cfg).ProjectPath,
	})
	if loginRunnerErr != nil {
		logger.WarnCF("commands", "interactive /login OAuth runner unavailable; falling back to placeholder text", map[string]any{
			"error": loginRunnerErr.Error(),
		})
		loginRunner = nil
	}
	if err := registry.Register(servicecommands.LoginCommand{
		Config: dereferenceConfig(cfg),
		Login:  loginRunner,
	}); err != nil {
		return nil, err
	}
	var logoutCredentialStore *oauth.OAuthCredentialStore
	if dereferenceConfig(cfg).HomeDir != "" {
		logoutCredentialStore, _ = oauth.NewOAuthCredentialStore(dereferenceConfig(cfg).HomeDir)
	}
	var logoutSettingsWriter *platformconfig.SettingsWriter
	if dereferenceConfig(cfg).HomeDir != "" {
		logoutSettingsWriter = platformconfig.NewSettingsWriter(dereferenceConfig(cfg).HomeDir, dereferenceConfig(cfg).ProjectPath)
	}
	if err := registry.Register(servicecommands.LogoutCommand{
		Config:          dereferenceConfig(cfg),
		CredentialStore: logoutCredentialStore,
		SettingsWriter:  logoutSettingsWriter,
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.CostCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.StatusCommand{
		Config:         dereferenceConfig(cfg),
		ToolRegistry:   statusToolRegistry.Tools,
		APIProbe:       buildStatusProbe(cfg),
		StatsCollector: statsCollector,
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
	if err := registry.Register(servicecommands.DesktopCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.MobileCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.FeedbackCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ExitCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.InstallCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ContextCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ReviewCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.RewindCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.SkillsCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.TagCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ColorCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.PassesCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.RateLimitOptionsCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.SandboxCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.StickersCommand{}); err != nil {
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
	if err := registry.Register(servicecommands.PluginCommand{Loader: pluginLoader}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.HooksCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.BtwCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ChromeCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ThinkBackCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ThinkbackPlayCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ReloadPluginsCommand{Loader: pluginLoader}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.AdvisorCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.StatuslineCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.UltrareviewCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.InsightsCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.RemoteControlCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.BridgeKickCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.CommitCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.CommitPushPRCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.HeapdumpCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.WebSetupCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.BriefCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.UltraplanCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ShareCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.SummaryCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.EnvCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.TeleportCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.OnboardingCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.OAuthRefreshCommand{
		CredentialStore: logoutCredentialStore,
		SettingsWriter:  logoutSettingsWriter,
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.IssueCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.GoodClaudeCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.BughunterCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.BreakCacheCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.CtxVizCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.BackfillSessionsCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.InitVerifiersCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.MockLimitsCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.ResetLimitsCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.AntTraceCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.PerfIssueCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.DebugToolCallCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.AgentsPlatformCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.AutofixPRCommand{}); err != nil {
		return nil, err
	}
	if err := registry.Register(servicecommands.SeedSessionsCommand{
		Repository:  sessionRepository,
		ProjectPath: dereferenceConfig(cfg).ProjectPath,
	}); err != nil {
		return nil, err
	}
	// Load skills from all sources (project, user, managed, bundled)
	// and register them into the command registry so SkillTool can invoke them.
	homeDir := dereferenceConfig(cfg).HomeDir
	projectPath := dereferenceConfig(cfg).ProjectPath

	var allSkills []*skill.Skill

	// Project skills: from .claude/skills/ in CWD and parent dirs up to $HOME.
	if projectPath != "" {
		projectSkills, loadErrors, err := skill.LoadProjectSkills(projectPath, homeDir)
		if err != nil {
			logger.WarnCF("bootstrap", "failed to load project skills", map[string]any{
				"error": err.Error(),
			})
		}
		for _, le := range loadErrors {
			logger.WarnCF("bootstrap", "skill load error", map[string]any{
				"skill": le.Name,
				"error": le.Error,
			})
		}
		allSkills = append(allSkills, projectSkills...)
	}

	// User skills: from ~/.claude/skills/.
	if homeDir != "" {
		userSkills, loadErrors, err := skill.LoadUserSkills(homeDir)
		if err != nil {
			logger.WarnCF("bootstrap", "failed to load user skills", map[string]any{
				"error": err.Error(),
			})
		}
		for _, le := range loadErrors {
			logger.WarnCF("bootstrap", "user skill load error", map[string]any{
				"skill": le.Name,
				"error": le.Error,
			})
		}
		allSkills = append(allSkills, userSkills...)
	}

	// Managed skills: from ~/.claude/managed/.claude/skills/.
	if homeDir != "" {
		managedSkills, loadErrors, err := skill.LoadManagedSkills(homeDir)
		if err != nil {
			logger.WarnCF("bootstrap", "failed to load managed skills", map[string]any{
				"error": err.Error(),
			})
		}
		for _, le := range loadErrors {
			logger.WarnCF("bootstrap", "managed skill load error", map[string]any{
				"skill": le.Name,
				"error": le.Error,
			})
		}
		allSkills = append(allSkills, managedSkills...)
	}

	// Legacy /commands/ loading: load old-format skills from .claude/commands/.
	if projectPath != "" {
		commandsSkills, loadErrors, err := skill.LoadSkillsFromCommandsDir(projectPath)
		if err != nil {
			logger.WarnCF("bootstrap", "failed to load commands skills", map[string]any{
				"error": err.Error(),
			})
		}
		for _, le := range loadErrors {
			logger.WarnCF("bootstrap", "commands skill load error", map[string]any{
				"skill": le.Name,
				"error": le.Error,
			})
		}
		allSkills = append(allSkills, commandsSkills...)
	}

	// Deduplicate by canonical path, then separate conditional skills
	// (skills with paths frontmatter that activate when matching files are touched).
	allSkills = skill.DeduplicateByPath(allSkills)
	allSkills = skill.SeparateConditionalSkills(allSkills)

	if registered := skill.RegisterSkills(registry, allSkills, "skills"); registered > 0 {
		logger.InfoCF("bootstrap", "loaded skills", map[string]any{
			"count": registered,
		})
	}

	// Initialize and register bundled skills.
	bundled.InitBundledSkills()
	for _, bs := range skill.GetBundledSkills() {
		if err := registry.Register(bs); err != nil {
			logger.DebugCF("bootstrap", "failed to register bundled skill", map[string]any{
				"name":  bs.Metadata().Name,
				"error": err.Error(),
			})
		}
	}

	// Publish the command registry for SkillTool to look up skills at invoke time.
	skill.SharedRegistry = registry

	// Start the file watcher for skill and command directories.
	skill.StartDefaultWatcher(homeDir, projectPath)
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
func DefaultEngineFactory(cfg coreconfig.Config, backgroundTaskStore *runtimesession.BackgroundTaskStore, taskStore coretask.Store) (*EngineAssembly, error) {
	sharedRuntimeStats := &model.RuntimeStats{}
	filesystem := platformfs.NewLocalFS()
	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		return nil, err
	}
	for _, configured := range cfg.Permissions.AdditionalDirectories {
		expanded, err := platformfs.ExpandPath(configured, cfg.ProjectPath)
		if err != nil {
			return nil, fmt.Errorf("expand configured additional directory %q: %w", configured, err)
		}
		policy.AddReadRoot(expanded)
	}

	hookRunner := runtimehooks.NewRunner()
	modules, err := wiring.NewBaseWorkspaceModulesWithHooks(filesystem, policy, cfg.Permissions, backgroundTaskStore, taskStore, hookRunner, cfg.Hooks, cfg.DisableAllHooks, cfg.HomeDir, cfg.ProjectPath)
	if err != nil {
		return nil, err
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

	rateLimitConsumer := claudeailimits.MakeAnthropicConsumer()
	rateLimitErrorAnnotator := claudeailimits.MakeErrorAnnotator()

	switch coreconfig.NormalizeProvider(cfg.Provider) {
	case coreconfig.ProviderAnthropic, coreconfig.ProviderVertex, coreconfig.ProviderBedrock, coreconfig.ProviderFoundry:
		var client model.Client
		switch coreconfig.NormalizeProvider(cfg.Provider) {
		case coreconfig.ProviderVertex:
			// Vertex / Bedrock / Foundry 429 errors are billing-tier
			// rejections, not Claude.ai subscription limits, so the
			// annotator that renders Claude.ai plan-limit text is
			// only attached to the first-party Anthropic client.
			client = anthropic.NewClient(anthropic.Config{
				APIKey:            cfg.APIKey,
				AuthToken:         cfg.AuthToken,
				BaseURL:           cfg.APIBaseURL,
				HTTPClient:        nil,
				IsFirstParty:      false,
				VertexEnabled:     true,
				VertexProjectID:   cfg.VertexProjectID,
				VertexRegion:      cfg.VertexRegion,
				VertexSkipAuth:    cfg.VertexSkipAuth,
				RateLimitConsumer: rateLimitConsumer,
			})
		case coreconfig.ProviderBedrock:
			client = anthropic.NewClient(anthropic.Config{
				APIKey:            cfg.APIKey,
				AuthToken:         cfg.AuthToken,
				BaseURL:           cfg.APIBaseURL,
				HTTPClient:        nil,
				IsFirstParty:      false,
				BedrockEnabled:    true,
				BedrockRegion:     cfg.BedrockRegion,
				BedrockModelID:    cfg.BedrockModelID,
				BedrockSkipAuth:   cfg.BedrockSkipAuth,
				RateLimitConsumer: rateLimitConsumer,
			})
		case coreconfig.ProviderFoundry:
			client = anthropic.NewClient(anthropic.Config{
				APIKey:            cfg.APIKey,
				AuthToken:         cfg.AuthToken,
				BaseURL:           cfg.APIBaseURL,
				HTTPClient:        nil,
				IsFirstParty:      false,
				FoundryEnabled:    true,
				FoundryResource:   cfg.FoundryResource,
				FoundryBaseURL:    cfg.FoundryBaseURL,
				FoundryAPIKey:     cfg.FoundryAPIKey,
				FoundrySkipAuth:   cfg.FoundrySkipAuth,
				RateLimitConsumer: rateLimitConsumer,
			})
		default:
			client = anthropic.NewClient(anthropic.Config{
				APIKey:                  cfg.APIKey,
				AuthToken:               cfg.AuthToken,
				BaseURL:                 cfg.APIBaseURL,
				HTTPClient:              nil,
				IsFirstParty:            true,
				RateLimitConsumer:       rateLimitConsumer,
				RateLimitErrorAnnotator: rateLimitErrorAnnotator,
			})
		}

		runtime := engine.New(client, cfg.Model, toolExecutor, toolCatalog...)
		runtime.StatsCollector = sharedRuntimeStats
		runtime.StatsCollector = sharedRuntimeStats
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
			agentTool := agenttool.NewTool(agentRegistry, runtime, mcpServerRegistry, modules.Tools, backgroundTaskStore)
			if regErr := modules.Tools.Register(agentTool); regErr != nil {
				logger.WarnCF("bootstrap", "failed to register agent tool", map[string]any{"error": regErr.Error()})
			} else {
				// Re-describe tools so the runtime catalog includes the Agent tool.
				runtime.ToolCatalog = engine.DescribeTools(modules.Tools)
			}
		}

		// Register the WebSearch tool using the runtime's model client for sub-calls.
		webSearchTool := web_search.NewTool(runtime.Client, cfg.Model)
		if regErr := modules.Tools.Register(webSearchTool); regErr != nil {
			logger.WarnCF("bootstrap", "failed to register web_search tool", map[string]any{"error": regErr.Error()})
		} else {
			runtime.ToolCatalog = engine.DescribeTools(modules.Tools)
		}

		// Initialize AwaySummary system with the live model client.
		awaysummary.InitAwaySummary(runtime.Client, func(hook awaysummary.PostTurnHookFunc) {
			engine.RegisterPostTurnHook(engine.PostTurnHook(hook))
		}, extractmemories.GetAutoMemPath(cfg.ProjectPath), awaysummary.DefaultConfig())

		// Initialize Haiku helper bound to the live Anthropic client. The
		// Haiku models served here are the source for downstream small/fast
		// query helpers (tool use summary, etc.). InitHaiku resets the
		// singleton to nil when FlagHaikuQuery is off or runtime.Client is nil.
		haiku.InitHaiku(haiku.InitOptions{Client: runtime.Client})

		// Initialize the tool use summary service against the haiku
		// singleton. The querier handle is captured eagerly so the service
		// remains usable even if haiku is later re-initialised.
		toolusesummary.InitToolUseSummary(toolusesummary.InitOptions{
			Querier: haiku.CurrentService(),
		})

			// Initialize the session title service against the haiku singleton.
			sessiontitle.InitSessionTitle(sessiontitle.InitOptions{
				Querier: haiku.CurrentService(),
			})

			// Initialize the rename suggestion service against the haiku singleton.
			rename.InitRename(rename.InitOptions{
				Querier: haiku.CurrentService(),
			})

			// Initialize the date/time parser service against the haiku singleton.
			datetimeparser.InitDateTimeParser(datetimeparser.InitOptions{
				Querier: haiku.CurrentService(),
			})


		return &EngineAssembly{
			Engine:         runtime,
			Policy:         policy,
			McpRegistry:    mcpServerRegistry,
			HookRunner:     hookRunner,
			Hooks:          &runtime.Hooks,
			StatsCollector: sharedRuntimeStats,
		}, nil
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
		runtime.StatsCollector = sharedRuntimeStats
		runtime.Hooks = cfg.Hooks
		runtime.DisableAllHooks = cfg.DisableAllHooks
		runtime.HookRunner = hookRunner
		runtime.EnablePromptCaching = cfg.EnablePromptCaching
		runtime.Source = "repl_main_thread"
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
			agentTool := agenttool.NewTool(agentRegistry, runtime, mcpServerRegistry, modules.Tools, backgroundTaskStore)
			if regErr := modules.Tools.Register(agentTool); regErr != nil {
				logger.WarnCF("bootstrap", "failed to register agent tool", map[string]any{"error": regErr.Error()})
			} else {
				// Re-describe tools so the runtime catalog includes the Agent tool.
				runtime.ToolCatalog = engine.DescribeTools(modules.Tools)
			}
		}

		// Register the WebSearch tool using the runtime's model client for sub-calls.
		webSearchTool := web_search.NewTool(runtime.Client, cfg.Model)
		if regErr := modules.Tools.Register(webSearchTool); regErr != nil {
			logger.WarnCF("bootstrap", "failed to register web_search tool", map[string]any{"error": regErr.Error()})
		} else {
			runtime.ToolCatalog = engine.DescribeTools(modules.Tools)
		}

		// Haiku models are Anthropic-only; reset any stale singleton when
		// bootstrapping with a non-Anthropic provider to avoid cross-provider
		// leakage in long-lived test or REPL processes.
		haiku.InitHaiku(haiku.InitOptions{})
		toolusesummary.InitToolUseSummary(toolusesummary.InitOptions{})
		sessiontitle.InitSessionTitle(sessiontitle.InitOptions{})
		rename.InitRename(rename.InitOptions{})
		datetimeparser.InitDateTimeParser(datetimeparser.InitOptions{})

		return &EngineAssembly{
			Engine:         runtime,
			Policy:         policy,
			McpRegistry:    mcpServerRegistry,
			HookRunner:     hookRunner,
			Hooks:          &runtime.Hooks,
			StatsCollector: sharedRuntimeStats,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
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
	snapshot.SettingSourcesFlag = cfg.SettingSourcesFlag
	snapshot.HasSettingSourcesFlag = cfg.HasSettingSourcesFlag

	return snapshot
}

// filterSettingsForSnapshot returns a safe subset of runtime configuration for guide agent prompts.
func filterSettingsForSnapshot(cfg coreconfig.Config) map[string]any {
	safe := map[string]any{
		"model":                  cfg.Model,
		"provider":               cfg.Provider,
		"effortLevel":            cfg.EffortLevel,
		"theme":                  cfg.Theme,
		"editorMode":             cfg.EditorMode,
		"approvalMode":           cfg.ApprovalMode,
		"outputFormat":           cfg.OutputFormat,
		"outputStyle":            cfg.OutputStyle,
		"language":               cfg.Language,
		"fastMode":               cfg.FastMode,
		"enablePromptCaching":    cfg.EnablePromptCaching,
		"autoUpdatesChannel":     cfg.AutoUpdatesChannel,
		"plansDirectory":         cfg.PlansDirectory,
		"skipWebFetchPreflight":  cfg.SkipWebFetchPreflight,
		"disableAllHooks":        cfg.DisableAllHooks,
		"allowManagedHooksOnly":  cfg.AllowManagedHooksOnly,
		"allowedHttpHookUrls":    cfg.AllowedHttpHookUrls,
		"httpHookAllowedEnvVars": cfg.HttpHookAllowedEnvVars,
		"channelsEnabled":        cfg.ChannelsEnabled,
		"claudeMdExcludes":       cfg.ClaudeMdExcludes,
		"additionalDirectories":  cfg.Permissions.AdditionalDirectories,
		"loadedSettingSources":   cfg.LoadedSettingSources,
		"policySettings": map[string]any{
			"origin":      cfg.PolicySettings.Origin,
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
		prompts.NotebookEditPromptSection{},
		prompts.WorktreePromptSection{},
		prompts.TodoV2PromptSection{},
		prompts.CronPromptSection{},
		prompts.TeamPromptSection{},
		prompts.AskUserQuestionPromptSection{},
		prompts.PlanModePromptSection{},
		prompts.SendMessagePromptSection{},
		prompts.RemoteTriggerPromptSection{},
		prompts.WebSearchPromptSection{},
		prompts.SkillPromptSection{},
		prompts.ToolSearchPromptSection{},
		prompts.MCPResourcePromptSection{},
		prompts.LSPPromptSection{},
		prompts.ConfigPromptSection{},
		prompts.BriefToolPromptSection{},
		prompts.SleepPromptSection{},
		prompts.TodoWritePromptSection{},
		prompts.TeamMemoryPromptSection{},
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
	case coreconfig.ProviderAnthropic, coreconfig.ProviderFoundry:
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
	defer a.Scheduler.Stop()
	if a.PluginWatcher != nil {
		defer a.PluginWatcher.Stop()
	}
	if a.PromptSuggestionCleanup != nil {
		defer a.PromptSuggestionCleanup()
	}
	if a.PreventSleepCleanup != nil {
		defer a.PreventSleepCleanup()
	}
	return a.Runner.Run(ctx, args)
}
