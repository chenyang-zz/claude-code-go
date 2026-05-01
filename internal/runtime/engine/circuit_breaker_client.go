package engine

import (
	"context"
	"errors"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// CircuitBreakerClient wraps a model.Client with circuit breaker protection.
// It delegates to the underlying client only when the breaker permits execution,
// and automatically records successes or retriable failures.
type CircuitBreakerClient struct {
	client  model.Client
	breaker *model.CircuitBreaker
	name    string
}

// NewCircuitBreakerClient creates a circuit-breaker-wrapped client.
func NewCircuitBreakerClient(client model.Client, name string, breaker *model.CircuitBreaker) *CircuitBreakerClient {
	return &CircuitBreakerClient{
		client:  client,
		breaker: breaker,
		name:    name,
	}
}

// Stream checks the circuit breaker before delegating to the underlying client.
// When the breaker is open the call returns a *model.CircuitBreakerOpenError
// without touching the wrapped client.
// On success it records a success; on a retriable failure it records a failure.
func (c *CircuitBreakerClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	if !c.breaker.CanExecute() {
		return nil, &model.CircuitBreakerOpenError{Provider: c.name}
	}

	stream, err := c.client.Stream(ctx, req)
	if err != nil {
		// Use ProviderError classification when available for precise
		// circuit-breaker decisions. Auth and quota errors should not
		// trip the breaker because they are configuration issues.
		var pe *model.ProviderError
		if errors.As(err, &pe) {
			if pe.ShouldTriggerCircuitBreaker() {
				c.breaker.RecordFailure()
			}
		} else if isRetriableError(err) {
			// Fallback for non-ProviderError errors: only count server-side
			// failures. Exclude auth-related keywords to avoid tripping the
			// breaker on bad credentials.
			msg := strings.ToLower(err.Error())
			if !strings.Contains(msg, "auth") && !strings.Contains(msg, "unauthorized") && !strings.Contains(msg, "forbidden") {
				c.breaker.RecordFailure()
			}
		}
		return nil, err
	}

	c.breaker.RecordSuccess()
	return stream, nil
}

// Unwrap returns the underlying client.
func (c *CircuitBreakerClient) Unwrap() model.Client {
	return c.client
}

// Name returns the provider identifier used for diagnostics.
func (c *CircuitBreakerClient) Name() string {
	return c.name
}

// Breaker returns the circuit breaker.
func (c *CircuitBreakerClient) Breaker() *model.CircuitBreaker {
	return c.breaker
}
