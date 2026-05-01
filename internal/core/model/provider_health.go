package model

import (
	"context"
	"sync"
	"time"
)

// HealthStatus represents the health state of a model provider.
type HealthStatus string

const (
	// HealthStatusHealthy means the provider responded successfully.
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusDegraded means the provider responded but with warnings.
	HealthStatusDegraded HealthStatus = "degraded"
	// HealthStatusUnhealthy means the provider failed to respond.
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	// HealthStatusUnknown means the provider has not been checked yet.
	HealthStatusUnknown HealthStatus = "unknown"
	// HealthStatusNotConfigured means the provider is not configured for this session.
	HealthStatusNotConfigured HealthStatus = "not_configured"
)

// HealthResult carries the outcome of a single provider health check.
type HealthResult struct {
	// Provider is the normalized provider identifier (e.g. "anthropic", "openai-compatible").
	Provider string
	// Status is the health state.
	Status HealthStatus
	// Message is a human-readable summary of the check result.
	Message string
	// CheckedAt is the time the check was performed.
	CheckedAt time.Time
	// ErrorKind is the classified error type when the check failed.
	// Empty when the check succeeded or the error could not be classified.
	ErrorKind ProviderErrorKind
}

// ProviderHealth is implemented by components that can check the runtime
// health of a model provider.
type ProviderHealth interface {
	// Check probes the provider and returns its current health.
	Check(ctx context.Context) HealthResult
}

// HealthChecker holds a collection of ProviderHealth checkers keyed by
// provider identifier and can run checks in parallel.
type HealthChecker struct {
	// checkers maps normalized provider names to their health probes.
	checkers map[string]ProviderHealth
}

// NewHealthChecker builds an empty HealthChecker.
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checkers: make(map[string]ProviderHealth),
	}
}

// Register adds a provider health probe. If a probe for the same provider
// already exists it is overwritten.
func (h *HealthChecker) Register(provider string, ph ProviderHealth) {
	if h.checkers == nil {
		h.checkers = make(map[string]ProviderHealth)
	}
	h.checkers[provider] = ph
}

// CheckAll runs every registered provider probe concurrently and returns
// the aggregated results.
func (h *HealthChecker) CheckAll(ctx context.Context) []HealthResult {
	if len(h.checkers) == 0 {
		return nil
	}

	type pair struct {
		provider string
		ph       ProviderHealth
	}
	pairs := make([]pair, 0, len(h.checkers))
	for p, ph := range h.checkers {
		pairs = append(pairs, pair{provider: p, ph: ph})
	}

	results := make([]HealthResult, len(pairs))
	var wg sync.WaitGroup
	for i, p := range pairs {
		wg.Add(1)
		go func(idx int, prov string, ph ProviderHealth) {
			defer wg.Done()
			results[idx] = ph.Check(ctx)
		}(i, p.provider, p.ph)
	}
	wg.Wait()
	return results
}

// Get returns the health probe for a specific provider, or nil if not registered.
func (h *HealthChecker) Get(provider string) ProviderHealth {
	return h.checkers[provider]
}
