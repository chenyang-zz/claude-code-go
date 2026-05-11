package engine

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// LoadBalanceStrategy defines how the ProviderPool selects a provider for each Stream call.
type LoadBalanceStrategy int

const (
	// LoadBalancePrimaryWithFailover tries each provider in order on connection failure.
	// This is the default and provides automatic failover without additional latency.
	LoadBalancePrimaryWithFailover LoadBalanceStrategy = iota
	// LoadBalanceRoundRobin distributes requests across all providers equally.
	LoadBalanceRoundRobin
)

// providerClient pairs a model.Client with a display name for diagnostics.
type providerClient struct {
	name   string
	client model.Client
}

// ProviderPool distributes Stream calls across multiple model.Clients with
// configurable load-balancing strategies. It implements model.Client and is
// designed to be used as the primary client in engine.Runtime.
//
// For PrimaryWithFailover (default): providers are tried in order when
// Stream returns an error. Circuit-broken providers are skipped automatically
// since CircuitBreakerClient.Stream returns CircuitBreakerOpenError immediately.
//
// For RoundRobin: requests are distributed evenly across all providers.
// Individual Stream failures are returned to the engine's retry loop, which
// naturally rotates through providers on each retry attempt.
type ProviderPool struct {
	clients  []providerClient
	strategy LoadBalanceStrategy
	counter  atomic.Uint64
}

// NewProviderPool creates a ProviderPool with the given clients, display names,
// and load-balance strategy. Clients with nil or nil inner client are skipped.
// When only one client is provided, the pool delegates directly without overhead.
func NewProviderPool(clients []model.Client, names []string, strategy LoadBalanceStrategy) *ProviderPool {
	pool := &ProviderPool{
		strategy: strategy,
	}
	for i, client := range clients {
		if client == nil {
			continue
		}
		name := fmt.Sprintf("provider-%d", i)
		if i < len(names) && names[i] != "" {
			name = names[i]
		}
		pool.clients = append(pool.clients, providerClient{name: name, client: client})
	}
	return pool
}

// Stream implements model.Client by selecting a provider based on the pool's
// load-balance strategy.
func (p *ProviderPool) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	if len(p.clients) == 0 {
		return nil, fmt.Errorf("provider pool: no providers configured")
	}
	if len(p.clients) == 1 {
		return p.clients[0].client.Stream(ctx, req)
	}

	switch p.strategy {
	case LoadBalanceRoundRobin:
		return p.roundRobinStream(ctx, req)
	default:
		return p.failoverStream(ctx, req)
	}
}

// failoverStream tries each provider in order. When a provider returns a
// CircuitBreakerOpenError it is skipped immediately. For other errors the
// next provider is also tried, since different providers may accept the
// same request (different credentials, models, or availability).
func (p *ProviderPool) failoverStream(ctx context.Context, req model.Request) (model.Stream, error) {
	var lastErr error
	for _, pc := range p.clients {
		stream, err := pc.client.Stream(ctx, req)
		if err == nil {
			return stream, nil
		}
		lastErr = err

		// Circuit-broken providers are skipped without comment.
		var cbErr *model.CircuitBreakerOpenError
		if errors.As(err, &cbErr) {
			continue
		}
	}

	if lastErr == nil {
		return nil, fmt.Errorf("provider pool: all providers circuit-broken")
	}
	return nil, lastErr
}

// roundRobinStream picks the next provider in round-robin order.
func (p *ProviderPool) roundRobinStream(ctx context.Context, req model.Request) (model.Stream, error) {
	n := p.counter.Add(1)
	idx := int(n % uint64(len(p.clients)))
	return p.clients[idx].client.Stream(ctx, req)
}

// NumProviders returns the number of configured providers.
func (p *ProviderPool) NumProviders() int {
	return len(p.clients)
}
