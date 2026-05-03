package repl

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	"github.com/sheepzhao/claude-code-go/internal/platform/remote"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
	"github.com/sheepzhao/claude-code-go/internal/services/settingssync"
	"github.com/sheepzhao/claude-code-go/internal/services/tips"
	"github.com/sheepzhao/claude-code-go/internal/ui/console"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
	"github.com/sheepzhao/claude-code-go/pkg/sdk"
)

// WorktreeLister resolves Git worktree paths for the current workspace without coupling runtime to one platform implementation.
type WorktreeLister interface {
	// ListWorktrees returns visible worktree paths for cwd, or an empty slice when none are available.
	ListWorktrees(ctx context.Context, cwd string) ([]string, error)
}

type sessionLifecycleEngine interface {
	RunSessionEndHooks(ctx context.Context, reason string, cwd string)
	RunUserPromptSubmitHooks(ctx context.Context, prompt string, cwd string) (results []hook.HookResult, blocked bool, blockingMessages []string)
}

// RemoteLifecycle wires remote stream subscription lifecycle into one REPL turn.
type RemoteLifecycle interface {
	// Subscribe opens one remote subscription and returns its unsubscribe handle.
	// onEvent is called for each remote event received; nil is accepted and events are discarded.
	Subscribe(ctx context.Context, session coreconfig.RemoteSessionConfig, onEvent func(remote.Event)) (func() error, error)
	// ActiveSubscriptionCount returns the number of currently active remote stream subscriptions.
	ActiveSubscriptionCount() int
	// IsClosed reports whether the lifecycle manager has been globally closed.
	IsClosed() bool
	// ConnectionState returns the current connection state label.
	ConnectionState() string
	// ReconnectCount returns the number of successful reconnections since startup.
	ReconnectCount() int
	// LastDisconnectError returns the error that caused the most recent disconnect, or nil.
	LastDisconnectError() error
	// LastDisconnectTime returns the timestamp of the most recent disconnect, or zero time.
	LastDisconnectTime() time.Time
	// Send writes raw bytes to the active remote transport.
	Send(data []byte) error
}

// RemoteSender sends user messages to a remote session via HTTP POST.
type RemoteSender interface {
	// SendUserMessage posts one user message to the remote session endpoint.
	SendUserMessage(ctx context.Context, msg sdk.User) error
}

// Runner coordinates one CLI turn between parsed input, engine execution and console rendering.
type Runner struct {
	// Engine handles normal prompt execution.
	Engine engine.Engine
	// Renderer handles console output for both engine events and slash placeholders.
	Renderer console.EventRenderer
	// Commands resolves registered slash handlers for REPL dispatch.
	Commands command.Registry
	// ProjectPath identifies the current workspace used for project-scoped session recovery.
	ProjectPath string
	// RemoteSession stores the minimum remote-mode context injected during bootstrap.
	RemoteSession coreconfig.RemoteSessionConfig
	// RemoteLifecycle manages optional remote stream subscribe/unsubscribe around one prompt turn.
	RemoteLifecycle RemoteLifecycle
	// RemoteSender posts user messages to the remote session via HTTP POST.
	RemoteSender RemoteSender
	// ApprovalService resolves runtime approval prompts for remote permission requests.
	ApprovalService approval.Service
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
	// nextSessionStartSource is attached to the next engine request when the
	// runner has just switched to a new logical session.
	nextSessionStartSource string
}

// NewRunner builds a runner from explicit dependencies.
func NewRunner(eng engine.Engine, renderer console.EventRenderer) *Runner {
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
	// Fire-and-forget: upload local settings to remote (interactive CLI only).
	// Non-blocking and fail-open — a failed upload is silently discarded.
	settingssync.UploadUserSettingsInBackground()

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
		r.endActiveSession(ctx, parsed.Command, result.NewSessionID)
		r.SessionID = result.NewSessionID
		r.nextSessionStartSource = parsed.Command
		logger.DebugCF("repl", "switched active session from slash command", map[string]any{
			"command":    parsed.Command,
			"session_id": r.SessionID,
		})
	}
	if result.ShouldQuery {
		history, err := r.restoreHistory(ctx, parsed.ContinueLatest, parsed.ForkSession)
		if err != nil {
			if errors.Is(err, errContinueHandled) {
				return nil
			}
			return err
		}
		return r.runPrompt(ctx, history, result.Output)
	}
	if result.Output != "" {
		return r.Renderer.RenderLine(result.Output)
	}
	return nil
}

