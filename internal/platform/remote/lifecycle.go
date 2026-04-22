package remote

import (
	"context"
	"fmt"
	"strings"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// StreamFactory creates one remote event stream from runtime remote session settings.
type StreamFactory interface {
	// Open builds and connects one remote event stream.
	Open(ctx context.Context, session coreconfig.RemoteSessionConfig) (EventStream, error)
}

// DefaultStreamFactory opens SSE/WS streams from one RemoteSessionConfig stream endpoint.
type DefaultStreamFactory struct{}

// Open dials the configured stream endpoint using the matching transport.
func (f DefaultStreamFactory) Open(ctx context.Context, session coreconfig.RemoteSessionConfig) (EventStream, error) {
	trimmedStreamURL := strings.TrimSpace(session.StreamURL)
	if trimmedStreamURL == "" {
		return nil, fmt.Errorf("missing remote stream url")
	}

	switch {
	case strings.HasPrefix(trimmedStreamURL, "ws://"), strings.HasPrefix(trimmedStreamURL, "wss://"):
		return DialWebSocket(ctx, trimmedStreamURL, nil, nil)
	case strings.HasPrefix(trimmedStreamURL, "http://"), strings.HasPrefix(trimmedStreamURL, "https://"):
		return DialSSE(ctx, trimmedStreamURL, nil, nil)
	default:
		return nil, fmt.Errorf("unsupported remote stream scheme: %s", trimmedStreamURL)
	}
}

// LifecycleManager wires subscribe/unsubscribe lifecycle around remote stream connections.
type LifecycleManager struct {
	// subscriptions owns active subscription state and teardown.
	subscriptions *SubscriptionManager
	// streamFactory creates transport connections from runtime remote session config.
	streamFactory StreamFactory
}

// NewLifecycleManager constructs one runtime lifecycle manager with explicit dependencies.
func NewLifecycleManager(subscriptions *SubscriptionManager, streamFactory StreamFactory) *LifecycleManager {
	manager := subscriptions
	if manager == nil {
		manager = NewSubscriptionManager()
	}

	factory := streamFactory
	if factory == nil {
		factory = DefaultStreamFactory{}
	}

	return &LifecycleManager{
		subscriptions: manager,
		streamFactory: factory,
	}
}

// Subscribe opens one remote stream, starts subscription delivery, and returns one unsubscribe function.
func (m *LifecycleManager) Subscribe(ctx context.Context, session coreconfig.RemoteSessionConfig) (func() error, error) {
	if m == nil {
		return nil, fmt.Errorf("remote lifecycle manager is nil")
	}

	stream, err := m.streamFactory.Open(ctx, session)
	if err != nil {
		return nil, err
	}

	subscriptionID, err := m.subscriptions.Subscribe(ctx, stream, nil, func(err error) {
		logger.WarnCF("remote_lifecycle", "remote subscription loop stopped with error", map[string]any{
			"session_id": session.SessionID,
			"error":      err.Error(),
		})
	})
	if err != nil {
		_ = stream.Close()
		return nil, err
	}

	logger.DebugCF("remote_lifecycle", "subscribed remote stream", map[string]any{
		"session_id":      session.SessionID,
		"subscription_id": subscriptionID,
	})

	return func() error {
		logger.DebugCF("remote_lifecycle", "unsubscribing remote stream", map[string]any{
			"session_id":      session.SessionID,
			"subscription_id": subscriptionID,
		})
		return m.subscriptions.Unsubscribe(subscriptionID)
	}, nil
}

// Close releases all active subscriptions tracked by this lifecycle manager.
func (m *LifecycleManager) Close() error {
	if m == nil {
		return nil
	}
	return m.subscriptions.Close()
}
