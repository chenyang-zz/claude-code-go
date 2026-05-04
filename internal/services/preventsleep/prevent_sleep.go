// Package preventsleep prevents macOS from sleeping while Claude is working.
//
// Uses the built-in `caffeinate` command to create a power assertion that
// prevents idle sleep. This keeps the Mac awake during API requests and
// tool execution so long-running operations don't get interrupted.
//
// The caffeinate process is spawned with a timeout and periodically restarted.
// This provides self-healing behavior: if the Go process is killed with
// SIGKILL (which doesn't run cleanup handlers), the orphaned caffeinate will
// automatically exit after the timeout expires.
//
// Only runs on macOS — no-op on other platforms.
package preventsleep

import (
	"context"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// caffeinateTimeoutSeconds is the value passed to `caffeinate -t`.
// The process auto-exits after this duration; we restart it before expiry
// to maintain continuous sleep prevention.
const caffeinateTimeoutSeconds = 300 // 5 minutes

// restartIntervalDuration is how often we proactively restart caffeinate.
// 4 minutes leaves a 1-minute buffer before the 5-minute -t timeout fires.
const restartIntervalDuration = 4 * time.Minute

// Package-level state. All access is guarded by `mu`.
//
// Invariants:
//   - refCount is non-negative; Stop never decrements below 0.
//   - When refCount == 0, proc is nil and cancelRestartLoop is nil.
//   - When refCount > 0, a restart loop has been started (cancelRestartLoop
//     is non-nil) and a caffeinate process either runs or has just exited
//     and will be respawned on the next tick.
var (
	mu                sync.Mutex
	refCount          int
	proc              *exec.Cmd
	cancelRestartLoop context.CancelFunc
	cleanupRegistered bool
)

// spawner indirects exec.CommandContext so tests can replace the spawner
// with a fake binary or no-op without touching mu-protected state. Tests
// must set/restore this under their own synchronization.
var spawner = func(ctx context.Context) *exec.Cmd {
	// -i: idle-sleep assertion (least aggressive — display can still sleep).
	// -t: auto-exit after N seconds (self-heals if our process is SIGKILLed).
	return exec.CommandContext(ctx, "caffeinate", "-i", "-t", itoa(caffeinateTimeoutSeconds))
}

// Start increments the reference count and, on the 0→1 transition, spawns
// the caffeinate subprocess and starts the restart loop. It is a no-op on
// non-darwin platforms.
//
// Pair every Start with exactly one Stop (typically via defer in the caller).
func Start() {
	if runtime.GOOS != "darwin" {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	refCount++
	if refCount != 1 {
		return
	}

	startRestartLoopLocked()
	spawnCaffeinateLocked()
}

// Stop decrements the reference count and, on the 1→0 transition, terminates
// the caffeinate subprocess and stops the restart loop. Calls beyond zero are
// no-ops (refCount has a zero floor). It is a no-op on non-darwin platforms.
func Stop() {
	if runtime.GOOS != "darwin" {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if refCount == 0 {
		return
	}
	refCount--
	if refCount > 0 {
		return
	}

	stopRestartLoopLocked()
	killCaffeinateLocked()
}

// ForceStop unconditionally clears the reference count and tears down both
// the restart loop and the caffeinate subprocess. It is intended as the
// process-exit fallback (App.Run defers it via the cleanup function returned
// by Init). Safe to call multiple times and on any platform.
func ForceStop() {
	if runtime.GOOS != "darwin" {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	refCount = 0
	stopRestartLoopLocked()
	killCaffeinateLocked()
}

// itoa is a tiny strconv-free integer-to-string helper to avoid pulling in
// strconv just for the constant arg formatting. Caffeinate's -t value is
// always a small positive int.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
