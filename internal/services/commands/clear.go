package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ClearCommand starts a fresh logical session so subsequent prompts do not continue previous history.
type ClearCommand struct{}

// Metadata returns the canonical slash descriptor for /clear.
func (c ClearCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "clear",
		Description: "Clear conversation history and start a new session",
		Usage:       "/clear",
	}
}

// Execute requests that the REPL switch to a fresh session identifier.
func (c ClearCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	newSessionID := uuid.NewString()
	logger.DebugCF("commands", "created fresh session for clear command", map[string]any{
		"session_id": newSessionID,
	})

	return command.Result{
		Output:       "Started a new session.",
		NewSessionID: newSessionID,
	}, nil
}
