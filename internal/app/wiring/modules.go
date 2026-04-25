package wiring

import (
	"context"
	"time"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretask "github.com/sheepzhao/claude-code-go/internal/core/task"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	platformshell "github.com/sheepzhao/claude-code-go/internal/platform/shell"
	runtimehooks "github.com/sheepzhao/claude-code-go/internal/runtime/hooks"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/bash"
	fileedit "github.com/sheepzhao/claude-code-go/internal/services/tools/file_edit"
	fileread "github.com/sheepzhao/claude-code-go/internal/services/tools/file_read"
	filewrite "github.com/sheepzhao/claude-code-go/internal/services/tools/file_write"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/glob"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/grep"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/task_create"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/task_get"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/task_list"
	taskstop "github.com/sheepzhao/claude-code-go/internal/services/tools/task_stop"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/task_update"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/web_fetch"
)

// Modules aggregates host-level runtime dependencies assembled during startup.
type Modules struct {
	// Tools is the registry exposed to executors for tool lookup and dispatch.
	Tools tool.Registry
}

// NewModules wires the provided tools into the default in-memory registry.
func NewModules(tools ...tool.Tool) (Modules, error) {
	registry := tool.NewMemoryRegistry()
	for _, item := range tools {
		if err := registry.Register(item); err != nil {
			return Modules{}, err
		}
	}

	return Modules{
		Tools: registry,
	}, nil
}

// hookDispatcher adapts a hooks.Runner + HooksConfig into the task tools' HookDispatcher interface.
type hookDispatcher struct {
	runner *runtimehooks.Runner
	config hook.HooksConfig
}

func (d *hookDispatcher) RunHooks(ctx context.Context, event hook.HookEvent, input any, cwd string) []hook.HookResult {
	if d.runner == nil || d.config == nil {
		return nil
	}
	return d.runner.RunHooksForEvent(ctx, d.config, event, input, cwd)
}

// bashNotificationEmitter adapts the hook runner into the BashTool NotificationEmitter
// interface so background task completion events can fire Notification hooks.
type bashNotificationEmitter struct {
	runner *runtimehooks.Runner
	config hook.HooksConfig
}

func (e *bashNotificationEmitter) EmitTaskNotification(taskID string, status string, summary string, outputPath string) {
	if e.runner == nil || e.config == nil {
		return
	}
	msg := summary
	if outputPath != "" {
		msg += "\nOutput: " + outputPath
	}
	input := hook.NotificationHookInput{
		BaseHookInput: hook.BaseHookInput{},
		HookEventName:    string(hook.EventNotification),
		Message:          msg,
		Title:            "Background Bash Task",
		NotificationType: "bash_task_" + status,
	}
	// Fire-and-forget so the background result consumer never blocks on hooks.
	go e.runner.RunHooksForEvent(context.Background(), e.config, hook.EventNotification, input, "")
}

// NewBaseWorkspaceModules wires the base workspace exploration and editing tools into one registry.
func NewBaseWorkspaceModules(fs platformfs.FileSystem, policy *corepermission.FilesystemPolicy, permissions coreconfig.PermissionConfig, backgroundTaskStore *runtimesession.BackgroundTaskStore, taskStore coretask.Store) (Modules, error) {
	return NewModules(BaseWorkspaceTools(fs, policy, permissions, backgroundTaskStore, taskStore)...)
}

// NewBaseWorkspaceModulesWithHooks wires the base tools with hook dispatch capability.
func NewBaseWorkspaceModulesWithHooks(fs platformfs.FileSystem, policy *corepermission.FilesystemPolicy, permissions coreconfig.PermissionConfig, backgroundTaskStore *runtimesession.BackgroundTaskStore, taskStore coretask.Store, hookRunner *runtimehooks.Runner, hookCfg hook.HooksConfig, disableAllHooks bool) (Modules, error) {
	return NewModules(BaseWorkspaceToolsWithHooks(fs, policy, permissions, backgroundTaskStore, taskStore, hookRunner, hookCfg, disableAllHooks)...)
}

// webFetchCache is the process-level shared cache for WebFetch results.
var webFetchCache = web_fetch.NewCache(50*1024*1024, 15*time.Minute)

// BaseWorkspaceTools returns the canonical registration list for the base workspace toolset.
func BaseWorkspaceTools(fs platformfs.FileSystem, policy *corepermission.FilesystemPolicy, permissions coreconfig.PermissionConfig, backgroundTaskStore *runtimesession.BackgroundTaskStore, taskStore coretask.Store) []tool.Tool {
	executor := platformshell.NewExecutor()
	return []tool.Tool{
		bash.NewToolWithRuntime(executor, platformshell.NewPermissionChecker(permissions), permissions.DefaultMode, backgroundTaskStore),
		taskstop.NewTool(backgroundTaskStore),
		glob.NewTool(fs, policy),
		grep.NewTool(fs, policy),
		fileread.NewTool(fs, policy),
		filewrite.NewTool(fs, policy),
		fileedit.NewTool(fs, policy),
		task_create.NewTool(taskStore),
		task_get.NewTool(taskStore),
		task_list.NewTool(taskStore),
		task_update.NewTool(taskStore),
		web_fetch.NewTool(webFetchCache, permissions.Allow, permissions.Deny, permissions.Ask),
	}
}

// BaseWorkspaceToolsWithHooks returns the base toolset with hook dispatch injected into task tools.
func BaseWorkspaceToolsWithHooks(fs platformfs.FileSystem, policy *corepermission.FilesystemPolicy, permissions coreconfig.PermissionConfig, backgroundTaskStore *runtimesession.BackgroundTaskStore, taskStore coretask.Store, hookRunner *runtimehooks.Runner, hookCfg hook.HooksConfig, disableAllHooks bool) []tool.Tool {
	dispatcher := &hookDispatcher{runner: hookRunner, config: hookCfg}
	executor := platformshell.NewExecutor()
	bashNotifier := &bashNotificationEmitter{runner: hookRunner, config: hookCfg}
	return []tool.Tool{
		bash.NewToolWithNotification(executor, platformshell.NewPermissionChecker(permissions), permissions.DefaultMode, backgroundTaskStore, bashNotifier),
		taskstop.NewTool(backgroundTaskStore),
		glob.NewTool(fs, policy),
		grep.NewTool(fs, policy),
		fileread.NewTool(fs, policy),
		filewrite.NewTool(fs, policy),
		fileedit.NewTool(fs, policy),
		task_create.NewToolWithHooks(taskStore, dispatcher, hookCfg, disableAllHooks),
		task_get.NewTool(taskStore),
		task_list.NewTool(taskStore),
		task_update.NewToolWithHooks(taskStore, dispatcher, hookCfg, disableAllHooks),
		web_fetch.NewTool(webFetchCache, permissions.Allow, permissions.Deny, permissions.Ask),
	}
}
