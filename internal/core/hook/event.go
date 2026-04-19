package hook

// HookEvent identifies one of the supported hook lifecycle events.
// The full set mirrors the TypeScript HOOK_EVENTS const; only a subset
// is actively used by the current migration pass.
type HookEvent string

const (
	// EventPreToolUse fires before a tool is executed.
	EventPreToolUse HookEvent = "PreToolUse"
	// EventPostToolUse fires after a tool completes successfully.
	EventPostToolUse HookEvent = "PostToolUse"
	// EventPostToolUseFailure fires after a tool fails.
	EventPostToolUseFailure HookEvent = "PostToolUseFailure"
	// EventNotification fires when a notification is emitted.
	EventNotification HookEvent = "Notification"
	// EventUserPromptSubmit fires when the user submits a prompt.
	EventUserPromptSubmit HookEvent = "UserPromptSubmit"
	// EventSessionStart fires at session startup.
	EventSessionStart HookEvent = "SessionStart"
	// EventSessionEnd fires at session end.
	EventSessionEnd HookEvent = "SessionEnd"
	// EventStop fires when the conversation loop ends normally.
	EventStop HookEvent = "Stop"
	// EventStopFailure fires when the conversation loop ends with an error.
	EventStopFailure HookEvent = "StopFailure"
	// EventSubagentStart fires when a sub-agent starts.
	EventSubagentStart HookEvent = "SubagentStart"
	// EventSubagentStop fires when a sub-agent stops.
	EventSubagentStop HookEvent = "SubagentStop"
	// EventPreCompact fires before context compaction.
	EventPreCompact HookEvent = "PreCompact"
	// EventPostCompact fires after context compaction.
	EventPostCompact HookEvent = "PostCompact"
	// EventPermissionRequest fires when a permission decision is needed.
	EventPermissionRequest HookEvent = "PermissionRequest"
	// EventPermissionDenied fires when a permission is denied.
	EventPermissionDenied HookEvent = "PermissionDenied"
	// EventSetup fires during setup/maintenance triggers.
	EventSetup HookEvent = "Setup"
	// EventTeammateIdle fires when a teammate goes idle.
	EventTeammateIdle HookEvent = "TeammateIdle"
	// EventTaskCreated fires when a task is created.
	EventTaskCreated HookEvent = "TaskCreated"
	// EventTaskCompleted fires when a task is completed.
	EventTaskCompleted HookEvent = "TaskCompleted"
	// EventElicitation fires for MCP elicitation requests.
	EventElicitation HookEvent = "Elicitation"
	// EventElicitationResult fires for MCP elicitation responses.
	EventElicitationResult HookEvent = "ElicitationResult"
	// EventConfigChange fires when a settings file changes.
	EventConfigChange HookEvent = "ConfigChange"
	// EventWorktreeCreate fires when a worktree is created.
	EventWorktreeCreate HookEvent = "WorktreeCreate"
	// EventWorktreeRemove fires when a worktree is removed.
	EventWorktreeRemove HookEvent = "WorktreeRemove"
	// EventInstructionsLoaded fires when instruction files are loaded.
	EventInstructionsLoaded HookEvent = "InstructionsLoaded"
	// EventCwdChanged fires when the working directory changes.
	EventCwdChanged HookEvent = "CwdChanged"
	// EventFileChanged fires when a watched file changes.
	EventFileChanged HookEvent = "FileChanged"
)

// AllEvents lists every supported hook event in stable order.
func AllEvents() []HookEvent {
	return []HookEvent{
		EventPreToolUse,
		EventPostToolUse,
		EventPostToolUseFailure,
		EventNotification,
		EventUserPromptSubmit,
		EventSessionStart,
		EventSessionEnd,
		EventStop,
		EventStopFailure,
		EventSubagentStart,
		EventSubagentStop,
		EventPreCompact,
		EventPostCompact,
		EventPermissionRequest,
		EventPermissionDenied,
		EventSetup,
		EventTeammateIdle,
		EventTaskCreated,
		EventTaskCompleted,
		EventElicitation,
		EventElicitationResult,
		EventConfigChange,
		EventWorktreeCreate,
		EventWorktreeRemove,
		EventInstructionsLoaded,
		EventCwdChanged,
		EventFileChanged,
	}
}

// IsValid reports whether a string is a recognized hook event name.
func (e HookEvent) IsValid() bool {
	switch e {
	case EventPreToolUse, EventPostToolUse, EventPostToolUseFailure,
		EventNotification, EventUserPromptSubmit,
		EventSessionStart, EventSessionEnd,
		EventStop, EventStopFailure,
		EventSubagentStart, EventSubagentStop,
		EventPreCompact, EventPostCompact,
		EventPermissionRequest, EventPermissionDenied,
		EventSetup,
		EventTeammateIdle,
		EventTaskCreated, EventTaskCompleted,
		EventElicitation, EventElicitationResult,
		EventConfigChange,
		EventWorktreeCreate, EventWorktreeRemove,
		EventInstructionsLoaded,
		EventCwdChanged, EventFileChanged:
		return true
	default:
		return false
	}
}
