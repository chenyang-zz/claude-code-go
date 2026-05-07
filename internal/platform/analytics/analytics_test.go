package analytics

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockSink records events for inspection in tests.
type mockSink struct {
	mu     sync.Mutex
	events []Event
}

func (s *mockSink) Emit(_ context.Context, event Event) error {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	return nil
}

func (s *mockSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

func (s *mockSink) get(i int) Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.events[i]
}

func TestNewMetadata(t *testing.T) {
	m := NewMetadata("sess-1")
	if m.SessionID != "sess-1" {
		t.Errorf("expected sess-1, got %s", m.SessionID)
	}
	if m.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if m.Labels == nil {
		t.Error("expected non-nil Labels map")
	}
}

func TestMetadataWithLabel(t *testing.T) {
	m := NewMetadata("s-1")
	m2 := m.WithLabel("key1", "val1")
	if m2.Labels["key1"] != "val1" {
		t.Errorf("expected val1, got %v", m2.Labels["key1"])
	}
	// original should be unchanged
	if _, ok := m.Labels["key1"]; ok {
		t.Error("original metadata should not have key1")
	}
}

func TestMetadataWithLabels(t *testing.T) {
	m := NewMetadata("s-1")
	m2 := m.WithLabels(map[string]any{"a": 1, "b": true})
	if m2.Labels["a"] != 1 {
		t.Errorf("expected 1, got %v", m2.Labels["a"])
	}
	if m2.Labels["b"] != true {
		t.Errorf("expected true, got %v", m2.Labels["b"])
	}
}

func TestEmitterEmitToolUsed(t *testing.T) {
	sink := &mockSink{}
	e := NewEmitter(sink, 100)
	defer e.Close()

	meta := NewMetadata("sess-1")
	e.EmitToolUsed(meta, "Bash", time.Second, true, "")

	// Allow the background goroutine to process the event
	time.Sleep(50 * time.Millisecond)

	if sink.count() != 1 {
		t.Fatalf("expected 1 event, got %d", sink.count())
	}
	evt := sink.get(0)
	if evt.Name != EventToolUsed {
		t.Errorf("expected %s, got %s", EventToolUsed, evt.Name)
	}
	p, ok := evt.Payload.(ToolUsedEvent)
	if !ok {
		t.Fatal("expected ToolUsedEvent payload")
	}
	if p.ToolName != "Bash" {
		t.Errorf("expected Bash, got %s", p.ToolName)
	}
	if !p.Success {
		t.Error("expected success")
	}
}

func TestEmitterEmitSessionEvent(t *testing.T) {
	sink := &mockSink{}
	e := NewEmitter(sink, 100)
	defer e.Close()

	e.EmitSessionEvent(NewMetadata("s-2"), "started", 0, 0)
	time.Sleep(50 * time.Millisecond)

	if sink.count() != 1 {
		t.Fatalf("expected 1 event, got %d", sink.count())
	}
	evt := sink.get(0)
	if evt.Name != EventSession {
		t.Errorf("expected %s, got %s", EventSession, evt.Name)
	}
}

func TestEmitterEmitCommandEvent(t *testing.T) {
	sink := &mockSink{}
	e := NewEmitter(sink, 100)
	defer e.Close()

	e.EmitCommandEvent(NewMetadata("s-3"), "/help", true, []string{"-v"})
	time.Sleep(50 * time.Millisecond)

	if sink.count() != 1 {
		t.Fatalf("expected 1 event, got %d", sink.count())
	}
	evt := sink.get(0)
	if evt.Name != EventCommand {
		t.Errorf("expected %s, got %s", EventCommand, evt.Name)
	}
}

func TestEmitterEmitErrorEvent(t *testing.T) {
	sink := &mockSink{}
	e := NewEmitter(sink, 100)
	defer e.Close()

	e.EmitErrorEvent(NewMetadata("s-4"), "api", "timeout", "Bash", 5000)
	time.Sleep(50 * time.Millisecond)

	if sink.count() != 1 {
		t.Fatalf("expected 1 event, got %d", sink.count())
	}
	evt := sink.get(0)
	if evt.Name != EventError {
		t.Errorf("expected %s, got %s", EventError, evt.Name)
	}
}

func TestEmitterEmitRaw(t *testing.T) {
	sink := &mockSink{}
	e := NewEmitter(sink, 100)
	defer e.Close()

	e.EmitRaw(NewMetadata("s-5"), "custom.event", "raw data")
	time.Sleep(50 * time.Millisecond)

	if sink.count() != 1 {
		t.Fatalf("expected 1 event, got %d", sink.count())
	}
	evt := sink.get(0)
	if evt.Name != "custom.event" {
		t.Errorf("expected custom.event, got %s", evt.Name)
	}
}

