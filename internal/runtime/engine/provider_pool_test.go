package engine

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// fakeClient implements model.Client with controllable behavior for testing.
type fakeClient struct {
	name       string
	shouldFail atomic.Bool
	failCount  atomic.Int32
	callCount  atomic.Int32
}

func (f *fakeClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	f.callCount.Add(1)
	if f.shouldFail.Load() {
		f.failCount.Add(1)
		return nil, errors.New(f.name + ": stream error")
	}
	ch := make(chan model.Event)
	close(ch)
	return ch, nil
}

func TestProviderPool_EmptyPoolReturnsError(t *testing.T) {
	pool := NewProviderPool(nil, nil, LoadBalancePrimaryWithFailover)
	_, err := pool.Stream(context.Background(), model.Request{})
	if err == nil {
		t.Fatal("expected error for empty pool")
	}
}

func TestProviderPool_SingleClientDelegatesDirectly(t *testing.T) {
	fc := &fakeClient{name: "single"}
	pool := NewProviderPool(
		[]model.Client{fc},
		[]string{"single"},
		LoadBalancePrimaryWithFailover,
	)

	stream, err := pool.Stream(context.Background(), model.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream == nil {
		t.Fatal("expected non-nil stream")
	}
	if calls := fc.callCount.Load(); calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestProviderPool_FailoverSkipsCircuitBreakerOpen(t *testing.T) {
	fc1 := &fakeClient{name: "broken"}
	fc2 := &fakeClient{name: "ok"}

	// Wrap fc1 in a breaker already tripped open
	breaker := model.NewCircuitBreaker(model.CircuitBreakerSettings{
		FailureThreshold:         1,
		RecoveryTimeout:          999999, // effectively never recovers
		HalfOpenMaxRequests:      1,
		HalfOpenSuccessThreshold: 1,
	})
	breaker.RecordFailure() // trips breaker to open

	cbClient := NewCircuitBreakerClient(fc1, "broken", breaker)

	pool := NewProviderPool(
		[]model.Client{cbClient, fc2},
		[]string{"primary", "fallback"},
		LoadBalancePrimaryWithFailover,
	)

	stream, err := pool.Stream(context.Background(), model.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream == nil {
		t.Fatal("expected non-nil stream")
	}

	if calls := fc1.callCount.Load(); calls != 0 {
		t.Fatalf("expected 0 calls to broken client (rejected by breaker), got %d", calls)
	}
	if calls := fc2.callCount.Load(); calls != 1 {
		t.Fatalf("expected 1 call to fallback client, got %d", calls)
	}
}

func TestProviderPool_FailoverAllBrokenReturnsLastError(t *testing.T) {
	fc1 := &fakeClient{name: "broken1"}
	fc2 := &fakeClient{name: "broken2"}

	breaker1 := model.NewCircuitBreaker(model.CircuitBreakerSettings{
		FailureThreshold:         1,
		RecoveryTimeout:          999999,
		HalfOpenMaxRequests:      1,
		HalfOpenSuccessThreshold: 1,
	})
	breaker1.RecordFailure()

	breaker2 := model.NewCircuitBreaker(model.CircuitBreakerSettings{
		FailureThreshold:         1,
		RecoveryTimeout:          999999,
		HalfOpenMaxRequests:      1,
		HalfOpenSuccessThreshold: 1,
	})
	breaker2.RecordFailure()

	pool := NewProviderPool(
		[]model.Client{
			NewCircuitBreakerClient(fc1, "broken1", breaker1),
			NewCircuitBreakerClient(fc2, "broken2", breaker2),
		},
		[]string{"p1", "p2"},
		LoadBalancePrimaryWithFailover,
	)

	_, err := pool.Stream(context.Background(), model.Request{})
	if err == nil {
		t.Fatal("expected error when all providers circuit-broken")
	}
}

func TestProviderPool_FailoverOnStreamError(t *testing.T) {
	fc1 := &fakeClient{name: "failing"}
	fc1.shouldFail.Store(true)
	fc2 := &fakeClient{name: "working"}

	pool := NewProviderPool(
		[]model.Client{fc1, fc2},
		[]string{"primary", "fallback"},
		LoadBalancePrimaryWithFailover,
	)

	stream, err := pool.Stream(context.Background(), model.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream == nil {
		t.Fatal("expected non-nil stream")
	}

	if calls := fc1.callCount.Load(); calls != 1 {
		t.Fatalf("expected 1 call to failing client, got %d", calls)
	}
	if calls := fc2.callCount.Load(); calls != 1 {
		t.Fatalf("expected 1 call to working fallback client, got %d", calls)
	}
}

func TestProviderPool_RoundRobinDistributes(t *testing.T) {
	fc1 := &fakeClient{name: "a"}
	fc2 := &fakeClient{name: "b"}
	fc3 := &fakeClient{name: "c"}

	pool := NewProviderPool(
		[]model.Client{fc1, fc2, fc3},
		[]string{"a", "b", "c"},
		LoadBalanceRoundRobin,
	)

	for range 6 {
		_, err := pool.Stream(context.Background(), model.Request{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if calls := fc1.callCount.Load(); calls != 2 {
		t.Fatalf("expected 2 calls to client a, got %d", calls)
	}
	if calls := fc2.callCount.Load(); calls != 2 {
		t.Fatalf("expected 2 calls to client b, got %d", calls)
	}
	if calls := fc3.callCount.Load(); calls != 2 {
		t.Fatalf("expected 2 calls to client c, got %d", calls)
	}
}

func TestProviderPool_FirstClientSucceedsNoFallback(t *testing.T) {
	fc1 := &fakeClient{name: "primary"}
	fc2 := &fakeClient{name: "fallback"}

	pool := NewProviderPool(
		[]model.Client{fc1, fc2},
		[]string{"primary", "fallback"},
		LoadBalancePrimaryWithFailover,
	)

	_, err := pool.Stream(context.Background(), model.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := fc1.callCount.Load(); calls != 1 {
		t.Fatalf("expected 1 call to primary client, got %d", calls)
	}
	if calls := fc2.callCount.Load(); calls != 0 {
		t.Fatalf("expected 0 calls to fallback client, got %d", calls)
	}
}

func TestProviderPool_NumProviders(t *testing.T) {
	fc1 := &fakeClient{}
	fc2 := &fakeClient{}
	pool := NewProviderPool(
		[]model.Client{fc1, fc2},
		[]string{"a", "b"},
		LoadBalancePrimaryWithFailover,
	)
	if n := pool.NumProviders(); n != 2 {
		t.Fatalf("expected 2 providers, got %d", n)
	}
}

func TestProviderPool_SkipsNilClients(t *testing.T) {
	pool := NewProviderPool(
		[]model.Client{nil, nil},
		[]string{"a", "b"},
		LoadBalancePrimaryWithFailover,
	)
	if n := pool.NumProviders(); n != 0 {
		t.Fatalf("expected 0 providers (all nil), got %d", n)
	}
	_, err := pool.Stream(context.Background(), model.Request{})
	if err == nil {
		t.Fatal("expected error for empty pool after nil clients skipped")
	}
}
