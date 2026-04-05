package repl

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
	"github.com/sheepzhao/claude-code-go/internal/ui/console"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Runner coordinates one CLI turn between parsed input, engine execution and console rendering.
type Runner struct {
	// Engine handles normal prompt execution.
	Engine engine.Engine
	// Renderer handles console output for both engine events and slash placeholders.
	Renderer *console.StreamRenderer
	// SessionID identifies the current logical CLI session.
	SessionID string
	// SessionManager restores previously persisted conversation history when available.
	SessionManager *runtimesession.Manager
	// AutoSave persists the final normalized history after each successful turn.
	AutoSave *runtimesession.AutoSave
}

// NewRunner builds a runner from explicit dependencies.
func NewRunner(eng engine.Engine, renderer *console.StreamRenderer) *Runner {
	return &Runner{
		Engine:    eng,
		Renderer:  renderer,
		SessionID: uuid.NewString(),
	}
}

// Run parses the CLI args and dispatches either a slash placeholder or one text turn.
func (r *Runner) Run(ctx context.Context, args []string) error {
	parsed, err := ParseArgs(args)
	if err != nil {
		return err
	}

	logger.DebugCF("repl", "parsed cli input", map[string]any{
		"is_slash_command": parsed.IsSlashCommand,
		"command":          parsed.Command,
	})

	if parsed.IsSlashCommand {
		return r.Renderer.RenderLine(fmt.Sprintf("Slash command /%s is not supported yet.", parsed.Command))
	}

	history, err := r.restoreHistory(ctx)
	if err != nil {
		return err
	}

	requestHistory := history.Clone()
	requestHistory.Append(message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart(parsed.Body),
		},
	})

	stream, err := r.Engine.Run(ctx, conversation.RunRequest{
		SessionID: r.sessionID(),
		Messages:  requestHistory.Messages,
	})
	if err != nil {
		return err
	}

	finalHistory, err := r.renderAndCaptureHistory(stream)
	if err != nil {
		return err
	}
	if r.AutoSave != nil {
		if _, err := r.AutoSave.PersistHistory(ctx, r.sessionID(), finalHistory); err != nil {
			return err
		}
	}
	return nil
}

// restoreHistory loads the current session history when a session manager is configured.
func (r *Runner) restoreHistory(ctx context.Context) (conversation.History, error) {
	if r == nil || r.SessionManager == nil {
		return conversation.History{}, nil
	}

	snapshot, err := r.SessionManager.Start(ctx, r.sessionID())
	if err != nil {
		return conversation.History{}, err
	}

	logger.DebugCF("repl", "restored session history for turn", map[string]any{
		"session_id":    r.sessionID(),
		"message_count": len(snapshot.Session.Messages),
		"resumed":       snapshot.Resumed,
	})
	return conversation.History{Messages: snapshot.Session.Messages}, nil
}

// renderAndCaptureHistory renders visible runtime events and captures the final history snapshot emitted by the engine.
func (r *Runner) renderAndCaptureHistory(stream event.Stream) (conversation.History, error) {
	var finalHistory conversation.History
	haveFinalHistory := false

	for evt := range stream {
		if evt.Type == event.TypeConversationDone {
			payload, ok := evt.Payload.(event.ConversationDonePayload)
			if !ok {
				return conversation.History{}, fmt.Errorf("conversation done payload type mismatch")
			}
			finalHistory = payload.History.Clone()
			haveFinalHistory = true
			continue
		}

		if err := r.Renderer.RenderEvent(evt); err != nil {
			return conversation.History{}, err
		}
	}

	if !haveFinalHistory {
		return conversation.History{}, fmt.Errorf("runtime did not emit final conversation history")
	}
	return finalHistory, nil
}

// sessionID returns the stable runner session identifier, defaulting to a generated UUID when unset.
func (r *Runner) sessionID() string {
	if r != nil && r.SessionID != "" {
		return r.SessionID
	}
	return uuid.NewString()
}
