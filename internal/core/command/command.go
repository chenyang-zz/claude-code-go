package command

import "context"

// Metadata describes one slash command exposed to the REPL and help surfaces.
type Metadata struct {
	// Name is the canonical slash command name without the leading slash.
	Name string
	// Aliases exposes additional slash names that should resolve to the same command.
	Aliases []string
	// Description summarizes the command's visible behavior.
	Description string
	// Usage documents the minimum stable invocation form.
	Usage string
	// Hidden suppresses the command from /help while keeping it executable via direct slash invocation.
	Hidden bool
}

// Result captures the minimum side effects a slash command can request from the REPL.
type Result struct {
	// Output prints a stable user-facing line when non-empty.
	Output string
	// NewSessionID asks the REPL to switch subsequent turns onto a fresh session.
	NewSessionID string
}

// Command describes one executable slash command implementation.
type Command interface {
	// Metadata returns the stable command descriptor used by registry lookups and help output.
	Metadata() Metadata
	// Execute runs the command against parsed arguments and returns the visible result.
	Execute(ctx context.Context, args Args) (Result, error)
}