// runPrompt appends one user prompt onto the supplied history, executes the engine and persists the final history.
func (r *Runner) runPrompt(ctx context.Context, history conversation.History, prompt string) error {
	remoteUnsubscribe := r.subscribeRemoteStream(ctx)
	if remoteUnsubscribe != nil {
		defer func() {
			if err := remoteUnsubscribe(); err != nil {
				logger.WarnCF("repl", "failed to unsubscribe remote stream", map[string]any{
					"session_id": r.sessionID(),
					"error":      err.Error(),
				})
			}
		}()
	}

	if blocked, err := r.runUserPromptSubmitHooks(ctx, prompt); err != nil {
		return err
	} else if blocked {
		return nil
	}

	// Show a contextual tip before the model responds.
	if tip := tips.GetTipToShow(); tip != nil {
		if err := r.Renderer.RenderLine(fmt.Sprintf("💡 %s", tip.Content)); err == nil {
			tips.OnTipShown(tip)
		}
	}

	requestHistory := history.Clone()
	requestHistory.Append(message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart(prompt),
		},
	})

	// Forward the user input to the remote session when a sender is configured.
	if r.RemoteSender != nil {
		msg := sdk.User{
			Base:    sdk.Base{Type: "user"},
			Message: prompt,
		}
		if sendErr := r.RemoteSender.SendUserMessage(ctx, msg); sendErr != nil {
			logger.WarnCF("repl", "failed to send user message to remote session", map[string]any{
				"session_id": r.sessionID(),
				"error":      sendErr.Error(),
			})
			if se, ok := remote.IsSendError(sendErr); ok {
				_ = r.Renderer.RenderLine(fmt.Sprintf("Warning: could not reach remote session (%s)", se.Kind))
			}
		}
	}

	stream, err := r.Engine.Run(ctx, conversation.RunRequest{
		SessionID:          r.sessionID(),
		Messages:           requestHistory.Messages,
		CWD:                r.ProjectPath,
		SessionStartSource: r.consumeSessionStartSource(),
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

// runUserPromptSubmitHooks fires UserPromptSubmit hooks before forwarding the
// prompt into the engine. When a hook blocks the prompt (exit code 2), the
// blocking message is rendered in the same `UserPromptSubmit operation
// blocked by hook:\n<stderr>` format as the TypeScript implementation and the
// caller is expected to abort the turn without mutating the conversation
// history. Returns blocked=true when the prompt is blocked, blocked=false
// otherwise. A non-nil error indicates a renderer failure that should
// propagate up the REPL.
func (r *Runner) runUserPromptSubmitHooks(ctx context.Context, prompt string) (bool, error) {
	if r == nil || r.Engine == nil {
		return false, nil
	}
	lifecycle, ok := r.Engine.(sessionLifecycleEngine)
	if !ok {
		return false, nil
	}
	_, blocked, blockingMessages := lifecycle.RunUserPromptSubmitHooks(ctx, prompt, r.ProjectPath)
	if !blocked {
		return false, nil
	}

	logger.DebugCF("repl", "user prompt submit hook blocked", map[string]any{
		"session_id":     r.sessionID(),
		"blocking_count": len(blockingMessages),
	})

	body := strings.Join(blockingMessages, "\n")
	rendered := fmt.Sprintf("UserPromptSubmit operation blocked by hook:\n%s", strings.TrimRight(body, "\n"))
	if r.Renderer != nil {
		if err := r.Renderer.RenderLine(rendered); err != nil {
			return true, err
		}
	}
	return true, nil
}

func (r *Runner) subscribeRemoteStream(ctx context.Context) func() error {
	if r == nil || !r.RemoteSession.Enabled || r.RemoteLifecycle == nil {
		return nil
	}

	bridge := &RemoteEventBridge{Renderer: r.Renderer}
	onEvent := bridge.OnEvent()
	permissionHandler := NewRemotePermissionHandler(r.ApprovalService)

	var sm *remote.SessionManager
	sm = remote.NewSessionManager(r.RemoteSession, r.RemoteLifecycle, remote.SessionCallbacks{
		OnSDKMessage: func(data []byte) {
			onEvent(remote.Event{Data: data})
		},
		OnPermissionRequest: func(req *sdk.ControlPermissionRequest, requestID string) {
			go permissionHandler.HandlePermissionRequest(ctx, sm, req, requestID)
		},
		OnPermissionCancelled: func(requestID string, toolUseID string) {
			logger.DebugCF("repl", "remote permission request cancelled", map[string]any{
				"request_id":  requestID,
				"tool_use_id": toolUseID,
			})
		},
	})

	unsubscribe, err := r.RemoteLifecycle.Subscribe(ctx, r.RemoteSession, sm.HandleEvent)
	if err != nil {
		logger.WarnCF("repl", "failed to subscribe remote stream", map[string]any{
			"session_id": r.sessionID(),
			"error":      err.Error(),
		})
		return nil
	}

	return func() error {
		if err := unsubscribe(); err != nil {
			return err
		}
		sm.Disconnect()
		return nil
	}
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

	if r.SessionID != "" {
		recovered, err := r.SessionManager.Recover(ctx, r.SessionID)
		if err == nil {
			return r.consumeRecoveredSnapshot("restore_session", recovered, false)
		}
		if !errors.Is(err, coresession.ErrSessionNotFound) {
			return conversation.History{}, err
		}
	}

	if r.SessionID == "" && r.ProjectPath != "" {
		recovered, err := r.SessionManager.RecoverLatest(ctx, r.ProjectPath)
		if err == nil {
			return r.consumeRecoveredSnapshot("restore_latest", recovered, false)
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
	if snapshot.Resumed {
		r.nextSessionStartSource = "resume"
	}
	return conversation.History{Messages: snapshot.Session.Messages}, nil
}

func (r *Runner) restoreContinueHistory(ctx context.Context, forkSession bool) (conversation.History, bool, error) {
	recovered, err := r.SessionManager.RecoverLatest(ctx, r.ProjectPath)
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
		recovered.Snapshot, err = r.forkSnapshot(ctx, recovered.Snapshot)
		if err != nil {
			return conversation.History{}, false, err
		}
	}
	history, err := r.consumeRecoveredSnapshot("restore_continue", recovered, forkSession)
	if err != nil {
		return conversation.History{}, false, err
	}
	r.nextSessionStartSource = "resume"
	return history, true, nil
}

// consumeRecoveredSnapshot normalizes one recovered snapshot into the runnable history consumed by the next prompt.
func (r *Runner) consumeRecoveredSnapshot(action string, recovered runtimesession.RecoveredSnapshot, forkSession bool) (conversation.History, error) {
	r.nextSessionStartSource = "resume"
	r.SessionID = recovered.Snapshot.Session.ID
	if recovered.Snapshot.Session.ProjectPath != "" {
		r.ProjectPath = recovered.Snapshot.Session.ProjectPath
	}

	history := conversation.History{
		Messages: runtimesession.RunnableRecoveredMessages(recovered.Snapshot.Session.Messages, recovered.State),
	}
	logger.DebugCF("repl", "prepared recovered session history for turn", map[string]any{
		"action":               action,
		"session_id":           r.SessionID,
		"project_path":         r.ProjectPath,
		"message_count":        len(recovered.Snapshot.Session.Messages),
		"prepared_count":       len(history.Messages),
		"fork_session":         forkSession,
		"interruption_kind":    recovered.State.Kind,
		"needs_continuation":   recovered.State.NeedsContinuation,
		"history_ends_on_user": len(history.Messages) > 0 && history.Messages[len(history.Messages)-1].Role == message.RoleUser,
	})
	return history, nil
}

func (r *Runner) consumeSessionStartSource() string {
	if r == nil {
		return "startup"
	}
	source := strings.TrimSpace(r.nextSessionStartSource)
	r.nextSessionStartSource = ""
	if source == "" {
		return "startup"
	}
	return source
}

func (r *Runner) endActiveSession(ctx context.Context, reason string, nextSessionID string) {
	if r == nil {
		return
	}
	currentSessionID := strings.TrimSpace(r.SessionID)
	if currentSessionID == "" || currentSessionID == strings.TrimSpace(nextSessionID) {
		return
	}
	lifecycle, ok := r.Engine.(sessionLifecycleEngine)
	if !ok {
		return
	}
	lifecycle.RunSessionEndHooks(ctx, reason, r.ProjectPath)
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
