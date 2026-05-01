package model

import (
	"sync"
	"testing"
	"time"
)

func TestDefaultCircuitBreakerSettings(t *testing.T) {
	s := DefaultCircuitBreakerSettings()
	if s.FailureThreshold != 5 {
		t.Errorf("FailureThreshold = %d, want 5", s.FailureThreshold)
	}
	if s.RecoveryTimeout != 30*time.Second {
		t.Errorf("RecoveryTimeout = %v, want 30s", s.RecoveryTimeout)
	}
	if s.HalfOpenMaxRequests != 1 {
		t.Errorf("HalfOpenMaxRequests = %d, want 1", s.HalfOpenMaxRequests)
	}
	if s.HalfOpenSuccessThreshold != 1 {
		t.Errorf("HalfOpenSuccessThreshold = %d, want 1", s.HalfOpenSuccessThreshold)
	}
}

func TestNewCircuitBreakerDefaults(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerSettings{})
	if cb.State() != CircuitBreakerClosed {
		t.Errorf("initial state = %s, want closed", cb.State())
	}
	if cb.settings.FailureThreshold != 5 {
		t.Errorf("defaulted FailureThreshold = %d, want 5", cb.settings.FailureThreshold)
	}
}

func TestCircuitBreakerClosedToOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerSettings{
		FailureThreshold:         3,
		RecoveryTimeout:          1 * time.Second,
		HalfOpenMaxRequests:      1,
		HalfOpenSuccessThreshold: 1,
	})

	for i := 0; i < 2; i++ {
		cb.RecordFailure()
		if cb.State() != CircuitBreakerClosed {
			t.Fatalf("state after %d failures = %s, want closed", i+1, cb.State())
		}
	}

	cb.RecordFailure()
	if cb.State() != CircuitBreakerOpen {
		t.Errorf("state after threshold failures = %s, want open", cb.State())
	}
	if cb.CanExecute() {
		t.Error("CanExecute() = true, want false when open")
	}
}

func TestCircuitBreakerOpenToHalfOpenToClosed(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerSettings{
		FailureThreshold:         1,
		RecoveryTimeout:          50 * time.Millisecond,
		HalfOpenMaxRequests:      1,
		HalfOpenSuccessThreshold: 1,
	})

	cb.RecordFailure()
	if cb.State() != CircuitBreakerOpen {
		t.Fatalf("state = %s, want open", cb.State())
	}
	if cb.CanExecute() {
		t.Error("CanExecute() = true immediately after open, want false")
	}

	time.Sleep(60 * time.Millisecond)

	if !cb.CanExecute() {
		t.Error("CanExecute() = false after recovery timeout, want true (half-open)")
	}
	if cb.State() != CircuitBreakerHalfOpen {
		t.Errorf("state after timeout = %s, want half-open", cb.State())
	}

	cb.RecordSuccess()
	if cb.State() != CircuitBreakerClosed {
		t.Errorf("state after success in half-open = %s, want closed", cb.State())
	}
	if !cb.CanExecute() {
		t.Error("CanExecute() = false after recovery, want true")
	}
}

func TestCircuitBreakerHalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerSettings{
		FailureThreshold:         1,
		RecoveryTimeout:          50 * time.Millisecond,
		HalfOpenMaxRequests:      1,
		HalfOpenSuccessThreshold: 1,
	})

	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	if !cb.CanExecute() {
		t.Fatal("CanExecute() = false, want true in half-open")
	}
	cb.RecordFailure()
	if cb.State() != CircuitBreakerOpen {
		t.Errorf("state after half-open failure = %s, want open", cb.State())
	}
}

func TestCircuitBreakerSuccessResetsFailures(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerSettings{
		FailureThreshold:         3,
		RecoveryTimeout:          1 * time.Second,
		HalfOpenMaxRequests:      1,
		HalfOpenSuccessThreshold: 1,
	})

	cb.RecordFailure()
	cb.RecordFailure()
	if cb.FailureCount() != 2 {
		t.Fatalf("failure count = %d, want 2", cb.FailureCount())
	}

	cb.RecordSuccess()
	if cb.FailureCount() != 0 {
		t.Errorf("failure count after success = %d, want 0", cb.FailureCount())
	}
	if cb.State() != CircuitBreakerClosed {
		t.Errorf("state after success = %s, want closed", cb.State())
	}
}

func TestCircuitBreakerHalfOpenMaxRequests(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerSettings{
		FailureThreshold:         1,
		RecoveryTimeout:          50 * time.Millisecond,
		HalfOpenMaxRequests:      2,
		HalfOpenSuccessThreshold: 2,
	})

	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	if !cb.CanExecute() {
		t.Fatal("first half-open request rejected")
	}
	if !cb.CanExecute() {
		t.Fatal("second half-open request rejected")
	}
	if cb.CanExecute() {
		t.Error("third half-open request accepted, want rejected")
	}
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerSettings{
		FailureThreshold:         100,
		RecoveryTimeout:          1 * time.Second,
		HalfOpenMaxRequests:      1,
		HalfOpenSuccessThreshold: 1,
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.CanExecute()
			cb.RecordFailure()
			cb.RecordSuccess()
		}()
	}
	wg.Wait()

	// After concurrent failures and successes, the breaker should still be in a
	// valid state. Because successes and failures race, we only assert that the
	// state is one of the valid states.
	s := cb.State()
	if s != CircuitBreakerClosed && s != CircuitBreakerOpen && s != CircuitBreakerHalfOpen {
		t.Errorf("invalid state after concurrency: %s", s)
	}
}

func TestCircuitBreakerOpenError(t *testing.T) {
	err := &CircuitBreakerOpenError{Provider: "anthropic"}
	if err.Error() != "circuit breaker open for provider anthropic" {
		t.Errorf("error message = %q, want circuit breaker open for provider anthropic", err.Error())
	}
}
