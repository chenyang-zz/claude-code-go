package remote

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ErrStreamDisconnected indicates the underlying transport disconnected due to a
// transient network error and an automatic reconnection is in progress.
// Callers (e.g. SubscriptionManager) should treat this as a recoverable
// condition: notify via onError and continue calling Recv.
var ErrStreamDisconnected = errors.New("remote stream disconnected, reconnecting")

// ConnectionState represents the lifecycle state of a resilient stream.
type ConnectionState int

const (
	// StateConnecting means the stream is actively trying to establish a
	// transport connection (initial dial or re-dial after disconnect).
	StateConnecting ConnectionState = iota
	// StateConnected means the stream has an active underlying transport and
	// is receiving events.
	StateConnected
	// StateDisconnected means the stream lost its transport and is waiting
	// before the next reconnection attempt.
	StateDisconnected
	// StateClosed means the stream has been permanently closed and will not
	// reconnect.
	StateClosed
)

// String returns a human-readable label for the connection state.
func (s ConnectionState) String() string {
	switch s {
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateDisconnected:
		return "disconnected"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// StateChangeCallback is invoked on every connection state transition.
// oldState and newState describe the transition; err carries the trigger
// error when transitioning into StateDisconnected or StateClosed.
type StateChangeCallback func(oldState, newState ConnectionState, err error)

// BackoffConfig tunes the exponential backoff behaviour used while
// reconnecting after transient failures.
type BackoffConfig struct {
	// InitialInterval is the first retry interval. Defaults to 1s.
	InitialInterval time.Duration
	// MaxInterval caps the retry interval. Defaults to 30s.
	MaxInterval time.Duration
	// Multiplier scales the interval after each failed attempt. Defaults to 2.0.
	Multiplier float64
	// JitterFraction adds random jitter (+/- fraction). Defaults to 0.1.
	JitterFraction float64
	// MaxRetries limits the total number of reconnection attempts.
	// Zero or negative means unlimited retries until Close() is called.
	MaxRetries int
}

// DefaultBackoffConfig returns a conservative backoff policy suitable for
// CLI remote sessions over the public internet.
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
		Multiplier:      2.0,
		JitterFraction:  0.1,
		MaxRetries:      0,
	}
}

// ResilientEventStream wraps an EventStream and automatically re-creates the
// underlying transport when transient network errors occur.
// It implements the EventStream interface transparently.
type ResilientEventStream struct {
	// dialer creates a fresh underlying EventStream. It is called on initial
	// connection and on every reconnection attempt.
	dialer func(ctx context.Context) (EventStream, error)
	// config holds the exponential-backoff parameters.
	config BackoffConfig

	// stream is the currently active underlying transport.
	stream   EventStream
	streamMu sync.RWMutex

	// reconnectMu serialises reconnection goroutines so only one retry loop
	// runs at a time.
	reconnectMu sync.Mutex
	// reconnecting is true while a background reconnection is in flight.
	reconnecting bool
	// reconnectDone closes when the current reconnection attempt finishes.
	reconnectDone chan struct{}
	// reconnectErr stores the terminal error when reconnection finally fails.
	reconnectErr error

	// state is the current connection state.
	state   ConnectionState
	stateMu sync.RWMutex
	// onStateChange is called (non-blocking) on every state transition.
	onStateChange StateChangeCallback

	// reconnectCount is the total number of successful reconnections.
	reconnectCount atomic.Int32
	// lastDisconnectErr records the error that caused the most recent disconnect.
	lastDisconnectErr error
	// lastDisconnectTime records when the most recent disconnect occurred.
	lastDisconnectTime time.Time

	// closeOnce ensures Close side-effects execute exactly once.
	closeOnce sync.Once
	// closed is set to true after Close() is called.
	closed atomic.Bool
}

