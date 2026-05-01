package oauth

import (
	"sync"
)

// refreshInFlight tracks a single in-flight refresh attempt. Concurrent
// callers waiting on the same key block on `done`, then read the cached
// result and error.
type refreshInFlight struct {
	done   chan struct{}
	result *OAuthTokens
	err    error
}

// RefreshDeduper coordinates concurrent OAuth refresh attempts so that a
// single refresh per key actually exchanges with the OAuth server even when
// multiple goroutines observe the same expired token simultaneously.
//
// It mirrors the TS-side `pending401Handlers` map keyed by the failed access
// token (src/utils/auth.ts:1343) and the `pendingRefreshCheck` singleton that
// guards the preemptive path (src/utils/auth.ts:1425). Both paths share the
// same dedup map here, distinguished only by their key.
type RefreshDeduper struct {
	mu      sync.Mutex
	pending map[string]*refreshInFlight
}

// NewRefreshDeduper creates an empty RefreshDeduper.
func NewRefreshDeduper() *RefreshDeduper {
	return &RefreshDeduper{
		pending: make(map[string]*refreshInFlight),
	}
}

// Do invokes fn at most once concurrently per key. Concurrent callers using
// the same key wait for the in-flight call to finish and observe the same
// result and error. Calls with different keys proceed independently.
//
// The supplied fn must be safe to invoke without holding any external lock;
// it is called with the deduper's internal lock released so that nested
// refresher operations (token endpoint round-trips, store writes) cannot
// deadlock with sibling callers.
func (d *RefreshDeduper) Do(key string, fn func() (*OAuthTokens, error)) (*OAuthTokens, error) {
	d.mu.Lock()
	if existing, ok := d.pending[key]; ok {
		d.mu.Unlock()
		<-existing.done
		return existing.result, existing.err
	}
	flight := &refreshInFlight{
		done: make(chan struct{}),
	}
	d.pending[key] = flight
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.pending, key)
		d.mu.Unlock()
		close(flight.done)
	}()

	flight.result, flight.err = fn()
	return flight.result, flight.err
}

// PendingCount returns the number of in-flight refreshes currently tracked.
// Exposed primarily for tests; callers should not rely on it for control
// flow because the count can change between observation and use.
func (d *RefreshDeduper) PendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.pending)
}
