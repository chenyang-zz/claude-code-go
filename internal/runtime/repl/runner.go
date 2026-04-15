package repl

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
	"github.com/sheepzhao/claude-code-go/internal/ui/console"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// WorktreeLister resolves Git worktree paths for the current workspace without coupling runtime to one platform implementation.
type WorktreeLister interface {
	// ListWorktrees returns visible worktree paths for cwd, or an empty slice when none are available.
	ListWorktrees(ctx context.Context, cwd string) ([]string, error)
}

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
	// RemoteSession stores the minimum remote-mode context injected during bootstrap.
	RemoteSession coreconfig.RemoteSessionConfig
	// SessionID identifies the current logical CLI session.
	SessionID string
	// SessionManager restores previously persisted conversation history when available.
	SessionManager *runtimesession.Manager
	// AutoSave persists the final normalized history after each successful turn.
	AutoSave *runtimesession.AutoSave
	// Input reads one-off interactive replies such as `/resume` picker selections.
	Input io.Reader
	// inputSource tracks which reader the buffered interactive reader was built from.
	inputSource io.Reader
	// inputReader reuses buffered stdin reads across multi-step interactive commands.
	inputReader *bufio.Reader
	// WorktreeLister resolves same-repo worktree membership for cross-project resume decisions.
	WorktreeLister WorktreeLister
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
	resumeMultipleMatchesUsage = "Use /resume <session-id> <prompt> to continue one of them."
	resumeCrossProjectUsage    = "For another project, change to that directory and use /resume <session-id> <prompt> there."
	resumeSelectionPrompt      = "Select a conversation number to resume, or press Enter to cancel."
	resumeSelectionCancelled   = "Resume cancelled."
	resumeSelectionInvalid     = "Invalid selection. Use a number from the list or press Enter to cancel."
	continueNotConfigured      = "Continue command is not available because session storage is not configured."
	continueNotFoundMessage    = "No conversation found to continue."
	renameUsageMessage         = "Rename command requires a title: use /rename <title>."
	renameNotConfiguredMessage = "Rename command is not available because session storage is not configured."
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
