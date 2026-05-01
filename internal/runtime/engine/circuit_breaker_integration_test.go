package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

func TestTryFallback_CircuitBreakerOpenErrorTriggersFallback(t *testing.T) {
	primaryBreaker := model.NewCircuitBreaker(model.CircuitBreakerSettings{
		FailureThreshold: 1,
	})
	primaryBreaker.RecordFailure() // trip to open

	primary := NewCircuitBreakerClient(&fakeModelClient{}, "primary", primaryBreaker)
	fallbackClient := &fakeModelClient{streamFn: func(_ context.Context, _ model.Request) (model.Stream, error) {
		return make(model.Stream), nil
	}}

	e := &Runtime{
		Client:          primary,
		FallbackClients: []model.Client{fallbackClient},
	}

	fb := e.tryFallback(context.Background(), model.Request{}, &model.CircuitBreakerOpenError{Provider: "primary"})
	if fb == nil {
		t.Fatal("tryFallback() returned nil, want fallback result when primary breaker is open")
	}
	if fb.model != "fallback-client-0" {
		t.Errorf("fallback model = %q, want fallback-client-0", fb.model)
	}
}

func TestTryFallback_SkipsOpenFallbackClients(t *testing.T) {
	clientA := NewCircuitBreakerClient(
		&fakeModelClient{streamFn: func(_ context.Context, _ model.Request) (model.Stream, error) {
			return make(model.Stream), nil
		}},
		"client-a",
		func() *model.CircuitBreaker {
			cb := model.NewCircuitBreaker(model.CircuitBreakerSettings{FailureThreshold: 1})
			cb.RecordFailure()
			return cb
		}(),
	)
	clientB := &fakeModelClient{streamFn: func(_ context.Context, _ model.Request) (model.Stream, error) {
		return make(model.Stream), nil
	}}

	e := &Runtime{
		Client:          &fakeModelClient{},
		FallbackClients: []model.Client{clientA, clientB},
	}

	fb := e.tryFallback(context.Background(), model.Request{}, errors.New("timeout"))
	if fb == nil {
		t.Fatal("tryFallback() returned nil, want fallback result from client B")
	}
	if fb.model != "fallback-client-1" {
		t.Errorf("fallback model = %q, want fallback-client-1", fb.model)
	}
}

func TestTryFallback_SkipsPrimaryWhenCircuitBreakerOpen(t *testing.T) {
	primaryBreaker := model.NewCircuitBreaker(model.CircuitBreakerSettings{FailureThreshold: 1})
	primaryBreaker.RecordFailure()

	// primary is a CircuitBreakerClient that is open.
	primary := NewCircuitBreakerClient(&fakeModelClient{}, "primary", primaryBreaker)

	// fallbackModel is configured on the same Client (primary), which is open.
	// tryFallback should skip the FallbackModel attempt and go straight to FallbackClients.
	fallbackClient := &fakeModelClient{streamFn: func(_ context.Context, _ model.Request) (model.Stream, error) {
		return make(model.Stream), nil
	}}

	e := &Runtime{
		Client:          primary,
		FallbackModel:   "fallback-model",
		FallbackClients: []model.Client{fallbackClient},
	}

	fb := e.tryFallback(context.Background(), model.Request{}, &model.CircuitBreakerOpenError{Provider: "primary"})
	if fb == nil {
		t.Fatal("tryFallback() returned nil, want fallback result")
	}
	// Should come from fallbackClients, not FallbackModel (which uses the same open client).
	if fb.model != "fallback-client-0" {
		t.Errorf("fallback model = %q, want fallback-client-0", fb.model)
	}
}

func TestTryFallback_AllFallbackClientsOpen(t *testing.T) {
	clientA := NewCircuitBreakerClient(&fakeModelClient{}, "a", func() *model.CircuitBreaker {
		cb := model.NewCircuitBreaker(model.CircuitBreakerSettings{FailureThreshold: 1})
		cb.RecordFailure()
		return cb
	}())
	clientB := NewCircuitBreakerClient(&fakeModelClient{}, "b", func() *model.CircuitBreaker {
		cb := model.NewCircuitBreaker(model.CircuitBreakerSettings{FailureThreshold: 1})
		cb.RecordFailure()
		return cb
	}())

	e := &Runtime{
		Client:          &fakeModelClient{},
		FallbackClients: []model.Client{clientA, clientB},
	}

	fb := e.tryFallback(context.Background(), model.Request{}, errors.New("timeout"))
	if fb != nil {
		t.Fatal("tryFallback() should return nil when all fallback clients are open")
	}
}
