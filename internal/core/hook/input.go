package hook

import "encoding/json"

// HookSource identifies which settings layer contributed a hook.
type HookSource string

const (
	// SourceUserSettings marks hooks from the user's global settings.
	SourceUserSettings HookSource = "userSettings"
	// SourceProjectSettings marks hooks from the project's shared settings.
	SourceProjectSettings HookSource = "projectSettings"
	// SourceLocalSettings marks hooks from the project's local settings.
	SourceLocalSettings HookSource = "localSettings"
	// SourcePolicySettings marks hooks from managed policy settings.
	SourcePolicySettings HookSource = "policySettings"
)

// BaseHookInput contains the common fields shared by all hook input schemas.
type BaseHookInput struct {
	// SessionID identifies the current conversation session.
	SessionID string `json:"session_id"`
	// TranscriptPath points at the session transcript file.
	TranscriptPath string `json:"transcript_path"`
	// CWD is the current working directory at hook execution time.
	CWD string `json:"cwd"`
	// PermissionMode is the active permission mode (e.g. "default", "plan").
	PermissionMode string `json:"permission_mode,omitempty"`
	// AgentID identifies the agent when running in a sub-agent context.
	AgentID string `json:"agent_id,omitempty"`
	// AgentType identifies the agent role (e.g. "general-purpose").
	AgentType string `json:"agent_type,omitempty"`
}

// StopHookInput is the JSON payload piped to Stop event hooks via stdin.
type StopHookInput struct {
	BaseHookInput
	// HookEventName is always "Stop".
	HookEventName string `json:"hook_event_name"`
	// StopHookActive indicates whether this is a re-entry from a prior blocking stop hook.
	StopHookActive bool `json:"stop_hook_active"`
	// LastAssistantMessage contains the text of the last assistant message, if any.
	LastAssistantMessage string `json:"last_assistant_message,omitempty"`
}

// SubagentStopHookInput is the JSON payload piped to SubagentStop event hooks.
type SubagentStopHookInput struct {
	BaseHookInput
	// HookEventName is always "SubagentStop".
	HookEventName string `json:"hook_event_name"`
	// StopHookActive indicates re-entry from a prior blocking stop hook.
	StopHookActive bool `json:"stop_hook_active"`
	// AgentTranscriptPath points at the sub-agent's transcript file.
	AgentTranscriptPath string `json:"agent_transcript_path"`
}

// SubagentStartHookInput is the JSON payload piped to SubagentStart event hooks via stdin.
// Hooks receive this when a sub-agent starts and can inject additional context into
// the sub-agent conversation.
type SubagentStartHookInput struct {
	BaseHookInput
	// HookEventName is always "SubagentStart".
	HookEventName string `json:"hook_event_name"`
	// AgentID identifies the sub-agent being started.
	AgentID string `json:"agent_id"`
	// AgentType is the type/name of the sub-agent (e.g. "general-purpose").
	AgentType string `json:"agent_type"`
}

// WorktreeCreateHookInput is the JSON payload piped to WorktreeCreate event hooks via stdin.
// Hooks receive this after a git worktree is created and can block the operation
// by returning exit code 2. In Go the worktree is created via git (not hook), so this
// is a post-creation notification hook.
type WorktreeCreateHookInput struct {
	BaseHookInput
	// HookEventName is always "WorktreeCreate".
	HookEventName string `json:"hook_event_name"`
	// Name is the worktree slug provided by the caller.
	Name string `json:"name"`
}

// WorktreeRemoveHookInput is the JSON payload piped to WorktreeRemove event hooks via stdin.
// Hooks receive this after a git worktree is removed and can observe the removal.
// This is a non-blocking notification hook (matching TS behavior where failed hooks only
// log errors and do not prevent cleanup).
type WorktreeRemoveHookInput struct {
	BaseHookInput
	// HookEventName is always "WorktreeRemove".
	HookEventName string `json:"hook_event_name"`
	// WorktreePath is the absolute path of the removed worktree.
	WorktreePath string `json:"worktree_path"`
}

// StopFailureHookInput is the JSON payload piped to StopFailure event hooks.
type StopFailureHookInput struct {
	BaseHookInput
	// HookEventName is always "StopFailure".
	HookEventName string `json:"hook_event_name"`
	// Error is the error category (e.g. "api_error").
	Error string `json:"error"`
	// ErrorDetails provides additional error context.
	ErrorDetails string `json:"error_details,omitempty"`
	// LastAssistantMessage contains the text of the last assistant message, if any.
	LastAssistantMessage string `json:"last_assistant_message,omitempty"`
}

