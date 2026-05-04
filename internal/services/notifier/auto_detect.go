package notifier

import "os"

// Terminal classifies the host terminal emulator for notification routing.
type Terminal int

const (
	// TerminalUnknown is the default when no recognised marker is present.
	TerminalUnknown Terminal = iota
	// TerminalAppleTerminal is macOS's bundled Terminal.app.
	TerminalAppleTerminal
	// TerminalITerm is iTerm2.
	TerminalITerm
	// TerminalKitty is the Kitty terminal.
	TerminalKitty
	// TerminalGhostty is the Ghostty terminal.
	TerminalGhostty
)

// String returns a human-friendly name (used in debug logs).
func (t Terminal) String() string {
	switch t {
	case TerminalAppleTerminal:
		return "Apple_Terminal"
	case TerminalITerm:
		return "iTerm.app"
	case TerminalKitty:
		return "kitty"
	case TerminalGhostty:
		return "ghostty"
	default:
		return "unknown"
	}
}

// DetectTerminal infers the host terminal emulator from environment
// variables. The detection rules mirror src/utils/env.ts (simplified — we
// only need the four terminals notifier cares about).
//
// Priority:
//  1. KITTY_WINDOW_ID set ⇒ Kitty
//  2. TERM_PROGRAM=iTerm.app ⇒ iTerm2
//  3. TERM_PROGRAM=ghostty ⇒ Ghostty
//  4. TERM_PROGRAM=Apple_Terminal ⇒ Apple Terminal
//  5. otherwise ⇒ Unknown
func DetectTerminal() Terminal {
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return TerminalKitty
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "iTerm.app":
		return TerminalITerm
	case "ghostty":
		return TerminalGhostty
	case "Apple_Terminal":
		return TerminalAppleTerminal
	}
	return TerminalUnknown
}

// sendAuto runs the channel=auto routing: pick a notification method based
// on the detected terminal, returning the method name used (matches the TS
// sendAuto return value vocabulary).
//
// Apple_Terminal: TS code probes osascript+defaults+plist to detect whether
// the bell is disabled; the Go port intentionally drops that probe (see
// analysis-m1.md §1.2 "[Apple Terminal bell] **裁剪**") and reports
// "no_method_available" so callers can fall back gracefully without
// requiring a macOS-only subprocess dance.
func (s *Service) sendAuto(opts NotificationOptions) string {
	switch DetectTerminal() {
	case TerminalAppleTerminal:
		// Bell-disabled probe omitted in Go port; always report unavailable.
		return MethodNoneAvailable
	case TerminalITerm:
		s.terminal.NotifyITerm2(opts)
		return MethodITerm2
	case TerminalKitty:
		s.terminal.NotifyKitty(opts, generateKittyID())
		return MethodKitty
	case TerminalGhostty:
		s.terminal.NotifyGhostty(opts)
		return MethodGhostty
	default:
		return MethodNoneAvailable
	}
}
