// Package analytics provides the core event infrastructure for analytics
// event logging. It defines event types, a Sink interface, a non-blocking
// EventQueue, and a ConsoleSink for log-based output.
//
// This is the first slice of the analytics pipeline — future slices will add
// Datadog HTTP reporting, 1P OpenTelemetry logging, GrowthBook sampling, and
// full metadata enrichment. This slice establishes the foundation: event types,
// Sink interface, event queue, and log-based output.
package analytics

import "time"

// Metadata carries common contextual data attached to every analytics event.
// Labels provides a type-safe key-value bag (bool, int64, float64, string).
type Metadata struct {
	Timestamp time.Time
	SessionID string
	Labels    map[string]any
}

// Event is the generic envelope for analytics events. Name identifies the
// event type (e.g. "tool.used", "session.started"). Payload carries the
// type-specific data; Components that need to inspect it should type-assert.
type Event struct {
	Name      string
	Metadata  Metadata
	Payload   any
}

// ToolUsedEvent is emitted when a tool finishes execution.
type ToolUsedEvent struct {
	ToolName    string
	Duration    time.Duration
	Success     bool
	ErrorMsg    string // empty on success
	InputSize   int    // approximate character count of tool input
	OutputSize  int    // approximate character count of tool output
}

// SessionEvent is emitted on session start / end.
type SessionEvent struct {
	Action      string // "started" | "ended"
	TurnCount   int
	MessageCount int
	Duration    time.Duration // session wall-clock duration
}

// CommandEvent is emitted when a slash command finishes.
type CommandEvent struct {
	CommandName string
	Duration    time.Duration
	Success     bool
	Args        []string
}

// ErrorEvent is emitted when an API or tool error occurs.
type ErrorEvent struct {
	Category   string // "api" | "tool" | "internal"
	ErrorType  string
	ToolName   string // empty for non-tool errors
	DurationMs int64
}

// BashCommandEvent is emitted when a BashTool command finishes execution.
// Provides tool-specific command metadata beyond the generic ToolUsedEvent.
type BashCommandEvent struct {
	CommandSnippet string `json:"command_snippet"` // truncated command text
	ExitCode       int    `json:"exit_code"`
	Duration       time.Duration
	OutputSize     int `json:"output_size"`
}

// FileEditEvent is emitted when a FileEditTool operation completes.
type FileEditEvent struct {
	EditType string `json:"edit_type"` // "replace" | "insert" | "delete"
	FilePath string `json:"file_path"`
	Success  bool   `json:"success"`
	Duration time.Duration
}

// Event name constants used in Event.Name.
const (
	EventToolUsed    = "tool.used"
	EventToolErrored = "tool.errored"
	EventSession     = "session"
	EventCommand     = "command"
	EventError       = "error"
	EventBashCommand = "bash.command.executed"
	EventFileEdit    = "file_edit.applied"
)
