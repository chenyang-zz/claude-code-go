package remote

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// EventHandler consumes one remote event delivered by one active subscription.
type EventHandler func(Event)

// ErrorHandler consumes one non-cancellation error emitted by one active subscription.
type ErrorHandler func(error)

// SubscriptionManager tracks active remote stream subscriptions and lifecycle teardown.
type SubscriptionManager struct {
	// mu protects active subscriptions and closed lifecycle state.
	mu sync.Mutex
	// subscriptions stores active subscription handles keyed by generated id.
	subscriptions map[string]*subscriptionState
	// nextID generates stable subscription identifiers.
	nextID uint64
	// closed indicates the manager has been globally closed.
	closed bool
}

type subscriptionState struct {
	// id uniquely identifies one active subscription.
	id string
	// stream stores the underlying remote stream bound to this subscription.
	stream EventStream
	// cancel stops the subscription receive loop.
	cancel context.CancelFunc
	// done closes when the subscription receive loop exits and resources are released.
	done chan struct{}
}

// NewSubscriptionManager constructs one empty subscription manager.
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions: make(map[string]*subscriptionState),
	}
}

// Subscribe starts one background receive loop for a remote stream and returns its subscription id.
func (m *SubscriptionManager) Subscribe(
	ctx context.Context,
	stream EventStream,
	onEvent EventHandler,
	onError ErrorHandler,
) (string, error) {
	if m == nil {
		return "", fmt.Errorf("subscription manager is nil")
	}
	if stream == nil {
		return "", fmt.Errorf("missing event stream")
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return "", fmt.Errorf("subscription manager is closed")
	}
	id := fmt.Sprintf("sub_%d", atomic.AddUint64(&m.nextID, 1))
	loopCtx, cancel := context.WithCancel(ctx)
	state := &subscriptionState{
		id:     id,
		stream: stream,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	m.subscriptions[id] = state
	m.mu.Unlock()

	logger.DebugCF("remote_subscription", "registered remote subscription", map[string]any{
		"subscription_id": id,
	})

	go m.runSubscription(loopCtx, state, onEvent, onError)
	return id, nil
}

// Unsubscribe stops one active subscription and waits for lifecycle teardown.
func (m *SubscriptionManager) Unsubscribe(subscriptionID string) error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	state, ok := m.subscriptions[subscriptionID]
	m.mu.Unlock()
	if !ok {
		return nil
	}

	logger.DebugCF("remote_subscription", "unsubscribing remote stream", map[string]any{
		"subscription_id": subscriptionID,
	})

	state.cancel()
	<-state.done
	return nil
}

// Close unsubscribes every active subscription and prevents future subscriptions.
func (m *SubscriptionManager) Close() error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	ids := make([]string, 0, len(m.subscriptions))
	for id := range m.subscriptions {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		if err := m.Unsubscribe(id); err != nil {
			return err
		}
	}

	logger.DebugCF("remote_subscription", "closed subscription manager", map[string]any{
		"subscription_count": len(ids),
	})
	return nil
}

func (m *SubscriptionManager) runSubscription(
	ctx context.Context,
	state *subscriptionState,
	onEvent EventHandler,
	onError ErrorHandler,
) {
	defer close(state.done)
	defer m.removeSubscription(state.id)
	defer func() {
		if err := state.stream.Close(); err != nil {
			logger.WarnCF("remote_subscription", "failed to close stream during teardown", map[string]any{
				"subscription_id": state.id,
				"error":           err.Error(),
			})
		}
	}()

	for {
		event, err := state.stream.Recv(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, ErrStreamClosed) {
				return
			}
			// Transient disconnect (e.g. network blip with an automatic
			// reconnection in progress). Notify the caller via onError but
			// keep the subscription alive so Recv resumes once the stream
			// reconnects.
			if errors.Is(err, ErrStreamDisconnected) {
				if onError != nil {
					onError(err)
				}
				continue
			}
			if onError != nil {
				onError(err)
			}
			return
		}
		if onEvent != nil {
			onEvent(event)
		}
	}
}

// ActiveCount returns the number of currently active subscriptions.
func (m *SubscriptionManager) ActiveCount() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.subscriptions)
}

// IsClosed reports whether the manager has been globally closed.
func (m *SubscriptionManager) IsClosed() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func (m *SubscriptionManager) removeSubscription(subscriptionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subscriptions, subscriptionID)
}
