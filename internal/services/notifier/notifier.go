// Package notifier implements terminal-channel notification dispatch for
// Claude Code (Go port of src/services/notifier.ts). Given a message + title
// + notificationType it (1) fans the event out to the configured
// notification hooks (delegated to engine.Runtime.RunNotificationHooks via
// the HookRunner interface) and (2) routes the notification to the user's
// preferred channel (iTerm2 OSC 9 / Kitty OSC 99 / Ghostty OSC 777 /
// terminal bell / auto-detect / disabled / none).
//
// The package is safe for concurrent use. All terminal writes go through
// TerminalNotifier which serialises access to the underlying io.Writer.
package notifier

import (
	"context"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// DefaultTitle is used when NotificationOptions.Title is empty. Matches the
// TS DEFAULT_TITLE constant.
const DefaultTitle = "Claude Code"

// NotificationOptions describes a single notification's content + type.
//
// Mirrors src/services/notifier.ts NotificationOptions:
//   - Message is required.
//   - Title is optional and defaults to DefaultTitle when empty.
//   - NotificationType is the analytic category (e.g. "idle_prompt",
//     "auth_success", "worker_permission_prompt"). It is forwarded to the
//     hook runner unchanged.
type NotificationOptions struct {
	Message          string
	Title            string
	NotificationType string
}

// HookRunner abstracts the smallest part of engine.Runtime that notifier
// needs: dispatching a Notification hook event. The concrete implementation
// lives in internal/runtime/engine (RunNotificationHooks landed in batch-78)
// and is wired into the Service at bootstrap time.
//
// The method does not return an error because the upstream
// engine.Runtime.RunNotificationHooks is fire-and-forget — hook execution
// failures are surfaced through the engine's own logging path.
type HookRunner interface {
	RunNotificationHooks(ctx context.Context, message, title, notificationType, cwd string)
}

// ChannelGetter returns the user's preferredNotifChannel setting at call
// time. The lookup is deferred to avoid stapling notifier to a particular
// config-loading mechanism (settings reload, dynamic override, etc.).
//
// If the getter returns the empty string the service treats the channel as
// "none" (no notification emitted, no error).
type ChannelGetter func() string

// CWDGetter returns the working directory used for hook scoping. Hooks may
// be scoped per-project, so this is read on every Send call rather than
// captured at construction time.
type CWDGetter func() string

// Service is the notifier package's main entrypoint. Construct with Init()
// and call Send() to dispatch a notification.
type Service struct {
	terminal      *TerminalNotifier
	hookRunner    HookRunner
	logger        Logger
	channelGetter ChannelGetter
	cwdGetter     CWDGetter
}

// Logger is the subset of pkg/logger that notifier uses. Defining it as a
// minimal interface keeps tests independent of the global zerolog state.
type Logger interface {
	DebugCF(component, message string, fields map[string]any)
	WarnCF(component, message string, fields map[string]any)
}

// loggerAdapter routes the interface calls to the package-level logger
// helpers. Used as a default when callers don't inject a custom Logger.
type loggerAdapter struct{}

func (loggerAdapter) DebugCF(c, m string, f map[string]any) { logger.DebugCF(c, m, f) }
func (loggerAdapter) WarnCF(c, m string, f map[string]any)  { logger.WarnCF(c, m, f) }

// Send executes one notification dispatch. The TS counterpart returns
// Promise<void>; the Go version returns an error so callers can decide
// whether to surface hook-runner failures (the TS code never surfaced them
// either — most call sites should ignore the return value).
//
// Order of operations matches src/services/notifier.ts:
//  1. Read preferredNotifChannel from the channel getter.
//  2. Run the Notification hooks (ignored on error — logged at debug only,
//     since hooks are best-effort and must not block UI feedback).
//  3. Route to the configured channel.
//  4. Emit a debug log capturing the configured channel + actual method
//     used (the analytics logEvent call from TS is intentionally omitted
//     until a future analytics batch wires it in).
func (s *Service) Send(ctx context.Context, opts NotificationOptions) error {
	channel := ""
	if s.channelGetter != nil {
		channel = s.channelGetter()
	}
	if channel == "" {
		channel = ChannelNone
	}

	if s.hookRunner != nil {
		cwd := ""
		if s.cwdGetter != nil {
			cwd = s.cwdGetter()
		}
		s.hookRunner.RunNotificationHooks(ctx, opts.Message, opts.Title, opts.NotificationType, cwd)
	}

	method := s.sendToChannel(channel, opts)

	s.logger.DebugCF("notifier", "channel routed", map[string]any{
		"configured":        channel,
		"method_used":       method,
		"notification_type": opts.NotificationType,
		"terminal":          DetectTerminal().String(),
	})

	return nil
}
