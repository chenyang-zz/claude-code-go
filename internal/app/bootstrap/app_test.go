package bootstrap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretask "github.com/sheepzhao/claude-code-go/internal/core/task"
	"github.com/sheepzhao/claude-code-go/internal/platform/api/anthropic"
	"github.com/sheepzhao/claude-code-go/internal/platform/api/openai"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

type stubLoader struct {
	cfg coreconfig.Config
}

func TestConfigureConsoleLoggingStreamJSONUsesStderr(t *testing.T) {
	stdoutFile, err := os.CreateTemp(t.TempDir(), "stdout-*.log")
	if err != nil {
		t.Fatalf("CreateTemp(stdout) error = %v", err)
	}
	stderrFile, err := os.CreateTemp(t.TempDir(), "stderr-*.log")
	if err != nil {
		t.Fatalf("CreateTemp(stderr) error = %v", err)
	}

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	originalLevel := logger.GetLevel()
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
		configureConsoleLogging("console")
		logger.SetLevel(originalLevel)
	}()

	os.Stdout = stdoutFile
	os.Stderr = stderrFile
	configureConsoleLogging("stream-json")
	logger.SetLevel(logger.DEBUG)
	logger.Debug("stream-json debug log")

	if err := stdoutFile.Sync(); err != nil {
		t.Fatalf("stdout Sync() error = %v", err)
	}
	if err := stderrFile.Sync(); err != nil {
		t.Fatalf("stderr Sync() error = %v", err)
	}

	stdoutBytes, err := os.ReadFile(stdoutFile.Name())
	if err != nil {
		t.Fatalf("ReadFile(stdout) error = %v", err)
	}
	stderrBytes, err := os.ReadFile(stderrFile.Name())
	if err != nil {
		t.Fatalf("ReadFile(stderr) error = %v", err)
	}

	if strings.Contains(string(stdoutBytes), "stream-json debug log") {
		t.Fatalf("stdout contains debug log in stream-json mode: %q", string(stdoutBytes))
	}
	if !strings.Contains(string(stderrBytes), "stream-json debug log") {
		t.Fatalf("stderr missing debug log in stream-json mode: %q", string(stderrBytes))
	}
}

