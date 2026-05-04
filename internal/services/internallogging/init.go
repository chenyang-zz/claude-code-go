package internallogging

import "context"

// InitOptions configures the one-shot initialization call. The struct is
// reserved for future expansion (e.g. injecting a custom logger or
// analytics sink) but currently only carries the optional permission
// context recorded with the 'initialization' moment.
type InitOptions struct {
	// ToolPermissionContext is the permission context snapshot to attach to
	// the initialization event. May be nil; the TS reference passes null
	// from main.tsx:2523.
	ToolPermissionContext interface{}
}

// Init records the 'initialization' moment for Ant-internal diagnostics.
// It is intended to be called exactly once from the application bootstrap
// (M2-4 will wire this into internal/app/bootstrap/app.go).
//
// The function is a no-op when FlagInternalLogging is disabled, so it is
// safe to call unconditionally from the bootstrap path.
func Init(opts InitOptions) {
	LogPermissionContextForAnts(
		context.Background(),
		opts.ToolPermissionContext,
		MomentInitialization,
	)
}