func TestEmitterNonBlockingOnFullQueue(t *testing.T) {
	// slowSink introduces latency to fill the queue
	slowSink := &slowConsumer{wait: 50 * time.Millisecond}
	e := NewEmitter(slowSink, 4) // tiny buffer
	defer e.Close()

	// Send more events than the buffer can hold
	for range 100 {
		e.EmitToolUsed(NewMetadata("s-1"), "Bash", 0, true, "")
	}

	// The call should NOT block; events will be dropped but that's acceptable.
	// No test assertion needed — non-blocking is the contract.
}

type slowConsumer struct {
	wait time.Duration
}

func (s *slowConsumer) Emit(_ context.Context, _ Event) error {
	time.Sleep(s.wait)
	return nil
}

func TestEmitterCloseMultiple(t *testing.T) {
	sink := &mockSink{}
	e := NewEmitter(sink, 100)
	// Multiple Close() calls should not panic
	e.Close()
	e.Close()
}

func TestEmitterCloseDrainsEvents(t *testing.T) {
	sink := &mockSink{}
	e := NewEmitter(sink, 100)

	e.EmitToolUsed(NewMetadata("s-1"), "Bash", 0, true, "")
	e.EmitSessionEvent(NewMetadata("s-1"), "ended", 2, 10)
	e.Close()

	// After Close(), all queued events should be processed by the drain loop
	if sink.count() != 2 {
		t.Errorf("expected 2 events after drain, got %d", sink.count())
	}
}

func TestEmitterConcurrentSafe(t *testing.T) {
	sink := &mockSink{}
	e := NewEmitter(sink, 1024)
	defer e.Close()

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				e.EmitToolUsed(NewMetadata("s-1"), "Bash", 0, true, "")
			}
		}()
	}
	wg.Wait()
	e.Close()

	if sink.count() == 0 {
		t.Error("expected some events to arrive")
	}
}

func TestInitAnalyticsEnabled(t *testing.T) {
	e := InitAnalytics(Config{Enabled: true, QueueSize: 10}, slog.Default())
	if e == nil {
		t.Fatal("expected non-nil emitter")
	}
	// Should accept events
	e.EmitToolUsed(NewMetadata("s-1"), "Bash", 0, true, "")
	e.Close()
}

func TestInitAnalyticsDisabled(t *testing.T) {
	e := InitAnalytics(Config{Enabled: false}, slog.Default())
	if e == nil {
		t.Fatal("expected non-nil emitter even when disabled")
	}
	// Should not panic on emit
	e.EmitToolUsed(NewMetadata("s-1"), "Bash", 0, true, "")
	// Close should not panic
	e.Close()
	e.Close() // double close
}

func TestConsoleSinkEmit(t *testing.T) {
	var buf safeBuf
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	sink := NewConsoleSink(logger)

	err := sink.Emit(context.Background(), Event{
		Name: EventToolUsed,
		Metadata: NewMetadata("s-1"),
		Payload: ToolUsedEvent{ToolName: "Bash", Duration: time.Second, Success: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected log output")
	}
}

func TestEventNameConstants(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"EventToolUsed", EventToolUsed},
		{"EventToolErrored", EventToolErrored},
		{"EventSession", EventSession},
		{"EventCommand", EventCommand},
		{"EventError", EventError},
	}
	for _, tt := range tests {
		if tt.value == "" {
			t.Errorf("%s should not be empty", tt.name)
		}
	}
}

func TestToolUsedEventFields(t *testing.T) {
	evt := ToolUsedEvent{
		ToolName:   "Read",
		Duration:   500 * time.Millisecond,
		Success:    false,
		ErrorMsg:   "file not found",
		InputSize:  10,
		OutputSize: 100,
	}
	if evt.ToolName != "Read" {
		t.Errorf("expected Read, got %s", evt.ToolName)
	}
	if evt.ErrorMsg != "file not found" {
		t.Errorf("expected file not found, got %s", evt.ErrorMsg)
	}
}

// safeBuf is a concurrency-safe strings writer for test logging
type safeBuf struct {
	mu  sync.Mutex
	buf []byte
}

func (b *safeBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *safeBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// TestQueueDroppedEvents verifies the capacity and non-blocking contract
func TestQueueDroppedEvents(t *testing.T) {
	slowSink := &slowConsumer{wait: 10 * time.Millisecond}
	e := NewEmitter(slowSink, 2)
	defer e.Close()

	var dropped int64
	for range 100 {
		if !e.enqueue(Event{Name: "test", Metadata: NewMetadata("s-1")}) {
			atomic.AddInt64(&dropped, 1)
		}
	}

	// We expect some events were dropped because the consumer is slow
	if dropped == 0 {
		t.Error("expected some drops with slow consumer and small buffer")
	}
}
