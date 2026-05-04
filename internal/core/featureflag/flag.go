package featureflag

import (
	"os"
	"strings"
)

// Well-known feature flag names. Each flag is controlled by the environment
// variable CLAUDE_FEATURE_{NAME} (e.g. CLAUDE_FEATURE_TOKEN_BUDGET).
const (
	// FlagTokenBudget gates the +500k token budget input parsing and
	// automatic budget continuation in the engine loop.
	FlagTokenBudget = "TOKEN_BUDGET"
	// FlagTodoV2 gates the TodoV2 task tools (TaskCreate, TaskGet, TaskList,
	// TaskUpdate, TaskClaim, TaskReset). When disabled, the task toolset is
	// hidden from the provider tool catalog.
	FlagTodoV2 = "TODO_V2"
	// FlagVerificationAgent gates the Verification Nudge feature in TaskUpdateTool.
	// When enabled, completing the last task in a 3+ item list triggers a nudge
	// to spawn a verification agent if no verification step was present.
	FlagVerificationAgent = "VERIFICATION_AGENT"
	// FlagBuiltinExplorePlanAgents gates the Explore and Plan built-in agents.
	// When enabled, these agents are registered in the built-in agent registry.
	FlagBuiltinExplorePlanAgents = "BUILTIN_EXPLORE_PLAN_AGENTS"
	// FlagCoordinatorMode gates the coordinator mode agents.
	// When enabled, coordinator-specific agents are registered.
	FlagCoordinatorMode = "COORDINATOR_MODE"
	// FlagExtractMemories gates the extractMemories background extraction.
	// When enabled, a forked subagent analyzes conversation history after each
	// complete turn and writes durable memories to the auto-memory directory.
	FlagExtractMemories = "EXTRACT_MEMORIES"
	// FlagAutoDream gates the autoDream background memory consolidation.
	// When enabled, a forked subagent periodically consolidates, deduplicates,
	// and prunes memory files in the auto-memory directory.
	FlagAutoDream = "AUTO_DREAM"
	// FlagPromptSuggestion gates the prompt suggestion feature.
	// When enabled, a forked subagent generates prompt suggestions during conversation.
	FlagPromptSuggestion = "PROMPT_SUGGESTION"
	// FlagSpeculation gates the speculation feature.
	// When enabled, speculative generation is used to improve response latency.
	FlagSpeculation = "SPECULATION"
	// FlagSpinnerTips gates the spinner tips feature.
	// When enabled, contextual usage tips are shown during the spinner wait state.
	FlagSpinnerTips = "SPINNER_TIPS"
	// FlagClaudeAILimits gates the Claude.ai rate limit observation system.
	// When enabled, the Anthropic client consumes ratelimit response headers
	// and persists the latest ClaudeAILimits snapshot for engine error
	// messages and `/usage` `/stats` rendering.
	FlagClaudeAILimits = "CLAUDEAI_LIMITS"
	// FlagPolicyLimits gates the organizational policy limits system.
	// When enabled, the runtime checks admin-configurable feature restrictions
	// for Team and Enterprise subscribers.
	FlagPolicyLimits = "POLICY_LIMITS"
	// FlagSettingsSyncPush gates the settings sync upload (push) path.
	// When enabled, interactive CLI sessions upload changed settings files
	// to the Anthropic backend in the background.
	FlagSettingsSyncPush = "SETTINGS_SYNC_PUSH"
	// FlagSettingsSyncPull gates the settings sync download (pull) path.
	// When enabled, /reload-plugins downloads the latest settings from the
	// Anthropic backend before refreshing plugins.
	FlagSettingsSyncPull = "SETTINGS_SYNC_PULL"
	// FlagTeamMemorySync gates the Team Memory Sync system.
	// When enabled, team memory files are synchronized between the local
	// filesystem and the Claude.ai backend API.
	FlagTeamMemorySync = "TEAM_MEMORY_SYNC"
	// FlagTeamMemoryScanner gates secret scanning of team memory files before push.
	// When enabled, team memory content is scanned against gitleaks rules
	// and files containing detected secrets are excluded from upload.
	FlagTeamMemoryScanner = "TEAM_MEMORY_SCANNER"
	// FlagTeamMemoryWatcher gates the team memory file watcher.
	// When enabled, the team memory directory is watched for file changes
	// via fsnotify and changes trigger debounced push operations.
	FlagTeamMemoryWatcher = "TEAM_MEMORY_WATCHER"
	// FlagAwaySummary gates the away summary ("while you were away") feature.
	// When enabled, a small/fast model generates a brief session recap after
	// the user has been idle for a configurable threshold (default 5 minutes).
	FlagAwaySummary = "AWAY_SUMMARY"
	// FlagNotifier gates the terminal notification dispatch service.
	// When enabled, the bootstrap layer constructs a notifier service that
	// fans Notification hook events out to the configured terminal channel
	// (iTerm2 / Kitty / Ghostty / terminal bell / auto-detect / disabled).
	FlagNotifier = "NOTIFIER"
	// FlagPreventSleep gates the macOS sleep-prevention service. When
	// enabled, the bootstrap layer initialises the prevent-sleep registry
	// (caffeinate subprocess + restart loop). The service is a no-op on
	// non-darwin platforms regardless of the flag value.
	FlagPreventSleep = "PREVENT_SLEEP"
	// FlagInternalLogging gates the Ant-internal diagnostic logging path
	// (Kubernetes namespace + OCI container ID extraction). Replaces the
	// TS-side esbuild USER_TYPE=ant build define with a runtime feature
	// flag so non-Ant deployments incur zero filesystem reads.
	FlagInternalLogging = "INTERNAL_LOGGING"
	// FlagHaikuQuery names the env-variable suffix gating the Haiku
	// single-prompt query helper (services/haiku). Unlike most flags here,
	// the helper applies a reverse-default reading on this name: it is on
	// by default and only disabled when CLAUDE_FEATURE_HAIKU is set to "0"
	// or "false". The constant is kept here so the haiku package and the
	// generic IsEnabled reader share a single source of truth for the name.
	FlagHaikuQuery = "HAIKU"
	// FlagToolUseSummary gates the tool use summary helper
	// (services/toolusesummary). Generates a ~30-character single-line
	// label describing what a completed tool batch accomplished, used by
	// the SDK to surface progress. Off by default; set
	// CLAUDE_FEATURE_TOOL_USE_SUMMARY=1 to enable.
	FlagToolUseSummary = "TOOL_USE_SUMMARY"
)

