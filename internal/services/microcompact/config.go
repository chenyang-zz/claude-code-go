package microcompact

// TimeBasedMCConfig holds the configuration for time-based micro-compaction.
// When the gap since the last main-loop assistant message exceeds the threshold,
// older tool results are content-cleared to shrink the prompt sent to the API.
// Aligns with TS TimeBasedMCConfig (src/services/compact/timeBasedMCConfig.ts).
type TimeBasedMCConfig struct {
	// Enabled is the master switch. When false, time-based microcompact is a no-op.
	Enabled bool

	// GapThresholdMinutes triggers content-clearing when (now - last assistant timestamp)
	// exceeds this many minutes. 60 is the safe choice: the server's 1h cache TTL is
	// guaranteed expired for all users, so we never force a miss that wouldn't have happened.
	GapThresholdMinutes int

	// KeepRecent keeps this many most-recent compactable tool results.
	// Older results are cleared. Default 5, floored at 1.
	KeepRecent int
}

// DefaultTimeBasedMCConfig returns the default time-based microcompact configuration.
// GrowthBook defaults are replaced by feature-flag-gated static values.
// Note: The Enabled field is not checked at runtime — service creation is gated
// by the MICRO_COMPACT feature flag at init time.
func DefaultTimeBasedMCConfig() TimeBasedMCConfig {
	return TimeBasedMCConfig{
		Enabled:             true,
		GapThresholdMinutes: 60,
		KeepRecent:          5,
	}
}