// PreToolHookInput is the JSON payload piped to PreToolUse event hooks via stdin.
// Hooks receive this before the tool is executed and can block the execution
// by returning exit code 2.
type PreToolHookInput struct {
	BaseHookInput
	// HookEventName is always "PreToolUse".
	HookEventName string `json:"hook_event_name"`
	// ToolName is the name of the tool about to be executed.
	ToolName string `json:"tool_name"`
	// ToolInput contains the raw tool arguments as JSON.
	ToolInput json.RawMessage `json:"tool_input"`
	// ToolUseID is the unique identifier for this tool use call.
	ToolUseID string `json:"tool_use_id"`
}

// PostToolHookInput is the JSON payload piped to PostToolUse event hooks via stdin.
// Hooks receive this after the tool completes successfully and can signal
// a blocking error by returning exit code 2.
type PostToolHookInput struct {
	BaseHookInput
	// HookEventName is always "PostToolUse".
	HookEventName string `json:"hook_event_name"`
	// ToolName is the name of the tool that was executed.
	ToolName string `json:"tool_name"`
	// ToolInput contains the raw tool arguments as JSON.
	ToolInput json.RawMessage `json:"tool_input"`
	// ToolResponse contains the raw tool response output as JSON.
	ToolResponse json.RawMessage `json:"tool_response"`
	// ToolUseID is the unique identifier for this tool use call.
	ToolUseID string `json:"tool_use_id"`
}

// PostToolFailureHookInput is the JSON payload piped to PostToolUseFailure hooks via stdin.
// Hooks receive this after the tool completes with an error result or execution failure.
type PostToolFailureHookInput struct {
	BaseHookInput
	// HookEventName is always "PostToolUseFailure".
	HookEventName string `json:"hook_event_name"`
	// ToolName is the name of the tool that was executed.
	ToolName string `json:"tool_name"`
	// ToolInput contains the raw tool arguments as JSON.
	ToolInput json.RawMessage `json:"tool_input"`
	// ToolResponse contains the raw tool response output as JSON when available.
	ToolResponse json.RawMessage `json:"tool_response,omitempty"`
	// Error contains the tool error message surfaced to the hook.
	Error string `json:"error"`
	// IsInterrupt indicates whether the failure was caused by a user interrupt (AbortError).
	IsInterrupt bool `json:"is_interrupt,omitempty"`
	// ToolUseID is the unique identifier for this tool use call.
	ToolUseID string `json:"tool_use_id"`
}

// TaskHookInput is the shared JSON payload piped to TaskCreated and TaskCompleted
// event hooks via stdin. Both events use identical fields; only HookEventName differs.
type TaskHookInput struct {
	BaseHookInput
	// HookEventName is "TaskCreated" or "TaskCompleted".
	HookEventName string `json:"hook_event_name"`
	// TaskID is the unique identifier of the task.
	TaskID string `json:"task_id"`
	// TaskSubject is the task title.
	TaskSubject string `json:"task_subject"`
	// TaskDescription is the optional task description.
	TaskDescription string `json:"task_description,omitempty"`
	// TeammateName is the optional agent name that created or completed the task.
	TeammateName string `json:"teammate_name,omitempty"`
	// TeamName is the optional team name the agent belongs to.
	TeamName string `json:"team_name,omitempty"`
}

// PreCompactHookInput is the JSON payload piped to PreCompact event hooks via stdin.
// Hooks receive this before context compaction and can inspect or modify the trigger.
type PreCompactHookInput struct {
	BaseHookInput
	// HookEventName is always "PreCompact".
	HookEventName string `json:"hook_event_name"`
	// Trigger indicates whether the compaction was "manual" or "auto".
	Trigger string `json:"trigger"`
	// CustomInstructions contains any custom instructions for the compaction, or nil.
	CustomInstructions *string `json:"custom_instructions"`
}

// PostCompactHookInput is the JSON payload piped to PostCompact event hooks via stdin.
// Hooks receive this after context compaction completes.
type PostCompactHookInput struct {
	BaseHookInput
	// HookEventName is always "PostCompact".
	HookEventName string `json:"hook_event_name"`
	// Trigger indicates whether the compaction was "manual" or "auto".
	Trigger string `json:"trigger"`
	// CompactSummary contains the summary text produced by the compaction.
	CompactSummary string `json:"compact_summary"`
}

// NotificationHookInput is the JSON payload piped to Notification event hooks via stdin.
// Hooks receive this when a notification is emitted. This is a fire-and-forget event.
type NotificationHookInput struct {
	BaseHookInput
	// HookEventName is always "Notification".
	HookEventName string `json:"hook_event_name"`
	// Message is the notification body text.
	Message string `json:"message"`
	// Title is the optional notification title.
	Title string `json:"title,omitempty"`
	// NotificationType categorizes the notification (e.g. "elicitation_complete").
	NotificationType string `json:"notification_type"`
}

