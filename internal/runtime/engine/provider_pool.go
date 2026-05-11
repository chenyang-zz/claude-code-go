package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
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
	mu       sync.Mutex
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
	p.mu.Lock()
	clients := p.clients
	strategy := p.strategy
	numProviders := len(clients)
	p.mu.Unlock()

	if numProviders == 0 {
		return nil, fmt.Errorf("provider pool: no providers configured")
	}
	if numProviders == 1 {
		return clients[0].client.Stream(ctx, req)
	}

	switch strategy {
	case LoadBalanceRoundRobin:
		return p.roundRobinStream(ctx, req, clients)
	default:
		return p.failoverStream(ctx, req, clients)
	}
}

// failoverStream tries each provider in order. When a provider returns a
// CircuitBreakerOpenError it is skipped immediately. For other errors the
// next provider is also tried, since different providers may accept the
// same request (different credentials, models, or availability).
func (p *ProviderPool) failoverStream(ctx context.Context, req model.Request, clients []providerClient) (model.Stream, error) {
	var lastErr error
	for _, pc := range clients {
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
func (p *ProviderPool) roundRobinStream(ctx context.Context, req model.Request, clients []providerClient) (model.Stream, error) {
	n := p.counter.Add(1)
	idx := int(n % uint64(len(clients)))
	return clients[idx].client.Stream(ctx, req)
}

// NumProviders returns the number of configured providers.
func (p *ProviderPool) NumProviders() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.clients)
}

// ProviderInfo carries diagnostic state for one provider in the pool.
type ProviderInfo struct {
	Name                string
	Position            int
	CircuitBreakerState string
	FailureCount        int
	TripCount           int
	Strategy            string
}

// Providers returns diagnostic information about all providers in the pool.
func (p *ProviderPool) Providers() []ProviderInfo {
	p.mu.Lock()
	defer p.mu.Unlock()

	info := make([]ProviderInfo, 0, len(p.clients))
	for i, pc := range p.clients {
		pi := ProviderInfo{
			Name:     pc.name,
			Position: i,
		}
		if cbc, ok := pc.client.(*CircuitBreakerClient); ok {
			pi.CircuitBreakerState = string(cbc.Breaker().State())
			pi.FailureCount = cbc.Breaker().FailureCount()
			pi.TripCount = cbc.Breaker().TripCount()
		}
		info = append(info, pi)
	}
	return info
}

// SetActiveProvider moves the named provider to the front of the pool,
// making it the primary target for future Stream calls under
// PrimaryWithFailover strategy. Returns an error if the name is not found.
func (p *ProviderPool) SetActiveProvider(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, pc := range p.clients {
		if pc.name == name {
			// Move to front by creating a new slice
			p.clients = append([]providerClient{pc}, append(p.clients[:i:i], p.clients[i+1:]...)...)
			// Reset round-robin counter so the first call hits the new primary
			p.counter.Store(0)
			return nil
		}
	}
	return fmt.Errorf("provider %q not found in pool", name)
}

// StrategyName returns the human-readable name of the active strategy.
func (p *ProviderPool) StrategyName() string {
	switch p.strategy {
	case LoadBalanceRoundRobin:
		return "round-robin"
	default:
		return "primary-with-failover"
	}
}
