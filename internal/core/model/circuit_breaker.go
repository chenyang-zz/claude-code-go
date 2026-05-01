package model

import (
	"fmt"
	"sync"
	"time"
)

// CircuitBreakerState represents the three states of a circuit breaker.
type CircuitBreakerState string

const (
	// CircuitBreakerClosed means requests flow normally.
	// Failures are counted; reaching the threshold trips the breaker to Open.
	CircuitBreakerClosed CircuitBreakerState = "closed"
	// CircuitBreakerOpen means requests are rejected immediately.
	// After RecoveryTimeout the breaker transitions to HalfOpen.
	CircuitBreakerOpen CircuitBreakerState = "open"
	// CircuitBreakerHalfOpen means a limited number of trial requests are allowed.
	// Successes restore Closed; a single failure reopens.
	CircuitBreakerHalfOpen CircuitBreakerState = "half-open"
)

// CircuitBreakerSettings configures the thresholds and timeouts for a circuit breaker.
type CircuitBreakerSettings struct {
	// FailureThreshold is the number of consecutive failures required to trip
	// the breaker from Closed to Open. Must be > 0.
	FailureThreshold int
	// RecoveryTimeout is the duration the breaker stays Open before allowing
	// trial requests (transition to HalfOpen). Must be > 0.
	RecoveryTimeout time.Duration
	// HalfOpenMaxRequests is the maximum number of trial requests permitted
	// while in HalfOpen state. Must be > 0.
	HalfOpenMaxRequests int
	// HalfOpenSuccessThreshold is the number of consecutive successes required
	// in HalfOpen state to transition back to Closed. Must be > 0.
	HalfOpenSuccessThreshold int
}

// DefaultCircuitBreakerSettings returns sensible defaults.
func DefaultCircuitBreakerSettings() CircuitBreakerSettings {
	return CircuitBreakerSettings{
		FailureThreshold:         5,
		RecoveryTimeout:          30 * time.Second,
		HalfOpenMaxRequests:      1,
		HalfOpenSuccessThreshold: 1,
	}
}

// CircuitBreaker implements the classic three-state circuit-breaker pattern.
// It wraps around a model of execution and rejects requests when the failure
// rate exceeds a threshold, automatically recovering after a timeout.
type CircuitBreaker struct {
	settings CircuitBreakerSettings

	mu              sync.RWMutex
	state           CircuitBreakerState
	failures        int
	lastFailureTime time.Time
	halfOpenReqs    int
	halfOpenSuccess int
	tripCount       int
}

// NewCircuitBreaker creates a new circuit breaker with the given settings.
// Zero or negative values are replaced by defaults.
func NewCircuitBreaker(settings CircuitBreakerSettings) *CircuitBreaker {
	if settings.FailureThreshold <= 0 {
		settings.FailureThreshold = DefaultCircuitBreakerSettings().FailureThreshold
	}
	if settings.RecoveryTimeout <= 0 {
		settings.RecoveryTimeout = DefaultCircuitBreakerSettings().RecoveryTimeout
	}
	if settings.HalfOpenMaxRequests <= 0 {
		settings.HalfOpenMaxRequests = DefaultCircuitBreakerSettings().HalfOpenMaxRequests
	}
	if settings.HalfOpenSuccessThreshold <= 0 {
		settings.HalfOpenSuccessThreshold = DefaultCircuitBreakerSettings().HalfOpenSuccessThreshold
	}
	return &CircuitBreaker{
		settings: settings,
		state:    CircuitBreakerClosed,
	}
}

// CanExecute reports whether the caller should proceed with a request.
// While Open, it returns false until RecoveryTimeout elapses, at which point
// it transitions to HalfOpen. In HalfOpen it permits up to HalfOpenMaxRequests
// trial requests.
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == CircuitBreakerOpen {
		if time.Since(cb.lastFailureTime) >= cb.settings.RecoveryTimeout {
			cb.state = CircuitBreakerHalfOpen
			cb.halfOpenReqs = 0
			cb.halfOpenSuccess = 0
		} else {
			return false
		}
	}

	if cb.state == CircuitBreakerHalfOpen {
		if cb.halfOpenReqs < cb.settings.HalfOpenMaxRequests {
			cb.halfOpenReqs++
			return true
		}
		return false
	}

	// CircuitBreakerClosed
	return true
}

// RecordSuccess records a successful request outcome.
// In Closed state it resets the failure counter.
// In HalfOpen state it counts toward restoring Closed.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitBreakerClosed:
		cb.failures = 0
	case CircuitBreakerHalfOpen:
		cb.halfOpenSuccess++
		if cb.halfOpenSuccess >= cb.settings.HalfOpenSuccessThreshold {
			cb.state = CircuitBreakerClosed
			cb.failures = 0
			cb.halfOpenReqs = 0
			cb.halfOpenSuccess = 0
		}
	}
}

// RecordFailure records a failed request outcome.
// In Closed state it increments the failure counter; reaching the threshold
// trips the breaker to Open. In HalfOpen state it immediately reopens.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitBreakerClosed:
		cb.failures++
		if cb.failures >= cb.settings.FailureThreshold {
			cb.state = CircuitBreakerOpen
			cb.tripCount++
		}
	case CircuitBreakerHalfOpen:
		cb.state = CircuitBreakerOpen
		cb.halfOpenReqs = 0
		cb.halfOpenSuccess = 0
		cb.tripCount++
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// FailureCount returns the number of consecutive failures in Closed state.
func (cb *CircuitBreaker) FailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// LastFailureTime returns the time of the most recent failure, or zero if none.
func (cb *CircuitBreaker) LastFailureTime() time.Time {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.lastFailureTime
}

// TripCount returns the total number of times the breaker has tripped to Open.
func (cb *CircuitBreaker) TripCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.tripCount
}

// String returns a human-readable representation of the circuit breaker state.
func (cb *CircuitBreaker) String() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return string(cb.state)
}

// CircuitBreakerOpenError is returned when a request is rejected because the
// circuit breaker is in Open state.
type CircuitBreakerOpenError struct {
	Provider string
}

// Error implements the error interface.
func (e *CircuitBreakerOpenError) Error() string {
	return fmt.Sprintf("circuit breaker open for provider %s", e.Provider)
}
