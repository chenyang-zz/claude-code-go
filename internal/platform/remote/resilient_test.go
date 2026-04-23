package remote

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockResult describes one return value from mockStream.Recv.
type mockResult struct {
	event Event
	err   error
}

// mockStream is a test-double EventStream that returns results in order.
type mockStream struct {
	results   []mockResult
	idx       atomic.Int32
	closeOnce sync.Once
	closed    atomic.Bool
}

func newMockStream(results []mockResult) *mockStream {
	return &mockStream{results: results}
}

func (m *mockStream) Recv(ctx context.Context) (Event, error) {
	if m.closed.Load() {
		return Event{}, ErrStreamClosed
	}

	i := int(m.idx.Add(1) - 1)
	if i < len(m.results) {
		return m.results[i].event, m.results[i].err
	}

	// Block until closed.
	<-ctx.Done()
	return Event{}, ctx.Err()
}

func (m *mockStream) Close() error {
	m.closeOnce.Do(func() {
		m.closed.Store(true)
	})
	return nil
}

// ---------------------------------------------------------------------------
// ResilientEventStream tests
// ---------------------------------------------------------------------------

func TestResilientEventStream_RecvSuccess(t *testing.T) {
	t.Parallel()

	want := Event{Transport: TransportSSE, Type: "test", Data: []byte("hello")}
	inner := newMockStream([]mockResult{{event: want}})

	dialer := func(ctx context.Context, lastSeqNum int64) (EventStream, error) {
		return inner, nil
	}

	r := NewResilientEventStream(dialer, BackoffConfig{MaxRetries: 1}, nil)
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got, err := r.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv error = %v", err)
	}
	if got.Type != want.Type {
		t.Fatalf("event.Type = %q, want %q", got.Type, want.Type)
	}
	if r.State() != StateConnected {
		t.Fatalf("state = %q, want connected", r.State().String())
	}
}

func TestResilientEventStream_ReconnectsAfterTransientError(t *testing.T) {
	t.Parallel()

	event1 := Event{Transport: TransportSSE, Type: "msg1", Data: []byte("a")}
	event2 := Event{Transport: TransportSSE, Type: "msg2", Data: []byte("b")}

	// First stream returns one event then a transient error.
	stream1 := newMockStream([]mockResult{
		{event: event1},
		{err: errors.New("network reset")},
	})
	// Second stream returns the second event.
	stream2 := newMockStream([]mockResult{{event: event2}})

	var callCount atomic.Int32
	dialer := func(ctx context.Context, lastSeqNum int64) (EventStream, error) {
		c := callCount.Add(1)
		if c == 1 {
			return stream1, nil
		}
		return stream2, nil
	}

	// Fast backoff so the test doesn't wait long.
	config := BackoffConfig{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		Multiplier:      2.0,
		JitterFraction:  0,
		MaxRetries:      5,
	}

	r := NewResilientEventStream(dialer, config, nil)
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// First Recv succeeds.
	got1, err := r.Recv(ctx)
	if err != nil {
		t.Fatalf("first Recv error = %v", err)
	}
	if got1.Type != event1.Type {
		t.Fatalf("first event = %q, want %q", got1.Type, event1.Type)
	}

	// Second Recv encounters the transient error and returns ErrStreamDisconnected.
	_, err = r.Recv(ctx)
	if !errors.Is(err, ErrStreamDisconnected) {
		t.Fatalf("second Recv error = %v, want ErrStreamDisconnected", err)
	}

	// Wait for reconnection to finish.
	time.Sleep(200 * time.Millisecond)

	// Third Recv should read from the new stream.
	got2, err := r.Recv(ctx)
	if err != nil {
		t.Fatalf("third Recv error = %v", err)
	}
	if got2.Type != event2.Type {
		t.Fatalf("second event = %q, want %q", got2.Type, event2.Type)
	}

	if r.ReconnectCount() != 1 {
		t.Fatalf("reconnectCount = %d, want 1", r.ReconnectCount())
	}
}

