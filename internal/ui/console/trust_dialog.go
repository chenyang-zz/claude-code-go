package console

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// TrustDialogResult represents the user's response to the trust dialog.
type TrustDialogResult int

const (
	// TrustResultAccepted means the user trusts the current workspace.
	TrustResultAccepted TrustDialogResult = iota
	// TrustResultRejected means the user does not trust the current workspace.
	TrustResultRejected
)

// TrustDialog displays an interactive terminal prompt asking the user whether
// they trust the current workspace. It returns the user's decision and whether
// the prompt was actually shown (false when non-interactive or already trusted).
//
// The dialog can be skipped via:
//   - Non-TTY stdin (piped input, CI environments)
//   - Explicit skip flag (handled by the caller before invoking this function)
func TrustDialog(cwd string) (TrustDialogResult, bool, error) {
	// Skip if stdin is not a terminal (non-interactive mode).
	if !isTerminal(os.Stdin) {
		return TrustResultRejected, false, nil
	}

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  Accessing workspace:\n")
	fmt.Fprintf(os.Stderr, "  %s\n\n", cwd)
	fmt.Fprintf(os.Stderr, "  Quick safety check: Is this a project you created or one you trust?\n")
	fmt.Fprintf(os.Stderr, "  (Like your own code, a well-known open source project, or work from your team).\n")
	fmt.Fprintf(os.Stderr, "  If not, take a moment to review what's in this folder first.\n\n")
	fmt.Fprintf(os.Stderr, "  Claude Code will be able to read, edit, and execute files here.\n\n")
	fmt.Fprintf(os.Stderr, "  Security guide: https://code.claude.com/docs/en/security\n\n")
	fmt.Fprintf(os.Stderr, "  [1] Yes, I trust this folder\n")
	fmt.Fprintf(os.Stderr, "  [2] No, exit\n\n")
	fmt.Fprintf(os.Stderr, "  Enter choice (1/2): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		// EOF on a terminal usually means the input stream was closed
		// (e.g. piped input exhausted) — treat as non-interactive.
		if err == io.EOF {
			return TrustResultRejected, false, nil
		}
		return TrustResultRejected, true, fmt.Errorf("read trust dialog input: %w", err)
	}

	choice := strings.TrimSpace(input)
	switch choice {
	case "1", "y", "yes", "Y", "YES":
		return TrustResultAccepted, true, nil
	default:
		return TrustResultRejected, true, nil
	}
}

// isTerminal reports whether the given file descriptor is connected to a terminal.
func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	// A terminal has mode type CharacterDevice.
	return stat.Mode()&os.ModeCharDevice != 0
}
