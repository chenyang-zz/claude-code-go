package repl

import (
	"sync"
	"time"
)

const (
	// MinWakeupDelaySeconds is the minimum allowed delay for ScheduleWakeup.
	MinWakeupDelaySeconds = 60
	// MaxWakeupDelaySeconds is the maximum allowed delay for ScheduleWakeup.
	MaxWakeupDelaySeconds = 3600
)

// WakeupRequest holds the parameters of a pending schedule wakeup.
type WakeupRequest struct {
	DelaySeconds int
	Reason       string
	Prompt       string
	FireAt       time.Time
}

// WakeupScheduler manages a single pending schedule-wakeup in the REPL. It
// supports last-write-wins semantics: a new Schedule call cancels and replaces
// any previously scheduled wakeup. The fire channel delivers the prompt string
// when the timer expires.
type WakeupScheduler struct {
	mu      sync.Mutex
	timer   *time.Timer
	pending *WakeupRequest
	fireCh  chan string
	stopped bool
}

// NewWakeupScheduler creates a WakeupScheduler with a buffered fire channel
// (capacity 1) to avoid blocking the timer goroutine.
func NewWakeupScheduler() *WakeupScheduler {
	return &WakeupScheduler{
		fireCh: make(chan string, 1),
	}
}

// Schedule sets or replaces the pending wakeup. delaySeconds is expected to be
// pre-clamped to [MinWakeupDelaySeconds, MaxWakeupDelaySeconds] by the caller
// (the ScheduleWakeupTool). If a wakeup was already pending it is cancelled
// and replaced.
func (w *WakeupScheduler) Schedule(delaySeconds int, reason, prompt string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopped {
		return
	}

	w.cancelLocked()

	req := &WakeupRequest{
		DelaySeconds: delaySeconds,
		Reason:       reason,
		Prompt:       prompt,
		FireAt:       time.Now().Add(time.Duration(delaySeconds) * time.Second),
	}
	w.pending = req

	w.timer = time.AfterFunc(time.Duration(delaySeconds)*time.Second, func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.stopped {
			return
		}
		// Non-blocking send; if the channel already has an item, drain and
		// replace.
		select {
		case <-w.fireCh:
		default:
		}
		w.fireCh <- req.Prompt
		w.pending = nil
	})
}

// FireChan returns a receive-only channel that delivers the prompt string when
// a scheduled wakeup fires. Callers should select on this channel to be
// notified of pending wakeups.
func (w *WakeupScheduler) FireChan() <-chan string {
	return w.fireCh
}

// Pending returns the currently scheduled wakeup request, or nil if none is
// pending.
func (w *WakeupScheduler) Pending() *WakeupRequest {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending == nil {
		return nil
	}
	cp := *w.pending
	return &cp
}

// Cancel removes the pending wakeup without firing.
func (w *WakeupScheduler) Cancel() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cancelLocked()
}

// Stop shuts down the scheduler and releases resources. After Stop, further
// Schedule calls are no-ops.
func (w *WakeupScheduler) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stopped = true
	w.cancelLocked()
	close(w.fireCh)
}

// cancelLocked cancels the pending timer and clears state. Caller must hold
// w.mu.
func (w *WakeupScheduler) cancelLocked() {
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	w.pending = nil
}

// ClampWakeupDelay clamps the delay to the [MinWakeupDelaySeconds, MaxWakeupDelaySeconds] range.
func ClampWakeupDelay(seconds int) int {
	if seconds < MinWakeupDelaySeconds {
		return MinWakeupDelaySeconds
	}
	if seconds > MaxWakeupDelaySeconds {
		return MaxWakeupDelaySeconds
	}
	return seconds
}