func TestResilientEventStream_ClosedRecvReturnsStreamClosed(t *testing.T) {
	t.Parallel()

	inner := newMockStream(nil)
	dialer := func(ctx context.Context, lastSeqNum int64) (EventStream, error) {
		return inner, nil
	}

	r := NewResilientEventStream(dialer, DefaultBackoffConfig(), nil)
	_ = r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := r.Recv(ctx)
	if !errors.Is(err, ErrStreamClosed) {
		t.Fatalf("Recv error = %v, want ErrStreamClosed", err)
	}
	if r.State() != StateClosed {
		t.Fatalf("state = %q, want closed", r.State().String())
	}
}

func TestResilientEventStream_StateCallbacksFire(t *testing.T) {
	t.Parallel()

	var transitions [][2]ConnectionState
	var mu sync.Mutex

	cb := func(oldState, newState ConnectionState, err error) {
		mu.Lock()
		defer mu.Unlock()
		transitions = append(transitions, [2]ConnectionState{oldState, newState})
	}

	inner := newMockStream([]mockResult{
		{event: Event{Type: "x"}},
		{err: errors.New("boom")},
	})
	dialer := func(ctx context.Context, lastSeqNum int64) (EventStream, error) {
		return inner, nil
	}

	config := BackoffConfig{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		Multiplier:      2.0,
		JitterFraction:  0,
		MaxRetries:      1,
	}

	r := NewResilientEventStream(dialer, config, cb)
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _ = r.Recv(ctx) // connect -> connected
	_, _ = r.Recv(ctx) // transient error -> disconnected

	time.Sleep(100 * time.Millisecond) // let reconnection run

	mu.Lock()
	defer mu.Unlock()

	if len(transitions) < 2 {
		t.Fatalf("expected at least 2 transitions, got %d", len(transitions))
	}

	// First transition should be connecting -> connected.
	if transitions[0][0] != StateConnecting || transitions[0][1] != StateConnected {
		t.Fatalf("first transition = %v -> %v, want connecting -> connected", transitions[0][0], transitions[0][1])
	}
}

func TestResilientEventStream_ExhaustedRetriesCloses(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	dialer := func(ctx context.Context, lastSeqNum int64) (EventStream, error) {
		callCount.Add(1)
		return nil, errors.New("always fails")
	}

	config := BackoffConfig{
		InitialInterval: 5 * time.Millisecond,
		MaxInterval:     10 * time.Millisecond,
		Multiplier:      1.0,
		JitterFraction:  0,
		MaxRetries:      2,
	}

	r := NewResilientEventStream(dialer, config, nil)
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := r.Recv(ctx)
	if !errors.Is(err, ErrStreamDisconnected) {
		t.Fatalf("first Recv error = %v, want ErrStreamDisconnected", err)
	}

	// Wait for retries to exhaust.
	time.Sleep(300 * time.Millisecond)

	// Stream should have moved to closed.
	if r.State() != StateClosed {
		t.Fatalf("state = %q, want closed after exhausted retries", r.State().String())
	}

	// Subsequent Recv should return the terminal error.
	_, err = r.Recv(ctx)
	if err == nil || errors.Is(err, ErrStreamDisconnected) {
		t.Fatalf("expected terminal error after exhausted retries, got %v", err)
	}
}

func TestResilientEventStream_DialPermanentError(t *testing.T) {
	t.Parallel()

	// Simulate an SSE stream rejection with 401 status.
	dialer := func(ctx context.Context, lastSeqNum int64) (EventStream, error) {
		return nil, errors.New("sse stream rejected: status=401 body=unauthorized")
	}

	r := NewResilientEventStream(dialer, DefaultBackoffConfig(), nil)
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := r.Recv(ctx)
	// Permanent dial errors should be returned directly, not wrapped as
	// ErrStreamDisconnected.
	if errors.Is(err, ErrStreamDisconnected) {
		t.Fatalf("expected permanent error, got ErrStreamDisconnected")
	}
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if r.State() != StateClosed {
		t.Fatalf("state = %q, want closed", r.State().String())
	}
}

func TestDefaultBackoffConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultBackoffConfig()
	if cfg.InitialInterval != 1*time.Second {
		t.Fatalf("InitialInterval = %v, want 1s", cfg.InitialInterval)
	}
	if cfg.MaxInterval != 30*time.Second {
		t.Fatalf("MaxInterval = %v, want 30s", cfg.MaxInterval)
	}
	if cfg.Multiplier != 2.0 {
		t.Fatalf("Multiplier = %v, want 2.0", cfg.Multiplier)
	}
	if cfg.JitterFraction != 0.1 {
		t.Fatalf("JitterFraction = %v, want 0.1", cfg.JitterFraction)
	}
	if cfg.MaxRetries != 0 {
		t.Fatalf("MaxRetries = %v, want 0 (unlimited)", cfg.MaxRetries)
	}
}

func TestConnectionState_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state ConnectionState
		want  string
	}{
		{StateConnecting, "connecting"},
		{StateConnected, "connected"},
		{StateDisconnected, "disconnected"},
		{StateClosed, "closed"},
		{ConnectionState(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Fatalf("state.String() = %q, want %q", got, tt.want)
		}
	}
}

// TestBackoffIntervals verifies the exponential backoff grows and caps correctly.
func TestBackoffIntervals(t *testing.T) {
	t.Parallel()

	b := newBackoff(BackoffConfig{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		JitterFraction:  0,
	})

	// First wait should be InitialInterval.
	start := time.Now()
	if !b.wait(context.Background()) {
		t.Fatalf("first wait returned false")
	}
	elapsed := time.Since(start)
	if elapsed < 8*time.Millisecond || elapsed > 20*time.Millisecond {
		t.Fatalf("first wait = %v, want ~10ms", elapsed)
	}

	// Second wait should be 20ms.
	start = time.Now()
	if !b.wait(context.Background()) {
		t.Fatalf("second wait returned false")
	}
	elapsed = time.Since(start)
	if elapsed < 15*time.Millisecond || elapsed > 30*time.Millisecond {
		t.Fatalf("second wait = %v, want ~20ms", elapsed)
	}

	// Third wait should be 40ms.
	start = time.Now()
	if !b.wait(context.Background()) {
		t.Fatalf("third wait returned false")
	}
	elapsed = time.Since(start)
	if elapsed < 30*time.Millisecond || elapsed > 55*time.Millisecond {
		t.Fatalf("third wait = %v, want ~40ms", elapsed)
	}

	// Fourth wait should be 80ms.
	start = time.Now()
	if !b.wait(context.Background()) {
		t.Fatalf("fourth wait returned false")
	}
	elapsed = time.Since(start)
	if elapsed < 65*time.Millisecond || elapsed > 100*time.Millisecond {
		t.Fatalf("fourth wait = %v, want ~80ms", elapsed)
	}

	// Fifth wait should cap at MaxInterval (100ms).
	start = time.Now()
	if !b.wait(context.Background()) {
		t.Fatalf("fifth wait returned false")
	}
	elapsed = time.Since(start)
	if elapsed < 80*time.Millisecond || elapsed > 120*time.Millisecond {
		t.Fatalf("fifth wait = %v, want ~100ms (capped)", elapsed)
	}

	// reset should restore to InitialInterval.
	b.reset()
	start = time.Now()
	if !b.wait(context.Background()) {
		t.Fatalf("post-reset wait returned false")
	}
	elapsed = time.Since(start)
	if elapsed < 8*time.Millisecond || elapsed > 20*time.Millisecond {
		t.Fatalf("post-reset wait = %v, want ~10ms", elapsed)
	}
}

// mockSeqStream is an EventStream that also implements GetLastSequenceNum.
type mockSeqStream struct {
	mockStream
	seqNum int64
}

func (m *mockSeqStream) GetLastSequenceNum() int64 {
	return m.seqNum
}

