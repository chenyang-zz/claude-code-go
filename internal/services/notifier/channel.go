package notifier

import (
	"math/rand"
)

// Channel constants mirror the TS preferredNotifChannel values verbatim so
// the Go port can read a TS-written settings file with no migration step.
const (
	ChannelAuto           = "auto"
	ChannelITerm2         = "iterm2"
	ChannelITerm2WithBell = "iterm2_with_bell"
	ChannelKitty          = "kitty"
	ChannelGhostty        = "ghostty"
	ChannelTerminalBell   = "terminal_bell"
	ChannelDisabled       = "notifications_disabled"
	ChannelNone           = "none"
)

// Method names returned by sendToChannel — fed into structured logs (the TS
// version forwards these to logEvent('tengu_notification_method_used', ...)).
const (
	MethodITerm2         = "iterm2"
	MethodITerm2WithBell = "iterm2_with_bell"
	MethodKitty          = "kitty"
	MethodGhostty        = "ghostty"
	MethodTerminalBell   = "terminal_bell"
	MethodDisabled       = "disabled"
	MethodNone           = "none"
	MethodNoneAvailable  = "no_method_available"
	MethodError          = "error"
)

// generateKittyID returns a small random integer used as the Kitty
// notification correlation id (i= parameter). Mirrors the TS
// generateKittyId(): Math.floor(Math.random() * 10000).
func generateKittyID() int {
	return rand.Intn(10000)
}

// sendToChannel dispatches the notification to the configured channel. It
// recovers from panics inside the notification implementations (matching the
// TS try/catch around sendToChannel) and reports MethodError so callers can
// degrade gracefully.
//
// The returned string is the actual method that was used (for analytics /
// debug logging). Channels that recognise themselves but produce no terminal
// output (disabled, none, default) return their corresponding Method*
// sentinel.
func (s *Service) sendToChannel(channel string, opts NotificationOptions) (used string) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.DebugCF("notifier", "channel dispatch panicked", map[string]any{
				"channel": channel,
				"panic":   r,
			})
			used = MethodError
		}
	}()

	switch channel {
	case ChannelAuto:
		return s.sendAuto(opts)
	case ChannelITerm2:
		s.terminal.NotifyITerm2(opts)
		return MethodITerm2
	case ChannelITerm2WithBell:
		s.terminal.NotifyITerm2(opts)
		s.terminal.NotifyBell()
		return MethodITerm2WithBell
	case ChannelKitty:
		s.terminal.NotifyKitty(opts, generateKittyID())
		return MethodKitty
	case ChannelGhostty:
		s.terminal.NotifyGhostty(opts)
		return MethodGhostty
	case ChannelTerminalBell:
		s.terminal.NotifyBell()
		return MethodTerminalBell
	case ChannelDisabled:
		return MethodDisabled
	default:
		return MethodNone
	}
}
