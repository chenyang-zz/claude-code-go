package growthbook

import (
	"sync"
)

// signal manages a set of listeners that get notified when a refresh occurs.
type signal struct {
	mu        sync.RWMutex
	listeners []GrowthBookRefreshListener
}

// subscribe adds a listener and returns an unsubscribe function.
func (s *signal) subscribe(listener GrowthBookRefreshListener) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, listener)

	idx := len(s.listeners) - 1
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.listeners[idx] = nil
	}
}

// emit notifies all registered listeners.
func (s *signal) emit() {
	s.mu.RLock()
	listeners := make([]GrowthBookRefreshListener, len(s.listeners))
	copy(listeners, s.listeners)
	s.mu.RUnlock()

	for _, l := range listeners {
		if l != nil {
			l()
		}
	}
}

// refreshed is the package-level signal for GrowthBook refresh notifications.
var refreshed signal

// OnRefresh registers a callback to fire when GrowthBook feature values refresh.
// Returns an unsubscribe function.
func OnRefresh(listener GrowthBookRefreshListener) func() {
	return refreshed.subscribe(listener)
}
