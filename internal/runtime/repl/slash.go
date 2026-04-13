package repl

import (
	"context"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	servicecommands "github.com/sheepzhao/claude-code-go/internal/services/commands"
)

// IsSlashCommand reports whether the input starts with a slash-prefixed command token.
func IsSlashCommand(input string) bool {
	return strings.HasPrefix(strings.TrimSpace(input), "/")
}

// resumeCommandAdapter keeps the existing /resume runtime path available through the shared command registry.
type resumeCommandAdapter struct {
	runner *Runner
}

// NewResumeCommandAdapter exposes the existing /resume flow through the shared command registry.
func NewResumeCommandAdapter(runner *Runner) command.Command {
	return resumeCommandAdapter{runner: runner}
}

// renameCommandAdapter keeps the existing `/rename` runtime path available through the shared command registry.
type renameCommandAdapter struct {
	runner *Runner
}

// addDirCommandAdapter keeps the interactive `/add-dir` runtime path available through the shared command registry.
type addDirCommandAdapter struct {
	runner *Runner
	cmd    servicecommands.AddDirCommand
}

// NewRenameCommandAdapter exposes the existing `/rename` flow through the shared command registry.
func NewRenameCommandAdapter(runner *Runner) command.Command {
	return renameCommandAdapter{runner: runner}
}

// NewAddDirCommandAdapter exposes the interactive `/add-dir` flow through the shared command registry.
func NewAddDirCommandAdapter(runner *Runner, cmd servicecommands.AddDirCommand) command.Command {
	return addDirCommandAdapter{runner: runner, cmd: cmd}
}

// Metadata describes the minimum slash descriptor exposed for /resume.
func (c resumeCommandAdapter) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "resume",
		Aliases:     []string{"continue"},
		Description: "Resume a saved session by search or continue it with a new prompt",
		Usage:       "/resume <search-term> | /resume <session-id> <prompt>",
	}
}

// Execute forwards /resume handling back into the existing runner recovery flow.
func (c resumeCommandAdapter) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	return command.Result{}, c.runner.runResumeCommand(ctx, args.RawLine, args.Flags["fork_session"] == "true")
}

// Metadata describes the minimum slash descriptor exposed for `/rename`.
func (c renameCommandAdapter) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "rename",
		Description: "Rename the current conversation for easier resume discovery",
		Usage:       "/rename <title>",
	}
}

// Execute forwards `/rename` handling back into the runner session-title flow.
func (c renameCommandAdapter) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	return command.Result{}, c.runner.runRenameCommand(ctx, args.RawLine)
}

// Metadata describes the minimum slash descriptor exposed for `/add-dir`.
func (c addDirCommandAdapter) Metadata() command.Metadata {
	return c.cmd.Metadata()
}

// Execute forwards `/add-dir` handling back into the runner text interaction flow.
func (c addDirCommandAdapter) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	return command.Result{}, c.runner.runAddDirCommand(ctx, c.cmd, args.RawLine)
}