// TestResilientEventStream_CapturesSequenceNumOnDisconnect verifies that the
// resilient stream captures the underlying transport's sequence number before
// closing it during a reconnect.
func TestResilientEventStream_CapturesSequenceNumOnDisconnect(t *testing.T) {
	t.Parallel()

	stream1 := &mockSeqStream{
		mockStream: mockStream{results: []mockResult{
			{event: Event{Type: "msg"}},
			{err: errors.New("transient")},
		}},
		seqNum: 42,
	}
	stream2 := newMockStream([]mockResult{{event: Event{Type: "msg2"}}})

	var callCount atomic.Int32
	dialer := func(ctx context.Context, lastSeqNum int64) (EventStream, error) {
		c := callCount.Add(1)
		if c == 1 {
			return stream1, nil
		}
		// Verify the sequence number from the first stream was passed.
		if lastSeqNum != 42 {
			t.Errorf("dialer lastSeqNum = %d, want 42", lastSeqNum)
		}
		return stream2, nil
	}

	config := BackoffConfig{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		Multiplier:      2.0,
		JitterFraction:  0,
		MaxRetries:      5,
	}

	r := NewResilientEventStream(dialer, config, nil)
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// First Recv reads from stream1.
	_, err := r.Recv(ctx)
	if err != nil {
		t.Fatalf("first Recv error = %v", err)
	}

	// Second Recv hits transient error, triggering reconnect.
	_, err = r.Recv(ctx)
	if !errors.Is(err, ErrStreamDisconnected) {
		t.Fatalf("second Recv error = %v, want ErrStreamDisconnected", err)
	}

	// Wait for reconnection to finish.
	time.Sleep(200 * time.Millisecond)

	// Third Recv should read from stream2.
	got, err := r.Recv(ctx)
	if err != nil {
		t.Fatalf("third Recv error = %v", err)
	}
	if got.Type != "msg2" {
		t.Fatalf("event = %q, want %q", got.Type, "msg2")
	}

	// ResilientEventStream should expose the captured sequence number.
	if got := r.GetLastSequenceNum(); got != 42 {
		t.Fatalf("GetLastSequenceNum() = %d, want 42", got)
	}
}

// TestResilientEventStream_UpdatesSequenceNumOnEachDisconnect verifies that
// the resilient stream updates its lastSequenceNum on every disconnect,
// keeping the highest value seen.
func TestResilientEventStream_UpdatesSequenceNumOnEachDisconnect(t *testing.T) {
	t.Parallel()

	stream1 := &mockSeqStream{
		mockStream: mockStream{results: []mockResult{
			{event: Event{Type: "a"}},
			{err: errors.New("transient")},
		}},
		seqNum: 10,
	}
	stream2 := &mockSeqStream{
		mockStream: mockStream{results: []mockResult{
			{event: Event{Type: "b"}},
			{err: errors.New("transient")},
		}},
		seqNum: 25,
	}
	stream3 := newMockStream([]mockResult{{event: Event{Type: "c"}}})

	var callCount atomic.Int32
	dialer := func(ctx context.Context, lastSeqNum int64) (EventStream, error) {
		c := callCount.Add(1)
		switch c {
		case 1:
			return stream1, nil
		case 2:
			if lastSeqNum != 10 {
				t.Errorf("second dial lastSeqNum = %d, want 10", lastSeqNum)
			}
			return stream2, nil
		default:
			if lastSeqNum != 25 {
				t.Errorf("third dial lastSeqNum = %d, want 25", lastSeqNum)
			}
			return stream3, nil
		}
	}

	config := BackoffConfig{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		Multiplier:      2.0,
		JitterFraction:  0,
		MaxRetries:      5,
	}

	r := NewResilientEventStream(dialer, config, nil)
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Trigger first disconnect.
	_, _ = r.Recv(ctx)
	_, _ = r.Recv(ctx)
	time.Sleep(200 * time.Millisecond)

	// Trigger second disconnect.
	_, _ = r.Recv(ctx)
	_, _ = r.Recv(ctx)
	time.Sleep(200 * time.Millisecond)

	// Final read.
	_, err := r.Recv(ctx)
	if err != nil {
		t.Fatalf("final Recv error = %v", err)
	}

	if got := r.GetLastSequenceNum(); got != 25 {
		t.Fatalf("GetLastSequenceNum() = %d, want 25", got)
	}
}
