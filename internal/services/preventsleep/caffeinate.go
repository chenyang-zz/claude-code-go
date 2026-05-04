package preventsleep

import (
	"context"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// startRestartLoopLocked starts the periodic respawn goroutine. Caller must
// hold mu. It is idempotent: if a loop is already running, this is a no-op.
//
// The loop fires every restartIntervalDuration; on each tick (while
// refCount > 0) it kills the current caffeinate and spawns a fresh one,
// keeping the assertion alive across the underlying -t 300 self-exit.
func startRestartLoopLocked() {
	if cancelRestartLoop != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancelRestartLoop = cancel
	go restartLoop(ctx)
}

// stopRestartLoopLocked cancels the running restart loop, if any. Caller
// must hold mu. Safe to call when no loop is running.
func stopRestartLoopLocked() {
	if cancelRestartLoop != nil {
		cancelRestartLoop()
		cancelRestartLoop = nil
	}
}

// restartLoop is the goroutine body. It exits when ctx is cancelled and
// otherwise restarts caffeinate every tick while refCount > 0.
func restartLoop(ctx context.Context) {
	ticker := time.NewTicker(restartIntervalDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mu.Lock()
			if refCount > 0 {
				logger.DebugCF("preventsleep", "restarting caffeinate to maintain sleep prevention", nil)
				killCaffeinateLocked()
				spawnCaffeinateLocked()
			}
			mu.Unlock()
		}
	}
}

// spawnCaffeinateLocked starts a new caffeinate subprocess. Caller must
// hold mu. If a process is already running, this is a no-op.
//
// Spawn failures (caffeinate missing, fork error) are logged and swallowed
// — sleep prevention is best-effort and must not break the host process.
//
// A monitor goroutine waits on the process and clears `proc` on exit, but
// only if `proc` still points at *this* instance. The thisProc capture
// avoids racing with a subsequent spawn from the restart loop: when the
// loop kills the old process and spawns a new one, the old monitor's
// Wait() returns and would otherwise null out the new instance.
func spawnCaffeinateLocked() {
	if proc != nil {
		return
	}

	cmd := spawner(context.Background())
	if err := cmd.Start(); err != nil {
		logger.DebugCF("preventsleep", "caffeinate spawn failed", map[string]any{
			"error": err.Error(),
		})
		return
	}
	proc = cmd

	pid := -1
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	logger.DebugCF("preventsleep", "caffeinate spawned", map[string]any{
		"pid": pid,
	})

	thisProc := cmd
	go func() {
		_ = thisProc.Wait()
		mu.Lock()
		defer mu.Unlock()
		if proc == thisProc {
			proc = nil
		}
	}()
}

// killCaffeinateLocked terminates the current caffeinate process via SIGKILL
// (immediate, vs SIGTERM which can be delayed) and clears the reference.
// Caller must hold mu. Idempotent — safe when proc is nil.
//
// We null `proc` *before* calling Kill so the monitor goroutine started in
// spawnCaffeinateLocked sees `proc != thisProc` when its Wait returns and
// doesn't trample a freshly-spawned successor.
func killCaffeinateLocked() {
	if proc == nil {
		return
	}
	target := proc
	proc = nil
	if target.Process != nil {
		_ = target.Process.Kill()
	}
	logger.DebugCF("preventsleep", "caffeinate stopped", nil)
}
