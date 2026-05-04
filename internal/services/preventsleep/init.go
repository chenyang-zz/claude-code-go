package preventsleep

import (
	"runtime"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// InitOptions are the initialization options for the prevent-sleep service.
// It is currently empty and reserved for future expansion (e.g. injecting
// a custom logger or spawner) so the bootstrap call site remains stable.
type InitOptions struct{}

// Init prepares the prevent-sleep service and returns a cleanup function
// to be deferred by App.Run. Init itself does NOT spawn caffeinate; the
// subprocess is only started on the first Start() call. The returned
// cleanup is equivalent to ForceStop and is safe to call multiple times
// (including on non-darwin platforms, where it is a no-op).
//
// On non-darwin platforms Init still returns a non-nil cleanup so callers
// can unconditionally `defer cleanup()`.
//
// The cleanupRegistered latch guards against double-registration if Init
// is ever invoked more than once in the same process (defence in depth —
// bootstrap should call Init exactly once).
func Init(_ InitOptions) (cleanup func()) {
	mu.Lock()
	already := cleanupRegistered
	cleanupRegistered = true
	mu.Unlock()

	if already {
		logger.DebugCF("preventsleep", "init called more than once; reusing existing cleanup", nil)
		return ForceStop
	}

	if runtime.GOOS != "darwin" {
		logger.DebugCF("preventsleep", "non-darwin platform, prevent-sleep is a no-op", map[string]any{
			"goos": runtime.GOOS,
		})
		return ForceStop
	}

	logger.DebugCF("preventsleep", "prevent-sleep service initialised", nil)
	return ForceStop
}