// envPrefix is the environment variable prefix used for all feature flags.
const envPrefix = "CLAUDE_FEATURE_"

// IsEnabled reports whether the named feature flag is enabled.
// A flag is enabled when the environment variable CLAUDE_FEATURE_{NAME}
// is set to exactly "1". All other values (including unset) mean disabled.
func IsEnabled(name string) bool {
	return os.Getenv(envPrefix+name) == "1"
}

// IsTodoV2Enabled reports whether the TodoV2 task tools should be exposed.
// It checks the TS-compatible CLAUDE_CODE_ENABLE_TASKS variable (any truthy
// value) and the Go-native CLAUDE_FEATURE_TODO_V2 flag.
//
// When neither variable is set, the default is true because Go's
// claude-code-go is currently REPL/CLI-first (inherently interactive).
// When non-interactive SDK mode is supported, this default should become
// conditional on the session type.
func IsTodoV2Enabled() bool {
	if isEnvTruthy("CLAUDE_CODE_ENABLE_TASKS") {
		return true
	}
	if IsEnabled(FlagTodoV2) {
		return true
	}
	// Default to enabled for the current REPL/CLI-first runtime.
	return true
}

// isEnvTruthy returns true if the environment variable is set to a truthy
// value (1, true, yes). Case-insensitive.
func isEnvTruthy(key string) bool {
	val := strings.ToLower(os.Getenv(key))
	return val == "1" || val == "true" || val == "yes"
}
