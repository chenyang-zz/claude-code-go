package repl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
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
	// ProjectPath identifies the current workspace used for project-scoped session recovery.
	ProjectPath string
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
		Engine:   eng,
		Renderer: renderer,
	}
}

const (
	resumeUsageMessage         = "Resume command requires a session id and prompt: use /resume <session-id> <prompt>."
	resumeNotConfiguredMessage = "Resume command is not available because session storage is not configured."
)

// Run parses the CLI args and dispatches either a supported slash command or one text turn.
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
		if parsed.Command == "resume" {
			return r.runResumeCommand(ctx, parsed.Body)
		}
		return r.Renderer.RenderLine(fmt.Sprintf("Slash command /%s is not supported yet.", parsed.Command))
	}

	history, err := r.restoreHistory(ctx)
	if err != nil {
		return err
	}
	return r.runPrompt(ctx, history, parsed.Body)
}

// runResumeCommand restores one persisted session and immediately continues it with the provided prompt tail.
func (r *Runner) runResumeCommand(ctx context.Context, body string) error {
	sessionID, prompt, err := parseResumeBody(body)
	if err != nil {
		return r.Renderer.RenderLine(err.Error())
	}
	if r == nil || r.SessionManager == nil {
		return r.Renderer.RenderLine(resumeNotConfiguredMessage)
	}

	snapshot, err := r.SessionManager.Resume(ctx, sessionID)
	if err != nil {
		if errors.Is(err, coresession.ErrSessionNotFound) {
			return r.Renderer.RenderLine(fmt.Sprintf("Session %s was not found.", sessionID))
		}
		return err
	}

	r.SessionID = sessionID
	if snapshot.Session.ProjectPath != "" {
		r.ProjectPath = snapshot.Session.ProjectPath
	}
	logger.DebugCF("repl", "resumed session from slash command", map[string]any{
		"session_id":    sessionID,
		"project_path":  r.ProjectPath,
		"message_count": len(snapshot.Session.Messages),
	})
	return r.runPrompt(ctx, conversation.History{Messages: snapshot.Session.Messages}, prompt)
}

// runPrompt appends one user prompt onto the supplied history, executes the engine and persists the final history.
func (r *Runner) runPrompt(ctx context.Context, history conversation.History, prompt string) error {
	requestHistory := history.Clone()
	requestHistory.Append(message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart(prompt),
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
		if _, err := r.AutoSave.PersistHistoryInProject(ctx, r.sessionID(), r.ProjectPath, finalHistory); err != nil {
			return err
		}
	}
	return nil
}

// parseResumeBody splits the /resume tail into one session identifier and one follow-up prompt.
func parseResumeBody(body string) (string, string, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return "", "", fmt.Errorf(resumeUsageMessage)
	}

	parts := strings.SplitN(trimmed, " ", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf(resumeUsageMessage)
	}

	sessionID := strings.TrimSpace(parts[0])
	prompt := strings.TrimSpace(parts[1])
	if sessionID == "" || prompt == "" {
		return "", "", fmt.Errorf(resumeUsageMessage)
	}
	return sessionID, prompt, nil
}

// restoreHistory loads the current session history when a session manager is configured.
func (r *Runner) restoreHistory(ctx context.Context) (conversation.History, error) {
	if r == nil || r.SessionManager == nil {
		return conversation.History{}, nil
	}

	if r.SessionID == "" && r.ProjectPath != "" {
		snapshot, err := r.SessionManager.ResumeLatest(ctx, r.ProjectPath)
		if err == nil {
			r.SessionID = snapshot.Session.ID
			logger.DebugCF("repl", "restored latest session history for turn", map[string]any{
				"session_id":    r.sessionID(),
				"project_path":  r.ProjectPath,
				"message_count": len(snapshot.Session.Messages),
				"resumed":       snapshot.Resumed,
			})
			return conversation.History{Messages: snapshot.Session.Messages}, nil
		}
		if !errors.Is(err, coresession.ErrSessionNotFound) {
			return conversation.History{}, err
		}
		logger.DebugCF("repl", "no latest session found for project; starting fresh session", map[string]any{
			"project_path": r.ProjectPath,
		})
	}

	snapshot, err := r.SessionManager.StartInProject(ctx, r.sessionID(), r.ProjectPath)
	if err != nil {
		return conversation.History{}, err
	}

	logger.DebugCF("repl", "restored session history for turn", map[string]any{
		"session_id":    r.sessionID(),
		"project_path":  r.ProjectPath,
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
	if r == nil {
		return uuid.NewString()
	}
	if r.SessionID == "" {
		r.SessionID = uuid.NewString()
	}
	return r.SessionID
}
