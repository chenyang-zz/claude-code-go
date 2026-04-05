package repl

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/sheepzhao/claude-code-go/internal/core/command"
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
	// Commands resolves registered slash handlers for REPL dispatch.
	Commands command.Registry
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
	resumeNoSessionsMessage    = "No conversations found to resume."
	continueNotConfigured      = "Continue command is not available because session storage is not configured."
	continueNotFoundMessage    = "No conversation found to continue."
)

var errContinueHandled = errors.New("continue flow already handled")

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
		return r.runSlashCommand(ctx, parsed)
	}

	history, err := r.restoreHistory(ctx, parsed.ContinueLatest, parsed.ForkSession)
	if err != nil {
		if errors.Is(err, errContinueHandled) {
			return nil
		}
		return err
	}
	return r.runPrompt(ctx, history, parsed.Body)
}

// runSlashCommand dispatches one parsed slash command through the registered command catalog.
func (r *Runner) runSlashCommand(ctx context.Context, parsed ParsedInput) error {
	if r == nil || r.Commands == nil {
		return r.Renderer.RenderLine(fmt.Sprintf("Slash command /%s is not supported yet.", parsed.Command))
	}

	cmd, ok := r.Commands.Get(parsed.Command)
	if !ok {
		return r.Renderer.RenderLine(fmt.Sprintf("Slash command /%s is not supported yet.", parsed.Command))
	}

	logger.DebugCF("repl", "dispatching slash command", map[string]any{
		"command":      parsed.Command,
		"has_body":     parsed.Body != "",
		"fork_session": parsed.ForkSession,
	})

	result, err := cmd.Execute(ctx, command.Args{
		Raw: strings.Fields(parsed.Body),
		Flags: map[string]string{
			"fork_session": fmt.Sprintf("%t", parsed.ForkSession),
		},
		RawLine: parsed.Body,
	})
	if err != nil {
		return err
	}
	if result.NewSessionID != "" {
		r.SessionID = result.NewSessionID
		logger.DebugCF("repl", "switched active session from slash command", map[string]any{
			"command":    parsed.Command,
			"session_id": r.SessionID,
		})
	}
	if result.Output != "" {
		return r.Renderer.RenderLine(result.Output)
	}
	return nil
}

// runResumeCommand restores one persisted session and immediately continues it with the provided prompt tail.
func (r *Runner) runResumeCommand(ctx context.Context, body string, forkSession bool) error {
	if strings.TrimSpace(body) == "" {
		return r.renderRecentResumeSessions(ctx)
	}

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

	if forkSession {
		snapshot, err = r.forkSnapshot(ctx, snapshot)
		if err != nil {
			return err
		}
	}

	r.SessionID = snapshot.Session.ID
	if snapshot.Session.ProjectPath != "" {
		r.ProjectPath = snapshot.Session.ProjectPath
	}
	logger.DebugCF("repl", "resumed session from slash command", map[string]any{
		"session_id":    r.SessionID,
		"fork_session":  forkSession,
		"project_path":  r.ProjectPath,
		"message_count": len(snapshot.Session.Messages),
	})
	return r.runPrompt(ctx, conversation.History{Messages: snapshot.Session.Messages}, prompt)
}

func (r *Runner) renderRecentResumeSessions(ctx context.Context) error {
	if r == nil || r.SessionManager == nil {
		return r.Renderer.RenderLine(resumeNotConfiguredMessage)
	}

	summaries, err := r.SessionManager.ListRecent(ctx, r.ProjectPath, 5)
	if err != nil {
		if strings.Contains(err.Error(), "missing project path") {
			return r.Renderer.RenderLine(resumeNoSessionsMessage)
		}
		return err
	}
	if len(summaries) == 0 {
		return r.Renderer.RenderLine(resumeNoSessionsMessage)
	}

	lines := []string{"Recent conversations:"}
	for _, summary := range summaries {
		lines = append(lines, formatRecentSessionLine(summary))
	}
	lines = append(lines, "Use /resume <session-id> <prompt> to continue one.")
	return r.Renderer.RenderLine(strings.Join(lines, "\n"))
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

// formatRecentSessionLine renders one recent-session candidate using a stable text-only layout.
func formatRecentSessionLine(summary coresession.Summary) string {
	preview := strings.TrimSpace(summary.Preview)
	if preview == "" {
		preview = "Previous conversation"
	}

	updatedAt := "unknown time"
	if !summary.UpdatedAt.IsZero() {
		updatedAt = summary.UpdatedAt.UTC().Format("2006-01-02 15:04 UTC")
	}

	projectHint := ""
	if summary.ProjectPath != "" {
		projectHint = fmt.Sprintf(" [%s]", filepath.Base(summary.ProjectPath))
	}

	return fmt.Sprintf("- %s | %s%s | %s", updatedAt, preview, projectHint, summary.ID)
}

// restoreHistory loads the current session history, optionally requiring explicit latest-session recovery or forking.
func (r *Runner) restoreHistory(ctx context.Context, explicitContinue bool, forkSession bool) (conversation.History, error) {
	if r == nil || r.SessionManager == nil {
		if explicitContinue {
			if err := r.Renderer.RenderLine(continueNotConfigured); err != nil {
				return conversation.History{}, err
			}
			return conversation.History{}, errContinueHandled
		}
		return conversation.History{}, nil
	}

	if explicitContinue {
		snapshot, handled, err := r.restoreContinueHistory(ctx, forkSession)
		if err != nil {
			return conversation.History{}, err
		}
		if handled {
			if len(snapshot.Messages) == 0 && r.SessionID == "" {
				return conversation.History{}, errContinueHandled
			}
			return snapshot, nil
		}
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

func (r *Runner) restoreContinueHistory(ctx context.Context, forkSession bool) (conversation.History, bool, error) {
	snapshot, err := r.SessionManager.ResumeLatest(ctx, r.ProjectPath)
	if err != nil {
		if errors.Is(err, coresession.ErrSessionNotFound) {
			if err := r.Renderer.RenderLine(continueNotFoundMessage); err != nil {
				return conversation.History{}, true, err
			}
			return conversation.History{}, true, nil
		}
		return conversation.History{}, false, err
	}

	if forkSession {
		snapshot, err = r.forkSnapshot(ctx, snapshot)
		if err != nil {
			return conversation.History{}, false, err
		}
	}

	r.SessionID = snapshot.Session.ID
	if snapshot.Session.ProjectPath != "" {
		r.ProjectPath = snapshot.Session.ProjectPath
	}
	logger.DebugCF("repl", "restored explicit continue session history for turn", map[string]any{
		"session_id":    r.SessionID,
		"project_path":  r.ProjectPath,
		"message_count": len(snapshot.Session.Messages),
		"fork_session":  forkSession,
	})
	return conversation.History{Messages: snapshot.Session.Messages}, true, nil
}

func (r *Runner) forkSnapshot(ctx context.Context, snapshot coresession.Snapshot) (coresession.Snapshot, error) {
	if r == nil || r.SessionManager == nil {
		return coresession.Snapshot{}, fmt.Errorf("missing session manager")
	}
	targetID := uuid.NewString()
	forked, err := r.SessionManager.Fork(ctx, snapshot.Session, targetID)
	if err != nil {
		return coresession.Snapshot{}, err
	}
	return forked, nil
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
