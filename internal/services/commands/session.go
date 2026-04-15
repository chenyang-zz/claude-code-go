package commands

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// SessionCommand renders the minimum text-only /session behavior available before remote mode exists in the Go host.
type SessionCommand struct {
	// RemoteSession stores the current remote-mode context surfaced by bootstrap.
	RemoteSession coreconfig.RemoteSessionConfig
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
			Output: renderRemoteSession(c.RemoteSession),
		}, nil
	}

	logger.DebugCF("commands", "rendered session command fallback output", map[string]any{
		"remote_mode_available": false,
	})

	return command.Result{
		Output: "Not in remote mode. Start with `claude --remote` to use this command.",
	}, nil
}

// renderRemoteSession formats the minimum source-aligned remote session details for the text-only Go host.
func renderRemoteSession(remote coreconfig.RemoteSessionConfig) string {
	return fmt.Sprintf("Remote session\n%s\nOpen in browser: %s", renderTextQRCode(remote.URL), remote.URL)
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