func TestResolveTaskStoreUsesStableDefaultTaskListPath(t *testing.T) {
	t.Setenv("CLAUDE_CODE_TASK_LIST_ID", "")

	homeDir := t.TempDir()
	store := resolveTaskStore(stubLoader{}, homeDir)
	if store == nil {
		t.Fatal("resolveTaskStore() = nil, want file store")
	}

	id, err := store.Create(context.Background(), coretask.NewTask{Subject: "persist me"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	taskPath := filepath.Join(homeDir, ".claude", "tasks", "default", id+".json")
	if _, err := os.Stat(taskPath); err != nil {
		t.Fatalf("task file %q missing: %v", taskPath, err)
	}
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

	app, err := NewAppWithDependencies(loader, func(cfg coreconfig.Config, backgroundTaskStore *runtimesession.BackgroundTaskStore, taskStore coretask.Store) (engine.Engine, *corepermission.FilesystemPolicy, error) {
		_ = cfg
		_ = backgroundTaskStore
		_ = taskStore
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

func TestDefaultEngineFactoryWiresHookRuntimeConfig(t *testing.T) {
	cfg := coreconfig.Config{
		Provider:        coreconfig.ProviderAnthropic,
		Model:           "claude-sonnet-4-5",
		APIKey:          "test-key",
		ApprovalMode:    "default",
		DisableAllHooks: true,
		Hooks: hook.HooksConfig{
			hook.EventStop: []hook.HookMatcher{{
				Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo ok"}`)},
			}},
		},
	}

	eng, _, err := DefaultEngineFactory(cfg, nil, nil)
	if err != nil {
		t.Fatalf("DefaultEngineFactory() error = %v", err)
	}

	runtime, ok := eng.(*engine.Runtime)
	if !ok {
		t.Fatalf("DefaultEngineFactory() engine type = %T, want *engine.Runtime", eng)
	}
	if runtime.HookRunner == nil {
		t.Fatal("DefaultEngineFactory() HookRunner = nil, want runner")
	}
	if !runtime.DisableAllHooks {
		t.Fatal("DefaultEngineFactory() DisableAllHooks = false, want true")
	}
	if !reflect.DeepEqual(runtime.Hooks, cfg.Hooks) {
		t.Fatalf("DefaultEngineFactory() Hooks = %#v, want %#v", runtime.Hooks, cfg.Hooks)
	}
}

// TestNewAppWithDependenciesAppliesSettingsEnv verifies bootstrap writes merged settings.env into the current process.
func TestNewAppWithDependenciesAppliesSettingsEnv(t *testing.T) {
	const envKey = "CLAUDE_CODE_BOOTSTRAP_ENV_TEST"
	t.Setenv(envKey, "host")

	loader := stubLoader{
		cfg: coreconfig.Config{
			Provider: "anthropic",
			Model:    "claude-sonnet-4-5",
			Env: map[string]string{
				envKey: "settings",
			},
		},
	}

	_, err := NewAppWithDependencies(loader, func(cfg coreconfig.Config, backgroundTaskStore *runtimesession.BackgroundTaskStore, taskStore coretask.Store) (engine.Engine, *corepermission.FilesystemPolicy, error) {
		if got := os.Getenv(envKey); got != "settings" {
			t.Fatalf("engineFactory observed %s = %q, want settings", envKey, got)
		}
		_ = backgroundTaskStore
		_ = taskStore
		policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
		if err != nil {
			return nil, nil, err
		}
		return stubEngine{}, policy, nil
	})
	if err != nil {
		t.Fatalf("NewAppWithDependencies() error = %v", err)
	}
	if got := os.Getenv(envKey); got != "settings" {
		t.Fatalf("process env %s = %q, want settings", envKey, got)
	}
}

// TestLoadMCPConfigsPreservesOAuthConfig verifies bootstrap keeps MCP oauth metadata intact.
func TestLoadMCPConfigsPreservesOAuthConfig(t *testing.T) {
	t.Setenv("CLAUDE_CODE_MCP_SERVERS", `{
		"proxy": {
			"type": "http",
			"url": "https://example.invalid/mcp",
			"oauth": {
				"clientId": "client-123",
				"callbackPort": 4321,
				"authServerMetadataUrl": "https://auth.example.invalid/.well-known/oauth-authorization-server",
				"xaa": true
			}
		}
	}`)

	configs := loadMCPConfigs()
	cfg, ok := configs["proxy"]
	if !ok {
		t.Fatal("loadMCPConfigs() missing proxy config")
	}
	if cfg.OAuth == nil {
		t.Fatal("loadMCPConfigs() OAuth = nil, want oauth metadata")
	}
	if cfg.OAuth.ClientID != "client-123" {
		t.Fatalf("OAuth.ClientID = %q, want client-123", cfg.OAuth.ClientID)
	}
	if cfg.OAuth.CallbackPort == nil || *cfg.OAuth.CallbackPort != 4321 {
		t.Fatalf("OAuth.CallbackPort = %#v, want 4321", cfg.OAuth.CallbackPort)
	}
	if cfg.OAuth.AuthServerMetadataURL != "https://auth.example.invalid/.well-known/oauth-authorization-server" {
		t.Fatalf("OAuth.AuthServerMetadataURL = %q, want auth metadata url", cfg.OAuth.AuthServerMetadataURL)
	}
	if cfg.OAuth.XAA == nil || !*cfg.OAuth.XAA {
		t.Fatalf("OAuth.XAA = %#v, want true", cfg.OAuth.XAA)
	}
}

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
	app, err := NewAppWithDependencies(loader, func(cfg coreconfig.Config, backgroundTaskStore *runtimesession.BackgroundTaskStore, taskStore coretask.Store) (engine.Engine, *corepermission.FilesystemPolicy, error) {
		called = true
		if cfg.APIKey != "test-key" {
			t.Fatalf("engine factory cfg = %#v, want api key", cfg)
		}
		_ = backgroundTaskStore
		_ = taskStore
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
	if _, ok := app.Runner.Commands.Get("branch"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /branch command")
	}
	if _, ok := app.Runner.Commands.Get("fork"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /fork alias")
	}
	if _, ok := app.Runner.Commands.Get("voice"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /voice command")
	}
	if _, ok := app.Runner.Commands.Get("ide"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /ide command")
	}
	if _, ok := app.Runner.Commands.Get("init"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /init command")
	}
	if _, ok := app.Runner.Commands.Get("install-github-app"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /install-github-app command")
	}
	if _, ok := app.Runner.Commands.Get("install-slack-app"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /install-slack-app command")
	}
	if _, ok := app.Runner.Commands.Get("remote-env"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /remote-env command")
	}
	if _, ok := app.Runner.Commands.Get("desktop"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /desktop command")
	}
	if _, ok := app.Runner.Commands.Get("app"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /app alias")
	}
	if _, ok := app.Runner.Commands.Get("mobile"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /mobile command")
	}
	if _, ok := app.Runner.Commands.Get("ios"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /ios alias")
	}
	if _, ok := app.Runner.Commands.Get("android"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /android alias")
	}
	if _, ok := app.Runner.Commands.Get("feedback"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /feedback command")
	}
	if _, ok := app.Runner.Commands.Get("bug"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /bug alias")
	}
	if _, ok := app.Runner.Commands.Get("exit"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /exit command")
	}
	if _, ok := app.Runner.Commands.Get("quit"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /quit alias")
	}
	if _, ok := app.Runner.Commands.Get("install"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /install command")
	}
	if _, ok := app.Runner.Commands.Get("context"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /context command")
	}
	if _, ok := app.Runner.Commands.Get("review"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /review command")
	}
	if _, ok := app.Runner.Commands.Get("rewind"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /rewind command")
	}
	if _, ok := app.Runner.Commands.Get("checkpoint"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /checkpoint alias")
	}
	if _, ok := app.Runner.Commands.Get("skills"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /skills command")
	}
	if _, ok := app.Runner.Commands.Get("tag"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /tag command")
	}
	if _, ok := app.Runner.Commands.Get("color"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /color command")
	}
	if _, ok := app.Runner.Commands.Get("passes"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /passes command")
	}
	if _, ok := app.Runner.Commands.Get("rate-limit-options"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /rate-limit-options command")
	}
	if _, ok := app.Runner.Commands.Get("sandbox"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /sandbox command")
	}
	if _, ok := app.Runner.Commands.Get("stickers"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /stickers command")
	}
	if _, ok := app.Runner.Commands.Get("privacy-settings"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /privacy-settings command")
	}
	if _, ok := app.Runner.Commands.Get("plan"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /plan command")
	}
	if _, ok := app.Runner.Commands.Get("tasks"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /tasks command")
	}
	if _, ok := app.Runner.Commands.Get("bashes"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /bashes alias")
	}
	if _, ok := app.Runner.Commands.Get("diff"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /diff command")
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
	if _, ok := app.Runner.Commands.Get("version"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /version command")
	}
	if _, ok := app.Runner.Commands.Get("release-notes"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /release-notes command")
	}
	if _, ok := app.Runner.Commands.Get("upgrade"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /upgrade command")
	}
	if _, ok := app.Runner.Commands.Get("usage"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /usage command")
	}
	if _, ok := app.Runner.Commands.Get("stats"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /stats command")
	}
	if _, ok := app.Runner.Commands.Get("extra-usage"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /extra-usage command")
	}
	if _, ok := app.Runner.Commands.Get("theme"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /theme command")
	}
	if _, ok := app.Runner.Commands.Get("vim"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /vim command")
	}
	if _, ok := app.Runner.Commands.Get("terminal-setup"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /terminal-setup command")
	}
	if _, ok := app.Runner.Commands.Get("keybindings"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /keybindings command")
	}
	if _, ok := app.Runner.Commands.Get("pr-comments"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /pr-comments command")
	}
	if _, ok := app.Runner.Commands.Get("security-review"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /security-review command")
	}
	if _, ok := app.Runner.Commands.Get("agents"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /agents command")
	}
	if _, ok := app.Runner.Commands.Get("plugin"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /plugin command")
	}
	if _, ok := app.Runner.Commands.Get("plugins"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /plugins alias")
	}
	if _, ok := app.Runner.Commands.Get("marketplace"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /marketplace alias")
	}
	if _, ok := app.Runner.Commands.Get("hooks"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /hooks command")
	}
	if _, ok := app.Runner.Commands.Get("btw"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /btw command")
	}
	if _, ok := app.Runner.Commands.Get("chrome"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /chrome command")
	}
	if _, ok := app.Runner.Commands.Get("think-back"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /think-back command")
	}
	if _, ok := app.Runner.Commands.Get("thinkback-play"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /thinkback-play command")
	}
	if _, ok := app.Runner.Commands.Get("reload-plugins"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /reload-plugins command")
	}
	if _, ok := app.Runner.Commands.Get("advisor"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /advisor command")
	}
	if _, ok := app.Runner.Commands.Get("statusline"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /statusline command")
	}
	if _, ok := app.Runner.Commands.Get("ultrareview"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /ultrareview command")
	}
	if _, ok := app.Runner.Commands.Get("insights"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /insights command")
	}
	if _, ok := app.Runner.Commands.Get("remote-control"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /remote-control command")
	}
	if _, ok := app.Runner.Commands.Get("rc"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /rc alias")
	}
	if _, ok := app.Runner.Commands.Get("bridge-kick"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /bridge-kick command")
	}
	if _, ok := app.Runner.Commands.Get("commit"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /commit command")
	}
	if _, ok := app.Runner.Commands.Get("commit-push-pr"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /commit-push-pr command")
	}
	if _, ok := app.Runner.Commands.Get("heapdump"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /heapdump command")
	}
	if _, ok := app.Runner.Commands.Get("web-setup"); !ok {
		t.Fatal("NewAppWithDependencies() runner commands missing /web-setup command")
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
	}, runtimesession.NewBackgroundTaskStore(), nil)
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

func TestDefaultEngineFactoryMarksAnthropicClientFirstParty(t *testing.T) {
	eng, _, err := DefaultEngineFactory(coreconfig.Config{
		Provider:     coreconfig.ProviderAnthropic,
		Model:        "claude-sonnet-4-5",
		ApprovalMode: approval.ModeBypassPermissions,
	}, runtimesession.NewBackgroundTaskStore(), nil)
	if err != nil {
		t.Fatalf("DefaultEngineFactory() error = %v", err)
	}

	runtime, ok := eng.(*engine.Runtime)
	if !ok {
		t.Fatalf("DefaultEngineFactory() engine = %T, want *engine.Runtime", eng)
	}
	client, ok := runtime.Client.(*anthropic.Client)
	if !ok {
		t.Fatalf("DefaultEngineFactory() client = %T, want *anthropic.Client", runtime.Client)
	}
	if !reflect.ValueOf(client).Elem().FieldByName("isFirstParty").Bool() {
		t.Fatal("DefaultEngineFactory() anthropic client isFirstParty = false, want true")
	}
}

// TestDefaultEngineFactoryBuildsOpenAICompatibleRuntime verifies bootstrap can wire the OpenAI-compatible provider.
func TestDefaultEngineFactoryBuildsOpenAICompatibleRuntime(t *testing.T) {
	eng, _, err := DefaultEngineFactory(coreconfig.Config{
		Provider:     coreconfig.ProviderOpenAICompatible,
		Model:        "gpt-5",
		ApprovalMode: approval.ModeBypassPermissions,
	}, runtimesession.NewBackgroundTaskStore(), nil)
	if err != nil {
		t.Fatalf("DefaultEngineFactory() error = %v", err)
	}

	runtime, ok := eng.(*engine.Runtime)
	if !ok {
		t.Fatalf("DefaultEngineFactory() engine = %T, want *engine.Runtime", eng)
	}
	if _, ok := runtime.Client.(*openai.Client); !ok {
		t.Fatalf("DefaultEngineFactory() client = %T, want *openai.Client", runtime.Client)
	}
}

// TestDefaultEngineFactoryBuildsGLMRuntime verifies bootstrap routes the GLM alias through the OpenAI-compatible runtime.
func TestDefaultEngineFactoryBuildsGLMRuntime(t *testing.T) {
	eng, _, err := DefaultEngineFactory(coreconfig.Config{
		Provider:     coreconfig.ProviderGLM,
		Model:        "glm-4.5",
		ApprovalMode: approval.ModeBypassPermissions,
	}, runtimesession.NewBackgroundTaskStore(), nil)
	if err != nil {
		t.Fatalf("DefaultEngineFactory() error = %v", err)
	}

	runtime, ok := eng.(*engine.Runtime)
	if !ok {
		t.Fatalf("DefaultEngineFactory() engine = %T, want *engine.Runtime", eng)
	}
	if _, ok := runtime.Client.(*openai.Client); !ok {
		t.Fatalf("DefaultEngineFactory() client = %T, want *openai.Client", runtime.Client)
	}
}

// TestNewCommandRegistryRegistersResume verifies batch-12 bootstrap wiring exposes the minimum resume command through the registry.
func TestNewCommandRegistryRegistersResume(t *testing.T) {
	registry, err := newCommandRegistry(&coreconfig.Config{}, nil, nil, nil, nil, nil, runtimesession.NewBackgroundTaskStore(), nil, nil)
	if err != nil {
		t.Fatalf("newCommandRegistry() error = %v", err)
	}

	cmds := registry.List()
	if len(cmds) != 80 {
		t.Fatalf("newCommandRegistry() list len = %d, want 80", len(cmds))
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
		Usage:       "/mcp [enable|disable <server-name>] | /mcp detail <server-name>",
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
		Name:        "branch",
		Aliases:     []string{"fork"},
		Description: "Create a branch of the current conversation at this point",
		Usage:       "/branch [name]",
	}) {
		t.Fatalf("newCommandRegistry() twenty-first metadata = %#v, want branch metadata", got)
	}
	if got := cmds[21].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "voice",
		Description: "Toggle voice mode",
		Usage:       "/voice",
	}) {
		t.Fatalf("newCommandRegistry() twenty-second metadata = %#v, want voice metadata", got)
	}
	if got := cmds[22].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "ide",
		Description: "Manage IDE integrations and show status",
		Usage:       "/ide [open]",
	}) {
		t.Fatalf("newCommandRegistry() twenty-third metadata = %#v, want ide metadata", got)
	}
	if got := cmds[23].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "init",
		Description: "Initialize a new CLAUDE.md file with codebase documentation",
		Usage:       "/init",
	}) {
		t.Fatalf("newCommandRegistry() twenty-fourth metadata = %#v, want init metadata", got)
	}
	if got := cmds[24].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "install-github-app",
		Description: "Set up Claude GitHub Actions for a repository",
		Usage:       "/install-github-app",
	}) {
		t.Fatalf("newCommandRegistry() twenty-fifth metadata = %#v, want install-github-app metadata", got)
	}
	if got := cmds[25].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "install-slack-app",
		Description: "Install the Claude Slack app",
		Usage:       "/install-slack-app",
	}) {
		t.Fatalf("newCommandRegistry() twenty-sixth metadata = %#v, want install-slack-app metadata", got)
	}
	if got := cmds[26].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "remote-env",
		Description: "Configure the default remote environment for teleport sessions",
		Usage:       "/remote-env",
	}) {
		t.Fatalf("newCommandRegistry() twenty-seventh metadata = %#v, want remote-env metadata", got)
	}
	if got := cmds[27].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "desktop",
		Aliases:     []string{"app"},
		Description: "Continue the current session in Claude Desktop",
		Usage:       "/desktop",
	}) {
		t.Fatalf("newCommandRegistry() twenty-eighth metadata = %#v, want desktop metadata", got)
	}
	if got := cmds[28].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "mobile",
		Aliases:     []string{"ios", "android"},
		Description: "Show QR code to download the Claude mobile app",
		Usage:       "/mobile",
	}) {
		t.Fatalf("newCommandRegistry() twenty-ninth metadata = %#v, want mobile metadata", got)
	}
	if got := cmds[29].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "feedback",
		Aliases:     []string{"bug"},
		Description: "Submit feedback about Claude Code",
		Usage:       "/feedback [report]",
	}) {
		t.Fatalf("newCommandRegistry() thirtieth metadata = %#v, want feedback metadata", got)
	}
	if got := cmds[30].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "exit",
		Aliases:     []string{"quit"},
		Description: "Exit the REPL",
		Usage:       "/exit",
	}) {
		t.Fatalf("newCommandRegistry() thirty-first metadata = %#v, want exit metadata", got)
	}
	if got := cmds[31].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "install",
		Description: "Install Claude Code native build",
		Usage:       "/install [options]",
	}) {
		t.Fatalf("newCommandRegistry() thirty-second metadata = %#v, want install metadata", got)
	}
	if got := cmds[32].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "context",
		Description: "Show current context usage",
		Usage:       "/context",
	}) {
		t.Fatalf("newCommandRegistry() thirty-third metadata = %#v, want context metadata", got)
	}
	if got := cmds[33].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "review",
		Description: "Review a pull request",
		Usage:       "/review [pr-number]",
	}) {
		t.Fatalf("newCommandRegistry() thirty-fourth metadata = %#v, want review metadata", got)
	}
	if got := cmds[34].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "rewind",
		Aliases:     []string{"checkpoint"},
		Description: "Restore the code and/or conversation to a previous point",
		Usage:       "/rewind",
	}) {
		t.Fatalf("newCommandRegistry() thirty-fifth metadata = %#v, want rewind metadata", got)
	}
	if got := cmds[35].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "skills",
		Description: "List available skills",
		Usage:       "/skills",
	}) {
		t.Fatalf("newCommandRegistry() thirty-sixth metadata = %#v, want skills metadata", got)
	}
	if got := cmds[36].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "tag",
		Description: "Toggle a searchable tag on the current session",
		Usage:       "/tag <tag-name>",
	}) {
		t.Fatalf("newCommandRegistry() thirty-seventh metadata = %#v, want tag metadata", got)
	}
	if got := cmds[37].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "color",
		Description: "Set the prompt bar color for this session",
		Usage:       "/color <color|default>",
	}) {
		t.Fatalf("newCommandRegistry() thirty-eighth metadata = %#v, want color metadata", got)
	}
	if got := cmds[38].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "passes",
		Description: "Share a free week of Claude Code with friends",
		Usage:       "/passes",
	}) {
		t.Fatalf("newCommandRegistry() thirty-ninth metadata = %#v, want passes metadata", got)
	}
	if got := cmds[39].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "rate-limit-options",
		Description: "Show options when rate limit is reached",
		Usage:       "/rate-limit-options",
		Hidden:      true,
	}) {
		t.Fatalf("newCommandRegistry() fortieth metadata = %#v, want rate-limit-options metadata", got)
	}
	if got := cmds[40].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "sandbox",
		Description: "Configure sandbox settings",
		Usage:       "/sandbox [exclude <command-pattern>]",
	}) {
		t.Fatalf("newCommandRegistry() forty-first metadata = %#v, want sandbox metadata", got)
	}
	if got := cmds[41].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "stickers",
		Description: "Order Claude Code stickers",
		Usage:       "/stickers",
	}) {
		t.Fatalf("newCommandRegistry() forty-second metadata = %#v, want stickers metadata", got)
	}
	if got := cmds[42].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "privacy-settings",
		Description: "View and update your privacy settings",
		Usage:       "/privacy-settings",
	}) {
		t.Fatalf("newCommandRegistry() forty-third metadata = %#v, want privacy-settings metadata", got)
	}
	if got := cmds[43].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "plan",
		Description: "Enable plan mode or view the current session plan",
		Usage:       "/plan [open|<description>]",
	}) {
		t.Fatalf("newCommandRegistry() forty-fourth metadata = %#v, want plan metadata", got)
	}
	if got := cmds[44].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "tasks",
		Aliases:     []string{"bashes"},
		Description: "List and manage background tasks",
		Usage:       "/tasks",
	}) {
		t.Fatalf("newCommandRegistry() forty-fifth metadata = %#v, want tasks metadata", got)
	}
	if got := cmds[45].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "diff",
		Description: "View uncommitted changes and per-turn diffs",
		Usage:       "/diff",
	}) {
		t.Fatalf("newCommandRegistry() forty-sixth metadata = %#v, want diff metadata", got)
	}
	if got := cmds[46].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "files",
		Description: "List all files currently in context",
		Usage:       "/files",
	}) {
		t.Fatalf("newCommandRegistry() forty-seventh metadata = %#v, want files metadata", got)
	}
	if got := cmds[47].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "copy",
		Description: "Copy Claude's last response to clipboard (or /copy N for the Nth-latest)",
		Usage:       "/copy [N]",
	}) {
		t.Fatalf("newCommandRegistry() forty-eighth metadata = %#v, want copy metadata", got)
	}
	if got := cmds[48].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "export",
		Description: "Export the current conversation to a file or clipboard",
		Usage:       "/export [filename]",
	}) {
		t.Fatalf("newCommandRegistry() forty-ninth metadata = %#v, want export metadata", got)
	}
	if got := cmds[49].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "version",
		Description: "Print the version this session is running (not what autoupdate downloaded)",
		Usage:       "/version",
	}) {
		t.Fatalf("newCommandRegistry() fiftieth metadata = %#v, want version metadata", got)
	}
	if got := cmds[50].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "release-notes",
		Description: "View release notes",
		Usage:       "/release-notes",
	}) {
		t.Fatalf("newCommandRegistry() fifty-first metadata = %#v, want release-notes metadata", got)
	}
	if got := cmds[51].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "upgrade",
		Description: "Upgrade to Max for higher rate limits and more Opus",
		Usage:       "/upgrade",
	}) {
		t.Fatalf("newCommandRegistry() fifty-second metadata = %#v, want upgrade metadata", got)
	}
	if got := cmds[52].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "usage",
		Description: "Show plan usage limits",
		Usage:       "/usage",
	}) {
		t.Fatalf("newCommandRegistry() fifty-third metadata = %#v, want usage metadata", got)
	}
	if got := cmds[53].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "stats",
		Description: "Show your Claude Code usage statistics and activity",
		Usage:       "/stats",
	}) {
		t.Fatalf("newCommandRegistry() fifty-fourth metadata = %#v, want stats metadata", got)
	}
	if got := cmds[54].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "extra-usage",
		Description: "Configure extra usage to keep working when limits are hit",
		Usage:       "/extra-usage",
	}) {
		t.Fatalf("newCommandRegistry() fifty-fifth metadata = %#v, want extra-usage metadata", got)
	}
	if got := cmds[55].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "theme",
		Description: "Change the theme",
		Usage:       "/theme <auto|dark|light|light-daltonized|dark-daltonized|light-ansi|dark-ansi>",
	}) {
		t.Fatalf("newCommandRegistry() fifty-sixth metadata = %#v, want theme metadata", got)
	}
	if got := cmds[56].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "vim",
		Description: "Toggle between Vim and Normal editing modes",
		Usage:       "/vim",
	}) {
		t.Fatalf("newCommandRegistry() fifty-seventh metadata = %#v, want vim metadata", got)
	}
	if got := cmds[57].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "terminal-setup",
		Description: "Install Shift+Enter key binding for newlines",
		Usage:       "/terminal-setup",
	}) {
		t.Fatalf("newCommandRegistry() fifty-eighth metadata = %#v, want terminal-setup metadata", got)
	}
	if got := cmds[58].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "keybindings",
		Description: "Open or create your keybindings configuration file",
		Usage:       "/keybindings",
	}) {
		t.Fatalf("newCommandRegistry() fifty-ninth metadata = %#v, want keybindings metadata", got)
	}
	if got := cmds[59].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "pr-comments",
		Description: "Get comments from a GitHub pull request",
		Usage:       "/pr-comments",
	}) {
		t.Fatalf("newCommandRegistry() sixtieth metadata = %#v, want pr-comments metadata", got)
	}
	if got := cmds[60].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "security-review",
		Description: "Complete a security review of the pending changes on the current branch",
		Usage:       "/security-review",
	}) {
		t.Fatalf("newCommandRegistry() sixty-first metadata = %#v, want security-review metadata", got)
	}
	if got := cmds[61].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "agents",
		Description: "Manage agent configurations",
		Usage:       "/agents",
	}) {
		t.Fatalf("newCommandRegistry() sixty-second metadata = %#v, want agents metadata", got)
	}
	if got := cmds[62].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "plugin",
		Aliases:     []string{"plugins", "marketplace"},
		Description: "Manage Claude Code plugins",
		Usage:       "/plugin [subcommand]",
	}) {
		t.Fatalf("newCommandRegistry() sixty-third metadata = %#v, want plugin metadata", got)
	}
	if got := cmds[63].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "hooks",
		Description: "View hook configurations for tool events",
		Usage:       "/hooks",
	}) {
		t.Fatalf("newCommandRegistry() sixty-fourth metadata = %#v, want hooks metadata", got)
	}
	if got := cmds[64].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "btw",
		Description: "Ask a quick side question without interrupting the main conversation",
		Usage:       "/btw <question>",
	}) {
		t.Fatalf("newCommandRegistry() sixty-fifth metadata = %#v, want btw metadata", got)
	}
	if got := cmds[65].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "chrome",
		Description: "Claude in Chrome (Beta) settings",
		Usage:       "/chrome",
	}) {
		t.Fatalf("newCommandRegistry() sixty-sixth metadata = %#v, want chrome metadata", got)
	}
	if got := cmds[66].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "think-back",
		Description: "Your 2025 Claude Code Year in Review",
		Usage:       "/think-back",
	}) {
		t.Fatalf("newCommandRegistry() sixty-seventh metadata = %#v, want think-back metadata", got)
	}
	if got := cmds[67].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "thinkback-play",
		Description: "Play the thinkback animation",
		Usage:       "/thinkback-play",
		Hidden:      true,
	}) {
		t.Fatalf("newCommandRegistry() sixty-eighth metadata = %#v, want thinkback-play metadata", got)
	}
	if got := cmds[68].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "reload-plugins",
		Description: "Activate pending plugin changes in the current session",
		Usage:       "/reload-plugins",
	}) {
		t.Fatalf("newCommandRegistry() sixty-ninth metadata = %#v, want reload-plugins metadata", got)
	}
	if got := cmds[69].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "advisor",
		Description: "Configure the advisor model",
		Usage:       "/advisor [<model>|off]",
	}) {
		t.Fatalf("newCommandRegistry() seventieth metadata = %#v, want advisor metadata", got)
	}
	if got := cmds[70].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "statusline",
		Description: "Set up Claude Code's status line UI",
		Usage:       "/statusline [prompt]",
	}) {
		t.Fatalf("newCommandRegistry() seventy-first metadata = %#v, want statusline metadata", got)
	}
	if got := cmds[71].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "ultrareview",
		Description: "Find and verify bugs in your branch using Claude Code on the web",
		Usage:       "/ultrareview [pr-number]",
	}) {
		t.Fatalf("newCommandRegistry() seventy-second metadata = %#v, want ultrareview metadata", got)
	}
	if got := cmds[72].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "insights",
		Description: "Generate a report analyzing your Claude Code sessions",
		Usage:       "/insights [--homespaces]",
	}) {
		t.Fatalf("newCommandRegistry() seventy-third metadata = %#v, want insights metadata", got)
	}
	if got := cmds[73].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "remote-control",
		Aliases:     []string{"rc"},
		Description: "Connect this terminal for remote-control sessions",
		Usage:       "/remote-control [name]",
	}) {
		t.Fatalf("newCommandRegistry() seventy-fourth metadata = %#v, want remote-control metadata", got)
	}
	if got := cmds[74].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "bridge-kick",
		Description: "Inject bridge failure states for manual recovery testing",
		Usage:       "/bridge-kick <subcommand>",
		Hidden:      true,
	}) {
		t.Fatalf("newCommandRegistry() seventy-fifth metadata = %#v, want bridge-kick metadata", got)
	}
	if got := cmds[75].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "commit",
		Description: "Create a git commit",
		Usage:       "/commit",
	}) {
		t.Fatalf("newCommandRegistry() seventy-sixth metadata = %#v, want commit metadata", got)
	}
	if got := cmds[76].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "commit-push-pr",
		Description: "Commit, push, and open a PR",
		Usage:       "/commit-push-pr [instructions]",
	}) {
		t.Fatalf("newCommandRegistry() seventy-seventh metadata = %#v, want commit-push-pr metadata", got)
	}
	if got := cmds[77].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "heapdump",
		Description: "Dump the JS heap to ~/Desktop",
		Usage:       "/heapdump",
		Hidden:      true,
	}) {
		t.Fatalf("newCommandRegistry() seventy-eighth metadata = %#v, want heapdump metadata", got)
	}
	if got := cmds[78].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "web-setup",
		Description: "Setup Claude Code on the web (requires connecting your GitHub account)",
		Usage:       "/web-setup",
	}) {
		t.Fatalf("newCommandRegistry() seventy-ninth metadata = %#v, want web-setup metadata", got)
	}
	if got := cmds[79].Metadata(); !reflect.DeepEqual(got, command.Metadata{
		Name:        "seed-sessions",
		Description: "Insert demo persisted sessions for /resume testing",
		Usage:       "/seed-sessions",
	}) {
		t.Fatalf("newCommandRegistry() eightieth metadata = %#v, want seed-sessions metadata", got)
	}
}
