package cron

import (
	"math"
	"time"
)

// CronJitterConfig holds tuning parameters for the cron scheduler's jitter
// (deterministic per-task delay/lead) to avoid thundering-herd problems when
// many sessions schedule the same cron string.
type CronJitterConfig struct {
	// RecurringFrac is the max forward delay as a fraction of the interval
	// between consecutive fires.
	RecurringFrac float64
	// RecurringCapMs is the upper bound on recurring forward delay regardless
	// of interval length.
	RecurringCapMs int64
	// OneShotMaxMs is the maximum ms a one-shot task may fire early.
	OneShotMaxMs int64
	// OneShotFloorMs is the minimum ms of lead for one-shot tasks landing on a
	// hot minute. With floor > 0, even a task with ID hashing to 0 gets some
	// lead — nobody fires on the exact wall-clock mark.
	OneShotFloorMs int64
	// OneShotMinuteMod gates jitter: only minutes where minute % N == 0 get
	// jitter. 30 → :00/:30 (the human-rounding hotspots).
	OneShotMinuteMod int
	// RecurringMaxAgeMs is how long recurring tasks live before auto-expiry.
	// 0 means unlimited (never age out).
	RecurringMaxAgeMs int64
}

// DefaultCronJitterConfig is the default set of jitter parameters that
// matches the TypeScript side's DEFAULT_CRON_JITTER_CONFIG.
var DefaultCronJitterConfig = CronJitterConfig{
	RecurringFrac:     0.1,
	RecurringCapMs:    15 * 60 * 1000,        // 15 minutes
	OneShotMaxMs:      90 * 1000,             // 90 seconds
	OneShotFloorMs:    0,
	OneShotMinuteMod:  30,                    // jitter only :00 and :30
	RecurringMaxAgeMs: 7 * 24 * 60 * 60 * 1000, // 7 days
}

// JitteredNextCronRunMs computes the next cron fire time with a deterministic
// per-task forward delay to avoid thundering-herd when many sessions share the
// same cron string (e.g. "0 * * * *" → everyone hits inference at :00).
//
// The delay is proportional to the gap between consecutive fires
// (RecurringFrac, capped at RecurringCapMs). Only used for recurring tasks.
func JitteredNextCronRunMs(cron string, fromMs int64, taskID string, cfg CronJitterConfig) int64 {
	t1 := NextCronRunMs(cron, fromMs)
	if t1 == 0 {
		return 0
	}
	t2 := NextCronRunMs(cron, t1)
	// No second match in the next year (e.g. pinned date) → nothing to
	// proportion against. Fire on t1.
	if t2 == 0 {
		return t1
	}
	jitter := int64(math.Min(
		jitterFrac(taskID)*cfg.RecurringFrac*float64(t2-t1),
		float64(cfg.RecurringCapMs),
	))
	return t1 + jitter
}

// OneShotJitteredNextCronRunMs computes the next cron fire time minus a
// deterministic per-task lead time when the fire time lands on a minute
// boundary matching OneShotMinuteMod.
//
// One-shot tasks are user-pinned ("remind me at 3pm") so delaying them breaks
// the contract — but firing slightly early is invisible and spreads the
// inference spike. At defaults (mod 30, max 90s, floor 0) only :00 and :30
// get jitter.
//
// Clamped to fromMs so a task created inside its own jitter window doesn't
// fire before it was created.
func OneShotJitteredNextCronRunMs(cron string, fromMs int64, taskID string, cfg CronJitterConfig) int64 {
	t1 := NextCronRunMs(cron, fromMs)
	if t1 == 0 {
		return 0
	}
	// Cron resolution is 1 minute → computed times always have :00 seconds.
	// Check local minute (cron is evaluated in local time).
	t1Time := time.UnixMilli(t1)
	if t1Time.Minute()%cfg.OneShotMinuteMod != 0 {
		return t1
	}
	lead := cfg.OneShotFloorMs + int64(jitterFrac(taskID)*float64(cfg.OneShotMaxMs-cfg.OneShotFloorMs))
	if t1-lead < fromMs {
		return fromMs
	}
	return t1 - lead
}

// jitterFrac derives a deterministic [0,1) float from the task ID (8 hex chars
// from a UUID slice → u32 / 2^32). Stable across restarts, uniformly
// distributed. Non-hex ids fall back to 0 = no jitter.
func jitterFrac(taskID string) float64 {
	if len(taskID) < 8 {
		return 0
	}
	var v uint64
	for i := 0; i < 8; i++ {
		v = v<<4 | uint64(hexVal(taskID[i]))
	}
	frac := float64(v) / 0x1_0000_0000
	if math.IsNaN(frac) || math.IsInf(frac, 0) {
		return 0
	}
	return frac
}

// hexVal returns the numeric value of a hex character, or 0 if invalid.
func hexVal(c byte) uint64 {
	switch {
	case c >= '0' && c <= '9':
		return uint64(c - '0')
	case c >= 'a' && c <= 'f':
		return uint64(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return uint64(c - 'A' + 10)
	default:
		return 0
	}
}

// CronTask is the minimal task representation needed by the jitter/missed
// functions. It mirrors the fields from the shared cron task store that are
// relevant for scheduling decisions.
type CronTask struct {
	ID          string
	Cron        string
	Prompt      string
	CreatedAt   time.Time
	LastFiredAt *time.Time
	Recurring   bool
}

// FindMissedTasks returns tasks whose next scheduled run (computed from
// createdAt or lastFiredAt) falls before nowMs. A task is "missed" when its
// next fire window has already passed.
func FindMissedTasks(tasks []CronTask, nowMs int64) []CronTask {
	var missed []CronTask
	for _, t := range tasks {
		anchor := t.CreatedAt.UnixMilli()
		if t.LastFiredAt != nil {
			anchor = t.LastFiredAt.UnixMilli()
		}
		next := NextCronRunMs(t.Cron, anchor)
		if next != 0 && next < nowMs {
			missed = append(missed, t)
		}
	}
	return missed
}

// IsRecurringTaskAged returns true when a recurring task was created more than
// maxAgeMs ago and should be deleted on its next fire. maxAgeMs of 0 means
// unlimited (never ages out).
func IsRecurringTaskAged(t CronTask, nowMs int64, maxAgeMs int64) bool {
	if maxAgeMs == 0 {
		return false
	}
	if !t.Recurring {
		return false
	}
	return nowMs-t.CreatedAt.UnixMilli() >= maxAgeMs
}
