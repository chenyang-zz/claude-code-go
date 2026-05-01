package model

import (
	"context"
	"testing"
	"time"
)

// mockProviderHealth is a test double for ProviderHealth.
type mockProviderHealth struct {
	result HealthResult
}

func (m *mockProviderHealth) Check(_ context.Context) HealthResult {
	return m.result
}

func TestNewHealthChecker(t *testing.T) {
	hc := NewHealthChecker()
	if hc == nil {
		t.Fatal("NewHealthChecker() returned nil")
	}
	if hc.checkers == nil {
		t.Fatal("checkers map is nil")
	}
}

func TestHealthChecker_Register(t *testing.T) {
	hc := NewHealthChecker()
	mock := &mockProviderHealth{result: HealthResult{Provider: "test", Status: HealthStatusHealthy}}

	hc.Register("test", mock)
	if hc.Get("test") == nil {
		t.Fatal("Register() did not store the probe")
	}

	// Overwrite should work.
	mock2 := &mockProviderHealth{result: HealthResult{Provider: "test", Status: HealthStatusDegraded}}
	hc.Register("test", mock2)
	got := hc.Get("test").Check(context.Background())
	if got.Status != HealthStatusDegraded {
		t.Fatalf("overwrite failed: got %q, want degraded", got.Status)
	}
}

func TestHealthChecker_CheckAll(t *testing.T) {
	hc := NewHealthChecker()
	hc.Register("a", &mockProviderHealth{result: HealthResult{
		Provider: "a", Status: HealthStatusHealthy, Message: "ok", CheckedAt: time.Now(),
	}})
	hc.Register("b", &mockProviderHealth{result: HealthResult{
		Provider: "b", Status: HealthStatusUnhealthy, Message: "down", CheckedAt: time.Now(),
	}})

	results := hc.CheckAll(context.Background())
	if len(results) != 2 {
		t.Fatalf("CheckAll() returned %d results, want 2", len(results))
	}

	// Results may be in any order due to concurrency; map them for assertion.
	byProvider := make(map[string]HealthResult)
	for _, r := range results {
		byProvider[r.Provider] = r
	}

	if byProvider["a"].Status != HealthStatusHealthy {
		t.Fatalf("provider a status = %q, want healthy", byProvider["a"].Status)
	}
	if byProvider["b"].Status != HealthStatusUnhealthy {
		t.Fatalf("provider b status = %q, want unhealthy", byProvider["b"].Status)
	}
}

func TestHealthChecker_CheckAll_Empty(t *testing.T) {
	hc := NewHealthChecker()
	results := hc.CheckAll(context.Background())
	if results != nil {
		t.Fatalf("CheckAll() on empty checker = %v, want nil", results)
	}
}

func TestHealthChecker_Get_Missing(t *testing.T) {
	hc := NewHealthChecker()
	if hc.Get("missing") != nil {
		t.Fatal("Get() for missing provider should return nil")
	}
}

func TestHealthChecker_RegisterOnNilMap(t *testing.T) {
	hc := &HealthChecker{}
	mock := &mockProviderHealth{result: HealthResult{Provider: "test", Status: HealthStatusHealthy}}
	hc.Register("test", mock)
	if hc.Get("test") == nil {
		t.Fatal("Register() should initialise nil checkers map")
	}
}
