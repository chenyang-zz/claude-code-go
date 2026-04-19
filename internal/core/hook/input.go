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