// UserPromptSubmitHookInput is the JSON payload piped to UserPromptSubmit event
// hooks via stdin. Hooks receive this when the user submits a prompt and can
// block the prompt by returning exit code 2; the resulting stderr is surfaced
// back to the user. Stdout JSON forms such as `continue:false` or hook-specific
// `additionalContext` are not yet consumed by this runtime.
type UserPromptSubmitHookInput struct {
	BaseHookInput
	// HookEventName is always "UserPromptSubmit".
	HookEventName string `json:"hook_event_name"`
	// Prompt is the user-submitted prompt text forwarded to the conversation.
	Prompt string `json:"prompt"`
}

// ElicitationHookInput is the JSON payload piped to Elicitation event hooks via stdin.
// Hooks receive this before the MCP server sees a response and can programmatically
// accept, decline, or cancel the elicitation.
type ElicitationHookInput struct {
	BaseHookInput
	// HookEventName is always "Elicitation".
	HookEventName string `json:"hook_event_name"`
	// MCPServerName identifies the server that issued the elicitation request.
	MCPServerName string `json:"mcp_server_name"`
	// Message is the text presented to the user by the server.
	Message string `json:"message"`
	// Mode identifies the elicitation mode: "form" or "url".
	Mode string `json:"mode,omitempty"`
	// URL carries the URL for URL-mode elicitations.
	URL string `json:"url,omitempty"`
	// ElicitationID identifies the elicitation when the server supplies one.
	ElicitationID string `json:"elicitation_id,omitempty"`
	// RequestedSchema carries the schema for form-mode elicitations.
	RequestedSchema map[string]any `json:"requested_schema,omitempty"`
}

// ElicitationResultHookInput is the JSON payload piped to ElicitationResult hooks via stdin.
// Hooks receive this after an elicitation has been resolved and can observe or override
// the response before it is sent back to the MCP server.
type ElicitationResultHookInput struct {
	BaseHookInput
	// HookEventName is always "ElicitationResult".
	HookEventName string `json:"hook_event_name"`
	// MCPServerName identifies the server that issued the elicitation request.
	MCPServerName string `json:"mcp_server_name"`
	// ElicitationID identifies the elicitation when the server supplies one.
	ElicitationID string `json:"elicitation_id,omitempty"`
	// Mode identifies the elicitation mode: "form" or "url".
	Mode string `json:"mode,omitempty"`
	// Action is the resolved elicitation action.
	Action string `json:"action"`
	// Content carries submitted form values when the action is "accept".
	Content map[string]any `json:"content,omitempty"`
}

// SessionStartHookInput is the JSON payload piped to SessionStart event hooks via stdin.
// Hooks receive this at session startup or resume.
type SessionStartHookInput struct {
	BaseHookInput
	// HookEventName is always "SessionStart".
	HookEventName string `json:"hook_event_name"`
	// Source indicates the startup trigger: "startup", "resume", "clear", or "compact".
	Source string `json:"source"`
	// Model is the model identifier in use, if available.
	Model string `json:"model,omitempty"`
}

// SessionEndHookInput is the JSON payload piped to SessionEnd event hooks via stdin.
// Hooks receive this when the session ends.
type SessionEndHookInput struct {
	BaseHookInput
	// HookEventName is always "SessionEnd".
	HookEventName string `json:"hook_event_name"`
	// Reason indicates why the session ended (e.g. "clear", "resume", "shutdown").
	Reason string `json:"reason"`
}

// HookResult captures the outcome of executing a single hook command.
type HookResult struct {
	// ExitCode is the process exit code.
	ExitCode int
	// Stdout contains the captured standard output.
	Stdout string
	// Stderr contains the captured standard error.
	Stderr string
	// TimedOut reports whether the hook exceeded its timeout.
	TimedOut bool
	// PreventContinuation reports whether the hook output requests the
	// conversation to stop (JSON stdout with "continue": false).
	PreventContinuation bool
	// ParsedOutput contains the structured JSON output parsed from stdout.
	// Nil when stdout is not valid JSON or does not start with '{'.
	ParsedOutput *HookOutput
}

// IsSuccess reports whether the hook exited with code 0.
func (r HookResult) IsSuccess() bool {
	return r.ExitCode == 0
}

// IsBlocking reports whether the hook returned exit code 2 (blocking error).
func (r HookResult) IsBlocking() bool {
	return r.ExitCode == 2
}

// IsError reports whether the hook returned a non-zero, non-2 exit code.
func (r HookResult) IsError() bool {
	return r.ExitCode != 0 && r.ExitCode != 2
}
