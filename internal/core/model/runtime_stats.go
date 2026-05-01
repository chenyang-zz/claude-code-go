package model

import "sync/atomic"

// RuntimeStats tracks engine-level retry and fallback counters for observability.
// All methods are safe for concurrent use.
type RuntimeStats struct {
	retryCount      atomic.Int32
	fallbackCount   atomic.Int32
	cbTripCount     atomic.Int32
}

// RecordRetry increments the retry counter.
func (s *RuntimeStats) RecordRetry() {
	if s != nil {
		s.retryCount.Add(1)
	}
}

// RecordFallback increments the fallback counter.
func (s *RuntimeStats) RecordFallback() {
	if s != nil {
		s.fallbackCount.Add(1)
	}
}

// RecordCircuitBreakerTrip increments the circuit-breaker trip counter.
func (s *RuntimeStats) RecordCircuitBreakerTrip() {
	if s != nil {
		s.cbTripCount.Add(1)
	}
}

// Snapshot returns the current counter values.
func (s *RuntimeStats) Snapshot() (retryCount, fallbackCount, cbTripCount int) {
	if s == nil {
		return 0, 0, 0
	}
	return int(s.retryCount.Load()), int(s.fallbackCount.Load()), int(s.cbTripCount.Load())
}
