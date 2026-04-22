package remote

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

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
// It wraps every raw transport with ResilientEventStream so transient disconnects are
// automatically retried.
type LifecycleManager struct {
	// subscriptions owns active subscription state and teardown.
	subscriptions *SubscriptionManager
	// streamFactory creates transport connections from runtime remote session config.
	streamFactory StreamFactory
	// currentStream holds the most recently created ResilientEventStream so that
	// observability methods can report connection health.
	currentStream *ResilientEventStream
	// currentMu protects currentStream.
	currentMu sync.RWMutex
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

// Subscribe opens one remote stream, wraps it with ResilientEventStream for automatic
// reconnection, starts subscription delivery, and returns one unsubscribe function.
// onEvent is called for each remote event received; nil is accepted and events are discarded.
func (m *LifecycleManager) Subscribe(ctx context.Context, session coreconfig.RemoteSessionConfig, onEvent func(Event)) (func() error, error) {
	if m == nil {
		return nil, fmt.Errorf("remote lifecycle manager is nil")
	}

	// Wrap the factory with resilient reconnection. The dialer uses a
	// background context so retries survive caller context cancellation.
	dialer := func(dialCtx context.Context) (EventStream, error) {
		return m.streamFactory.Open(dialCtx, session)
	}

	stream := NewResilientEventStream(dialer, DefaultBackoffConfig(), nil)

	m.currentMu.Lock()
	m.currentStream = stream
	m.currentMu.Unlock()

	subscriptionID, err := m.subscriptions.Subscribe(ctx, stream, onEvent, func(err error) {
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

// ConnectionState returns the current resilient stream connection state as a
// human-readable string. If no stream has been created yet, it returns
// "connecting".
func (m *LifecycleManager) ConnectionState() string {
	if m == nil {
		return StateConnecting.String()
	}
	m.currentMu.RLock()
	s := m.currentStream
	m.currentMu.RUnlock()
	if s != nil {
		return s.State().String()
	}
	return StateConnecting.String()
}

// ReconnectCount returns the number of successful reconnections performed by
// the current resilient stream.
func (m *LifecycleManager) ReconnectCount() int {
	if m == nil {
		return 0
	}
	m.currentMu.RLock()
	s := m.currentStream
	m.currentMu.RUnlock()
	if s != nil {
		return s.ReconnectCount()
	}
	return 0
}

// LastDisconnectError returns the error that caused the most recent disconnect,
// or nil if the stream has never disconnected.
func (m *LifecycleManager) LastDisconnectError() error {
	if m == nil {
		return nil
	}
	m.currentMu.RLock()
	s := m.currentStream
	m.currentMu.RUnlock()
	if s != nil {
		return s.LastDisconnectError()
	}
	return nil
}

// LastDisconnectTime returns the timestamp of the most recent disconnect,
// or the zero time if the stream has never disconnected.
func (m *LifecycleManager) LastDisconnectTime() time.Time {
	if m == nil {
		return time.Time{}
	}
	m.currentMu.RLock()
	s := m.currentStream
	m.currentMu.RUnlock()
	if s != nil {
		return s.LastDisconnectTime()
	}
	return time.Time{}
}

// ActiveSubscriptionCount returns the number of active subscriptions managed by this lifecycle manager.
func (m *LifecycleManager) ActiveSubscriptionCount() int {
	if m == nil {
		return 0
	}
	return m.subscriptions.ActiveCount()
}

// IsClosed reports whether the underlying subscription manager has been closed.
func (m *LifecycleManager) IsClosed() bool {
	if m == nil {
		return false
	}
	return m.subscriptions.IsClosed()
}

// Close releases all active subscriptions tracked by this lifecycle manager.
func (m *LifecycleManager) Close() error {
	if m == nil {
		return nil
	}
	return m.subscriptions.Close()
}
