package notifier

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// TerminalNotifier writes notification escape sequences to an io.Writer.
//
// The writer is injectable so tests can capture exact byte output. In
// production the bootstrap code passes os.Stdout (matching the TS
// useTerminalNotification hook which writes to the raw terminal stream).
type TerminalNotifier struct {
	mu sync.Mutex
	w  io.Writer
}

// NewTerminalNotifier returns a TerminalNotifier bound to w. If w is nil
// the notifier writes to os.Stdout.
func NewTerminalNotifier(w io.Writer) *TerminalNotifier {
	if w == nil {
		w = os.Stdout
	}
	return &TerminalNotifier{w: w}
}

// write serialises raw writes to the underlying writer. Multiple notify*
// calls on a single TerminalNotifier may run concurrently in different
// goroutines (e.g. iterm2_with_bell sequences two notify calls).
func (t *TerminalNotifier) write(s string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, _ = io.WriteString(t.w, s)
}

// NotifyITerm2 emits the iTerm2 OSC 9 notification:
//
//	ESC ] 9 ; \n\n<title>:\n<message> BEL
//
// wrapped for tmux/screen passthrough. When title is empty the message alone
// is sent (still prefixed by two newlines, matching the TS implementation).
func (t *TerminalNotifier) NotifyITerm2(opts NotificationOptions) {
	display := opts.Message
	if opts.Title != "" {
		display = fmt.Sprintf("%s:\n%s", opts.Title, opts.Message)
	}
	body := "\n\n" + display
	seq := buildOSC(BEL, OSCITerm2, body)
	t.write(wrapForMultiplexer(seq))
}

// NotifyKitty emits the Kitty OSC 99 notification protocol — three sequences
// terminated by ST:
//
//	ESC ] 99 ; i=ID:d=0:p=title ; <title> ST
//	ESC ] 99 ; i=ID:p=body      ; <message> ST
//	ESC ] 99 ; i=ID:d=1:a=focus ;          ST
//
// Each sequence is wrapped for tmux/screen passthrough. Title falls back to
// the default "Claude Code" if empty.
func (t *TerminalNotifier) NotifyKitty(opts NotificationOptions, id int) {
	title := opts.Title
	if title == "" {
		title = DefaultTitle
	}
	idStr := fmt.Sprintf("i=%d", id)

	t.write(wrapForMultiplexer(buildOSC(ST, OSCKitty, idStr+":d=0:p=title", title)))
	t.write(wrapForMultiplexer(buildOSC(ST, OSCKitty, idStr+":p=body", opts.Message)))
	t.write(wrapForMultiplexer(buildOSC(ST, OSCKitty, idStr+":d=1:a=focus", "")))
}

// NotifyGhostty emits the Ghostty OSC 777 notification protocol:
//
//	ESC ] 777 ; notify ; <title> ; <message> BEL
//
// wrapped for tmux/screen passthrough. Title falls back to the default
// "Claude Code" if empty.
func (t *TerminalNotifier) NotifyGhostty(opts NotificationOptions) {
	title := opts.Title
	if title == "" {
		title = DefaultTitle
	}
	seq := buildOSC(BEL, OSCGhostty, "notify", title, opts.Message)
	t.write(wrapForMultiplexer(seq))
}

// NotifyBell emits a raw BEL (\x07). Deliberately NOT wrapped: a raw BEL
// triggers tmux's bell-action (window flag), while wrapping would make it
// opaque DCS payload and lose that fallback.
func (t *TerminalNotifier) NotifyBell() {
	t.write(BEL)
}
