package remote

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeStream struct {
	mu     sync.Mutex
	events chan Event
	closed bool
}

func newFakeStream() *fakeStream {
	return &fakeStream{
		events: make(chan Event, 8),
	}
}

func (f *fakeStream) Recv(ctx context.Context) (Event, error) {
	select {
	case <-ctx.Done():
		return Event{}, ctx.Err()
	case event, ok := <-f.events:
		if !ok {
			return Event{}, ErrStreamClosed
		}
		return event, nil
	}
}

func (f *fakeStream) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	close(f.events)
	return nil
}

func (f *fakeStream) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// TestSubscriptionManagerSubscribe verifies events are delivered through active subscriptions.
func TestSubscriptionManagerSubscribe(t *testing.T) {
	t.Parallel()

	manager := NewSubscriptionManager()
	stream := newFakeStream()

	eventCh := make(chan Event, 1)
	subscriptionID, err := manager.Subscribe(context.Background(), stream, func(event Event) {
		eventCh <- event
	}, nil)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	if subscriptionID == "" {
		t.Fatalf("Subscribe() returned empty subscription id")
	}

	stream.events <- Event{Transport: TransportSSE, Type: "client_event", Data: []byte("ok")}

	select {
	case event := <-eventCh:
		if event.Type != "client_event" {
			t.Fatalf("event.Type = %q, want %q", event.Type, "client_event")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for subscribed event")
	}
}

// TestSubscriptionManagerUnsubscribe verifies unsubscribe cancels and closes resources.
func TestSubscriptionManagerUnsubscribe(t *testing.T) {
	t.Parallel()

	manager := NewSubscriptionManager()
	stream := newFakeStream()

	subscriptionID, err := manager.Subscribe(context.Background(), stream, nil, nil)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	if err := manager.Unsubscribe(subscriptionID); err != nil {
		t.Fatalf("Unsubscribe() error = %v", err)
	}
	if !stream.isClosed() {
		t.Fatalf("stream should be closed after unsubscribe")
	}
}

// TestSubscriptionManagerClose verifies manager close releases all active subscriptions.
func TestSubscriptionManagerClose(t *testing.T) {
	t.Parallel()

	manager := NewSubscriptionManager()
	streamA := newFakeStream()
	streamB := newFakeStream()

	_, err := manager.Subscribe(context.Background(), streamA, nil, nil)
	if err != nil {
		t.Fatalf("Subscribe(streamA) error = %v", err)
	}
	_, err = manager.Subscribe(context.Background(), streamB, nil, nil)
	if err != nil {
		t.Fatalf("Subscribe(streamB) error = %v", err)
	}

	if err := manager.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !streamA.isClosed() || !streamB.isClosed() {
		t.Fatalf("all streams should be closed after manager close")
	}

	_, err = manager.Subscribe(context.Background(), newFakeStream(), nil, nil)
	if err == nil {
		t.Fatalf("Subscribe() after close should fail")
	}
}

// TestSubscriptionManager_ResilientStreamDisconnected verifies that a
// subscription survives ErrStreamDisconnected and resumes event delivery
// once the underlying stream reconnects.
func TestSubscriptionManager_ResilientStreamDisconnected(t *testing.T) {
	t.Parallel()

	manager := NewSubscriptionManager()

	// Stream returns: event1, ErrStreamDisconnected, event2.
	stream := newMockStream([]mockResult{
		{event: Event{Transport: TransportSSE, Type: "event1", Data: []byte("a")}},
		{err: ErrStreamDisconnected},
		{event: Event{Transport: TransportSSE, Type: "event2", Data: []byte("b")}},
	})

	var events []Event
	var gotErrors []error
	var mu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	subscriptionID, err := manager.Subscribe(ctx, stream, func(event Event) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	}, func(err error) {
		mu.Lock()
		gotErrors = append(gotErrors, err)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	// Poll until both events are delivered.
	for {
		mu.Lock()
		done := len(events) >= 2
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-time.After(50 * time.Millisecond):
			continue
		case <-ctx.Done():
			t.Fatalf("timed out waiting for events, got %d events and %d errors", len(events), len(gotErrors))
		}
	}

	mu.Lock()
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Type != "event1" || events[1].Type != "event2" {
		t.Fatalf("events = %v, want [event1, event2]", events)
	}
	if len(gotErrors) != 1 || !errors.Is(gotErrors[0], ErrStreamDisconnected) {
		t.Fatalf("errors = %v, want [ErrStreamDisconnected]", gotErrors)
	}
	mu.Unlock()

	if err := manager.Unsubscribe(subscriptionID); err != nil {
		t.Fatalf("Unsubscribe() error = %v", err)
	}
}
