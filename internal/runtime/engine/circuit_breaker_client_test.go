package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// mockClient implements model.Client for testing.
type mockClient struct {
	stream model.Stream
	err    error
}

func (m *mockClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	return m.stream, m.err
}

func TestCircuitBreakerClient_AllowsWhenClosed(t *testing.T) {
	breaker := model.NewCircuitBreaker(model.CircuitBreakerSettings{
		FailureThreshold: 5,
	})
	client := &mockClient{stream: make(model.Stream)}
	cbClient := NewCircuitBreakerClient(client, "test", breaker)

	stream, err := cbClient.Stream(context.Background(), model.Request{})
	if err != nil {
		t.Fatalf("Stream() error = %v, want nil", err)
	}
	if stream == nil {
		t.Fatal("Stream() returned nil stream")
	}
	if breaker.State() != model.CircuitBreakerClosed {
		t.Errorf("breaker state = %s, want closed", breaker.State())
	}
}

func TestCircuitBreakerClient_RecordsSuccess(t *testing.T) {
	breaker := model.NewCircuitBreaker(model.CircuitBreakerSettings{
		FailureThreshold: 2,
	})
	client := &mockClient{stream: make(model.Stream)}
	cbClient := NewCircuitBreakerClient(client, "test", breaker)

	_, _ = cbClient.Stream(context.Background(), model.Request{})
	if breaker.FailureCount() != 0 {
		t.Errorf("failure count after success = %d, want 0", breaker.FailureCount())
	}
}

func TestCircuitBreakerClient_RecordsRetriableFailure(t *testing.T) {
	breaker := model.NewCircuitBreaker(model.CircuitBreakerSettings{
		FailureThreshold: 2,
	})
	client := &mockClient{err: errors.New("timeout")}
	cbClient := NewCircuitBreakerClient(client, "test", breaker)

	_, err := cbClient.Stream(context.Background(), model.Request{})
	if err == nil {
		t.Fatal("Stream() expected error, got nil")
	}
	if breaker.FailureCount() != 1 {
		t.Errorf("failure count after retriable error = %d, want 1", breaker.FailureCount())
	}
}

func TestCircuitBreakerClient_SkipsNonRetriableFailure(t *testing.T) {
	breaker := model.NewCircuitBreaker(model.CircuitBreakerSettings{
		FailureThreshold: 1,
	})
	// "permanent error" does not match isRetriableError keywords.
	client := &mockClient{err: errors.New("permanent error")}
	cbClient := NewCircuitBreakerClient(client, "test", breaker)

	_, err := cbClient.Stream(context.Background(), model.Request{})
	if err == nil {
		t.Fatal("Stream() expected error, got nil")
	}
	if breaker.State() != model.CircuitBreakerClosed {
		t.Errorf("breaker state after non-retriable error = %s, want closed", breaker.State())
	}
}

func TestCircuitBreakerClient_RejectsWhenOpen(t *testing.T) {
	breaker := model.NewCircuitBreaker(model.CircuitBreakerSettings{
		FailureThreshold: 1,
	})
	client := &mockClient{err: errors.New("timeout")}
	cbClient := NewCircuitBreakerClient(client, "test", breaker)

	// Trip the breaker.
	_, _ = cbClient.Stream(context.Background(), model.Request{})
	if breaker.State() != model.CircuitBreakerOpen {
		t.Fatalf("breaker state = %s, want open", breaker.State())
	}

	// Next call should be rejected without touching the underlying client.
	_, err := cbClient.Stream(context.Background(), model.Request{})
	if err == nil {
		t.Fatal("Stream() expected error when open, got nil")
	}
	var cbErr *model.CircuitBreakerOpenError
	if !errors.As(err, &cbErr) {
		t.Fatalf("error type = %T, want *model.CircuitBreakerOpenError", err)
	}
	if cbErr.Provider != "test" {
		t.Errorf("error provider = %q, want test", cbErr.Provider)
	}
}

func TestCircuitBreakerClient_Unwrap(t *testing.T) {
	client := &mockClient{}
	breaker := model.NewCircuitBreaker(model.DefaultCircuitBreakerSettings())
	cbClient := NewCircuitBreakerClient(client, "test", breaker)

	if cbClient.Unwrap() != client {
		t.Error("Unwrap() returned different client")
	}
}

func TestCircuitBreakerClient_Name(t *testing.T) {
	client := &mockClient{}
	breaker := model.NewCircuitBreaker(model.DefaultCircuitBreakerSettings())
	cbClient := NewCircuitBreakerClient(client, "anthropic", breaker)

	if cbClient.Name() != "anthropic" {
		t.Errorf("Name() = %q, want anthropic", cbClient.Name())
	}
}