// NewResilientEventStream creates a resilient wrapper around a dialer.
// The stream is lazily connected on the first Recv call.
func NewResilientEventStream(
	dialer func(ctx context.Context) (EventStream, error),
	config BackoffConfig,
	onStateChange StateChangeCallback,
) *ResilientEventStream {
	if config.InitialInterval <= 0 {
		config.InitialInterval = 1 * time.Second
	}
	if config.MaxInterval <= 0 {
		config.MaxInterval = 30 * time.Second
	}
	if config.Multiplier <= 1.0 {
		config.Multiplier = 2.0
	}
	if config.JitterFraction < 0 {
		config.JitterFraction = 0
	}

	r := &ResilientEventStream{
		dialer:        dialer,
		config:        config,
		onStateChange: onStateChange,
		state:         StateConnecting,
	}

	logger.DebugCF("remote_resilient", "created resilient event stream", map[string]any{
		"initial_interval": config.InitialInterval.String(),
		"max_interval":     config.MaxInterval.String(),
		"multiplier":       config.Multiplier,
		"jitter":           config.JitterFraction,
		"max_retries":      config.MaxRetries,
	})

	return r
}

// Recv blocks until an event is available, the stream is permanently closed,
// or the provided context is cancelled.
// If the underlying transport fails with a transient error, Recv returns
// ErrStreamDisconnected and initiates a background reconnection. The caller
// should treat this as recoverable and call Recv again.
func (r *ResilientEventStream) Recv(ctx context.Context) (Event, error) {
	for {
		if r.closed.Load() {
			return Event{}, ErrStreamClosed
		}

		// Fast path: we already have an active stream.
		stream := r.getStream()
		if stream != nil {
			event, err := stream.Recv(ctx)
			if err == nil {
				return event, nil
			}

			// The underlying transport returned an error. Determine whether
			// this is a terminal condition or a transient disconnect.
			if r.closed.Load() {
				return Event{}, ErrStreamClosed
			}
			if isPermanentRecvError(err) {
				logger.WarnCF("remote_resilient", "permanent receive error, closing stream", map[string]any{
					"error": err.Error(),
				})
				r.transitionState(StateClosed, err)
				return Event{}, err
			}

			// Transient error: tear down the current transport and start
			// reconnection in the background.
			r.beginReconnect(err)
			return Event{}, ErrStreamDisconnected
		}

		// Slow path: no active stream (initial connection or after disconnect).
		// If already closed, return the terminal error immediately.
		if r.State() == StateClosed {
			r.reconnectMu.Lock()
			err := r.reconnectErr
			r.reconnectMu.Unlock()
			if err != nil {
				return Event{}, err
			}
			return Event{}, ErrStreamClosed
		}
		// Try to establish one inline first.
		if err := r.connect(ctx); err != nil {
			if r.closed.Load() {
				return Event{}, ErrStreamClosed
			}
			if isPermanentDialError(err) {
				r.transitionState(StateClosed, err)
				return Event{}, err
			}
			// Transient dial error: start background reconnection and
			// report disconnection to the caller.
			r.beginReconnect(err)
			return Event{}, ErrStreamDisconnected
		}
		// Connection succeeded — loop back and read from the new stream.
	}
}

// Close permanently shuts down the resilient stream and releases all
// underlying transport resources. Subsequent Recv calls return ErrStreamClosed.
func (r *ResilientEventStream) Close() error {
	r.closeOnce.Do(func() {
		r.closed.Store(true)

		// Tear down any active transport.
		r.streamMu.Lock()
		if r.stream != nil {
			_ = r.stream.Close()
			r.stream = nil
		}
		r.streamMu.Unlock()

		r.transitionState(StateClosed, nil)

		logger.DebugCF("remote_resilient", "closed resilient event stream", map[string]any{
			"reconnect_count": r.reconnectCount.Load(),
		})
	})
	return nil
}

// State returns the current connection state.
func (r *ResilientEventStream) State() ConnectionState {
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()
	return r.state
}

// ReconnectCount returns the number of successful reconnections performed
// since the stream was created.
func (r *ResilientEventStream) ReconnectCount() int {
	return int(r.reconnectCount.Load())
}

