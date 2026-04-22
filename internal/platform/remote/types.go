package remote

import (
	"context"
	"errors"
)

// ErrStreamClosed indicates the remote event stream has already been closed.
var ErrStreamClosed = errors.New("remote stream closed")

// TransportKind identifies which remote transport produced one incoming event.
type TransportKind string

const (
	// TransportSSE indicates one event originated from an SSE stream.
	TransportSSE TransportKind = "sse"
	// TransportWebSocket indicates one event originated from a WebSocket stream.
	TransportWebSocket TransportKind = "websocket"
)

// Event stores one normalized remote transport payload consumed by runtime bridges.
type Event struct {
	// Transport identifies the source transport that produced this event.
	Transport TransportKind
	// ID stores the optional frame/event identifier when the transport provides one.
	ID string
	// Type stores the logical event name (SSE event name or ws message kind).
	Type string
	// Data stores the raw payload bytes.
	Data []byte
}

// EventStream exposes the minimum lifecycle used by remote runtime bridges.
type EventStream interface {
	// Recv blocks until one event is available, the stream closes, or context cancellation fires.
	Recv(ctx context.Context) (Event, error)
	// Close releases all underlying transport resources.
	Close() error
}
