package commands

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/platform/remote"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// RemoteStateProvider exposes observable remote subscription and lifecycle state for /session reporting.
type RemoteStateProvider interface {
	// ActiveSubscriptionCount returns the number of currently active remote stream subscriptions.
	ActiveSubscriptionCount() int
	// IsClosed reports whether the remote lifecycle manager has been globally closed.
	IsClosed() bool
	// ConnectionState returns the current resilient stream connection state label.
	ConnectionState() string
	// ReconnectCount returns the number of successful reconnections since startup.
	ReconnectCount() int
	// LastDisconnectError returns the error that caused the most recent disconnect, or nil.
	LastDisconnectError() error
	// LastDisconnectTime returns the timestamp of the most recent disconnect, or zero time.
	LastDisconnectTime() time.Time
}

// RemoteSendStateProvider exposes the HTTP POST sender state for /session reporting.
type RemoteSendStateProvider interface {
	// SendCount returns the total number of successful sends.
	SendCount() int64
	// LastSendTime returns the timestamp of the most recent successful send.
	LastSendTime() time.Time
}

// SessionCommand renders the text-only /session behavior including remote session URL, QR code, and subscription state.
type SessionCommand struct {
	// RemoteSession stores the current remote-mode context surfaced by bootstrap.
	RemoteSession coreconfig.RemoteSessionConfig
	// StateProvider supplies optional live remote subscription and lifecycle state for observability output.
	StateProvider RemoteStateProvider
	// SendStateProvider supplies optional HTTP POST sender state for observability output.
	SendStateProvider RemoteSendStateProvider
	// SubagentStateProvider supplies optional subagent state for observability output.
	SubagentStateProvider remote.RemoteSubagentStateProvider
	// AuthStateProvider supplies optional authentication state for observability output.
	AuthStateProvider remote.AuthStateProvider
}

// Metadata returns the canonical slash descriptor for /session.
func (c SessionCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "session",
		Description: "Show remote session URL and QR code",
		Usage:       "/session",
	}
}

// Execute reports the stable non-remote fallback until remote session infrastructure is migrated.
func (c SessionCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	if c.RemoteSession.Enabled && strings.TrimSpace(c.RemoteSession.URL) != "" {
		logger.DebugCF("commands", "rendered remote session command output", map[string]any{
			"remote_mode_available": true,
			"remote_session_url":    c.RemoteSession.URL,
		})
		return command.Result{
			Output: renderRemoteSession(c.RemoteSession, c.StateProvider, c.SendStateProvider, c.SubagentStateProvider, c.AuthStateProvider),
		}, nil
	}

	logger.DebugCF("commands", "rendered session command fallback output", map[string]any{
		"remote_mode_available": false,
	})

	return command.Result{
		Output: "Not in remote mode. Start with `claude --remote` to use this command.",
	}, nil
}

// renderRemoteSession formats the remote session details including QR code, URL, and optional live state.
func renderRemoteSession(remote coreconfig.RemoteSessionConfig, state RemoteStateProvider, sendState RemoteSendStateProvider, subagentState remote.RemoteSubagentStateProvider, authState remote.AuthStateProvider) string {
	var b strings.Builder
	b.WriteString("Remote session\n")
	b.WriteString(renderTextQRCode(remote.URL))
	b.WriteString("\nOpen in browser: ")
	b.WriteString(remote.URL)
	if state != nil {
		b.WriteString("\n\nConnection state:\n")
		fmt.Fprintf(&b, "  Status: %s\n", state.ConnectionState())
		fmt.Fprintf(&b, "  Subscriptions active: %d\n", state.ActiveSubscriptionCount())
		fmt.Fprintf(&b, "  Reconnections: %d\n", state.ReconnectCount())
		if lastErr := state.LastDisconnectError(); lastErr != nil {
			fmt.Fprintf(&b, "  Last disconnect: %s", lastErr.Error())
			if !state.LastDisconnectTime().IsZero() {
				fmt.Fprintf(&b, " (%s ago)", time.Since(state.LastDisconnectTime()).Round(time.Second).String())
			}
			b.WriteString("\n")
		}
		closedStr := "no"
		if state.IsClosed() {
			closedStr = "yes (closed)"
		}
		fmt.Fprintf(&b, "  Lifecycle closed: %s", closedStr)
	}
	if sendState != nil {
		b.WriteString("\n\nWrite path:\n")
		fmt.Fprintf(&b, "  HTTP POST sends: %d\n", sendState.SendCount())
		if last := sendState.LastSendTime(); !last.IsZero() {
			fmt.Fprintf(&b, "  Last send: %s ago", time.Since(last).Round(time.Second).String())
		} else {
			b.WriteString("  Last send: never")
		}
	}
	if subagentState != nil {
		b.WriteString("\n\nSubagents:\n")
		agents := subagentState.SubagentList()
		if len(agents) == 0 {
			b.WriteString("  No subagents observed.")
		} else {
			fmt.Fprintf(&b, "  Known subagents: %d\n", subagentState.SubagentCount())
			for _, a := range agents {
				fmt.Fprintf(&b, "  - %s", a.AgentID)
				if a.AgentType != "" {
					fmt.Fprintf(&b, " (%s)", a.AgentType)
				}
				fmt.Fprintf(&b, ": %s", a.Status)
				if a.EventCount > 0 {
					fmt.Fprintf(&b, ", events: %d", a.EventCount)
				}
				b.WriteString("\n")
			}
		}
	}
	if authState != nil {
		b.WriteString("\n\nAuthentication:\n")
		as := authState.AuthState()
		if as.Token == "" {
			b.WriteString("  No token configured")
		} else {
			fmt.Fprintf(&b, "  Source: %s\n", as.Source)
			if !as.RefreshedAt.IsZero() {
				fmt.Fprintf(&b, "  Last refresh: %s ago\n", time.Since(as.RefreshedAt).Round(time.Second).String())
			} else {
				b.WriteString("  Last refresh: never\n")
			}
			fmt.Fprintf(&b, "  Refresh count: %d", as.RefreshCount)
		}
	}
	return b.String()
}

// renderTextQRCode emits one stable ASCII QR-like block so `/session` remains observable in the text host.
func renderTextQRCode(value string) string {
	const size = 12
	sum := sha256.Sum256([]byte(value))
	lines := make([]string, 0, size)
	for row := 0; row < size; row++ {
		var line strings.Builder
		for col := 0; col < size; col++ {
			line.WriteString(qrCell(sum, row, col))
		}
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}

// qrCell deterministically maps one row/column position into an ASCII block for the session preview.
func qrCell(sum [32]byte, row int, col int) string {
	if isFinderCell(row, col) {
		return "##"
	}
	index := (row*12 + col) % len(sum)
	bit := (sum[index] >> uint((row+col)%8)) & 1
	if bit == 1 {
		return "##"
	}
	return "  "
}

// isFinderCell draws three simple corner anchors so the preview reads like a QR block in plain text.
func isFinderCell(row int, col int) bool {
	return inFinder(row, col) || inFinder(row, col-8) || inFinder(row-8, col)
}

// inFinder reports whether one normalized cell belongs to a 4x4 corner marker.
func inFinder(row int, col int) bool {
	return row >= 0 && row < 4 && col >= 0 && col < 4
}
