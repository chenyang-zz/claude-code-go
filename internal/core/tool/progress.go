package tool

import "context"

type progressKey struct{}

// ProgressFunc is the callback signature tools use to emit incremental progress.
// Implementations must be safe to call from any goroutine.
type ProgressFunc func(data any)

// WithProgress returns a new context that carries the provided progress callback.
// Tools retrieve the callback via GetProgress and call it during long-running operations.
func WithProgress(ctx context.Context, fn ProgressFunc) context.Context {
	return context.WithValue(ctx, progressKey{}, fn)
}

// GetProgress retrieves the progress callback from the context.
// Returns nil if no callback was set, allowing callers to skip progress reporting.
func GetProgress(ctx context.Context) ProgressFunc {
	if ctx == nil {
		return nil
	}
	fn, _ := ctx.Value(progressKey{}).(ProgressFunc)
	return fn
}

// ReportProgress is a nil-safe helper that calls the progress callback from the
// context when one is available. Does nothing when no callback is set.
func ReportProgress(ctx context.Context, data any) {
	if fn := GetProgress(ctx); fn != nil {
		fn(data)
	}
}
