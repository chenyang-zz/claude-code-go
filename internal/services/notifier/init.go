package notifier

import (
	"io"
	"os"
)

// InitOptions captures the dependencies the bootstrap layer must inject
// when constructing a notifier Service. M2-4 will populate these from the
// engine.Runtime + config layers; tests can build a Service directly via
// New() with custom collaborators.
type InitOptions struct {
	// HookRunner is the source of Notification hook dispatch. May be nil
	// (in which case Send simply skips the hook step).
	HookRunner HookRunner

	// ChannelGetter returns the current preferredNotifChannel setting.
	// May be nil; the service then defaults to ChannelNone (no-op).
	ChannelGetter ChannelGetter

	// CWDGetter returns the working directory passed to hook execution.
	// May be nil; an empty cwd is forwarded.
	CWDGetter CWDGetter

	// Writer is the destination for terminal escape sequences. nil ⇒
	// os.Stdout (the production default — matches the TS hook which writes
	// directly to process.stdout via writeRaw).
	Writer io.Writer

	// Logger is the structured logger sink. nil ⇒ the package-level
	// pkg/logger functions.
	Logger Logger
}

// Init constructs a Service with the supplied dependencies. The returned
// Service is ready to receive Send() calls; bootstrap code stores it on
// the App struct for later injection into REPL / hooks / OAuth flows.
func Init(opts InitOptions) *Service {
	w := opts.Writer
	if w == nil {
		w = os.Stdout
	}
	var lg Logger = loggerAdapter{}
	if opts.Logger != nil {
		lg = opts.Logger
	}
	return &Service{
		terminal:      NewTerminalNotifier(w),
		hookRunner:    opts.HookRunner,
		logger:        lg,
		channelGetter: opts.ChannelGetter,
		cwdGetter:     opts.CWDGetter,
	}
}
