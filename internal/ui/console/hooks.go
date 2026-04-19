package console

import (
	"fmt"
	"io"
	"os"
)

// HookPrinter writes hook-related console output for the CLI flow.
type HookPrinter struct {
	// Writer receives rendered console output.
	Writer io.Writer
}

// NewHookPrinter builds a hook printer that defaults to stderr.
func NewHookPrinter(writer io.Writer) *HookPrinter {
	if writer == nil {
		writer = os.Stderr
	}
	return &HookPrinter{Writer: writer}
}

// HookStarted writes a progress line when a hook begins execution.
func (p *HookPrinter) HookStarted(command string) {
	if p == nil {
		return
	}
	fmt.Fprintf(p.Writer, "  running hook: %s\n", truncateCommand(command, 60))
}

// HookBlocking writes a message when a hook returns a blocking error.
func (p *HookPrinter) HookBlocking(command string, stderr string) {
	if p == nil {
		return
	}
	if stderr != "" {
		fmt.Fprintf(p.Writer, "  hook blocked (%s): %s\n", truncateCommand(command, 40), truncateOutput(stderr, 100))
	}
}

// HookError writes a message when a hook returns a non-blocking error.
func (p *HookPrinter) HookError(command string, stderr string) {
	if p == nil {
		return
	}
	if stderr != "" {
		fmt.Fprintf(p.Writer, "  hook error (%s): %s\n", truncateCommand(command, 40), truncateOutput(stderr, 100))
	}
}

// HookTimedOut writes a message when a hook exceeds its timeout.
func (p *HookPrinter) HookTimedOut(command string) {
	if p == nil {
		return
	}
	fmt.Fprintf(p.Writer, "  hook timed out: %s\n", truncateCommand(command, 60))
}

// HookSummary writes a summary line after all hooks for an event have executed.
func (p *HookPrinter) HookSummary(count int, blocking int, errors int) {
	if p == nil {
		return
	}
	if count == 0 {
		return
	}
	fmt.Fprintf(p.Writer, "  hooks: %d ran", count)
	if blocking > 0 {
		fmt.Fprintf(p.Writer, ", %d blocking", blocking)
	}
	if errors > 0 {
		fmt.Fprintf(p.Writer, ", %d errors", errors)
	}
	fmt.Fprintln(p.Writer)
}

// truncateCommand limits a command string for safe display.
func truncateCommand(s string, maxLen int) string {
	return truncateString(s, maxLen)
}

// truncateOutput limits an output string for safe display.
func truncateOutput(s string, maxLen int) string {
	return truncateString(s, maxLen)
}

// truncateString limits a string for display, adding ellipsis when truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