// LastDisconnectError returns the error that caused the most recent
// disconnect, or nil if the stream has never disconnected.
func (r *ResilientEventStream) LastDisconnectError() error {
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()
	return r.lastDisconnectErr
}

// LastDisconnectTime returns the timestamp of the most recent disconnect,
// or the zero time if the stream has never disconnected.
func (r *ResilientEventStream) LastDisconnectTime() time.Time {
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()
	return r.lastDisconnectTime
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (r *ResilientEventStream) getStream() EventStream {
	r.streamMu.RLock()
	defer r.streamMu.RUnlock()
	return r.stream
}

// connect attempts one inline dial. On success it stores the stream and
// transitions to StateConnected.
func (r *ResilientEventStream) connect(ctx context.Context) error {
	r.transitionState(StateConnecting, nil)

	stream, err := r.dialer(ctx)
	if err != nil {
		return err
	}

	r.streamMu.Lock()
	r.stream = stream
	r.streamMu.Unlock()

	r.transitionState(StateConnected, nil)

	logger.DebugCF("remote_resilient", "transport connected", nil)
	return nil
}

// beginReconnect tears down the current transport and spawns a background
// goroutine that retries the dial with exponential backoff.
func (r *ResilientEventStream) beginReconnect(triggerErr error) {
	r.reconnectMu.Lock()
	defer r.reconnectMu.Unlock()

	if r.reconnecting {
		return // another goroutine is already reconnecting
	}

	r.reconnecting = true
	r.reconnectDone = make(chan struct{})
	r.reconnectErr = nil

	// Close the broken transport.
	r.streamMu.Lock()
	if r.stream != nil {
		_ = r.stream.Close()
		r.stream = nil
	}
	r.streamMu.Unlock()

	// Record disconnect metadata.
	r.stateMu.Lock()
	r.lastDisconnectErr = triggerErr
	r.lastDisconnectTime = time.Now()
	r.stateMu.Unlock()

	r.transitionState(StateDisconnected, triggerErr)

	logger.WarnCF("remote_resilient", "transport disconnected, starting reconnection", map[string]any{
		"error": triggerErr.Error(),
	})

	go r.reconnectLoop()
}

// reconnectLoop retries the dial with exponential backoff until success,
// a permanent error, or Close() is called.
func (r *ResilientEventStream) reconnectLoop() {
	defer func() {
		r.reconnectMu.Lock()
		close(r.reconnectDone)
		r.reconnecting = false
		r.reconnectMu.Unlock()
	}()

	backoff := newBackoff(r.config)
	attempt := 0

	for {
		if r.closed.Load() {
			r.reconnectErr = ErrStreamClosed
			return
		}

		if r.config.MaxRetries > 0 && attempt >= r.config.MaxRetries {
			err := fmt.Errorf("reconnection exhausted after %d attempts", attempt)
			r.reconnectErr = err
			r.transitionState(StateClosed, err)
			logger.ErrorCF("remote_resilient", "reconnection exhausted", map[string]any{
				"attempts": attempt,
			})
			return
		}
		attempt++

		r.transitionState(StateConnecting, nil)

		// Use a background context so reconnection survives caller
		// context cancellation; only Close() can stop it.
		stream, err := r.dialer(context.Background())
		if err == nil {
			r.streamMu.Lock()
			r.stream = stream
			r.streamMu.Unlock()

			r.reconnectCount.Add(1)
			backoff.reset()
			r.transitionState(StateConnected, nil)

			logger.DebugCF("remote_resilient", "transport reconnected", map[string]any{
				"attempt": attempt,
			})
			return
		}

		if r.closed.Load() {
			r.reconnectErr = ErrStreamClosed
			return
		}

		if isPermanentDialError(err) {
			r.reconnectErr = err
			r.transitionState(StateClosed, err)
			logger.ErrorCF("remote_resilient", "permanent dial error during reconnection", map[string]any{
				"error": err.Error(),
			})
			return
		}

		logger.WarnCF("remote_resilient", "reconnection attempt failed", map[string]any{
			"attempt": attempt,
			"error":   err.Error(),
		})

		// Wait before the next attempt.
		if !backoff.wait(context.Background()) {
			return // closed
		}
	}
}

// waitReconnect blocks until the current background reconnection finishes.
// It returns a channel that closes when reconnection completes (success or
// failure). If no reconnection is in flight it returns a closed channel.
func (r *ResilientEventStream) waitReconnect() <-chan struct{} {
	r.reconnectMu.Lock()
	defer r.reconnectMu.Unlock()

	if !r.reconnecting {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return r.reconnectDone
}

// transitionState updates the state and notifies the callback.
func (r *ResilientEventStream) transitionState(newState ConnectionState, err error) {
	r.stateMu.Lock()
	oldState := r.state
	if oldState != newState {
		r.state = newState
	}
	r.stateMu.Unlock()

	if oldState != newState && r.onStateChange != nil {
		// Invoke callback outside the lock.
		r.onStateChange(oldState, newState, err)
	}
}

// ---------------------------------------------------------------------------
// Error classification
// ---------------------------------------------------------------------------

// isPermanentDialError classifies dial-time errors.
// HTTP 4xx client errors (except 429 Too Many Requests) are considered
// permanent because retrying the same request will not help.
func isPermanentDialError(err error) bool {
	if err == nil {
		return false
	}
	// HTTP client errors (4xx) except 429 are permanent.
	// We detect these via the error string because the underlying dialer
	// returns wrapped errors; the status code is embedded in the message.
	var status int
	if _, parseErr := fmt.Sscanf(err.Error(), "connect sse stream: %*w (status=%d)", &status); parseErr == nil {
		if status >= 400 && status < 500 && status != http.StatusTooManyRequests {
			return true
		}
	}
	if _, parseErr := fmt.Sscanf(err.Error(), "connect websocket stream: %*w (status=%d)", &status); parseErr == nil {
		if status >= 400 && status < 500 && status != http.StatusTooManyRequests {
			return true
		}
	}
	// Dial errors that include HTTP status in the SSE rejection format.
	if _, parseErr := fmt.Sscanf(err.Error(), "sse stream rejected: status=%d", &status); parseErr == nil {
		if status >= 400 && status < 500 && status != http.StatusTooManyRequests {
			return true
		}
	}
	return false
}

// isPermanentRecvError classifies receive-time errors.
// Context cancellation and stream closure are terminal; everything else
// is treated as transient.
func isPermanentRecvError(err error) bool {
	if err == nil {
		return false
	}
	// Normal lifecycle termination.
	if errors.Is(err, context.Canceled) || errors.Is(err, ErrStreamClosed) {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Exponential backoff
// ---------------------------------------------------------------------------

// backoff implements jittered exponential backoff.
type backoff struct {
	config   BackoffConfig
	interval time.Duration
}

func newBackoff(config BackoffConfig) *backoff {
	return &backoff{
		config:   config,
		interval: config.InitialInterval,
	}
}

// wait sleeps for the current backoff interval and then advances the interval.
// It returns false if the stream was closed during the wait.
func (b *backoff) wait(ctx context.Context) bool {
	jitter := time.Duration(0)
	if b.config.JitterFraction > 0 {
		jitter = time.Duration(
			(float64(b.interval) * b.config.JitterFraction) * (2*rand.Float64() - 1),
		)
	}
	delay := b.interval + jitter
	if delay < 0 {
		delay = 0
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		// Advance interval for next attempt.
		next := time.Duration(float64(b.interval) * b.config.Multiplier)
		if next > b.config.MaxInterval {
			next = b.config.MaxInterval
		}
		b.interval = next
		return true
	}
}

func (b *backoff) reset() {
	b.interval = b.config.InitialInterval
}
