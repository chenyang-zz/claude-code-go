package analytics

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestKillSwitchSink_Emit_Normal(t *testing.T) {
	var count int64
	recorder := &countRecorder{count: &count}
	ks := NewKillSwitchSink(SinkDatadog, recorder)

	err := ks.Emit(context.Background(), Event{Name: "test.event", Metadata: NewMetadata("s-1")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&count) != 1 {
		t.Errorf("expected 1 emit, got %d", atomic.LoadInt64(&count))
	}
}

func TestKillSwitchSink_Emit_Killed(t *testing.T) {
	var count int64
	recorder := &countRecorder{count: &count}
	ks := NewKillSwitchSink(SinkDatadog, recorder)

	// Kill the switch
	ks.SetKilled(true)
	if !ks.IsKilled() {
		t.Error("expected IsKilled()=true")
	}

	err := ks.Emit(context.Background(), Event{Name: "test.event", Metadata: NewMetadata("s-1")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&count) != 0 {
		t.Errorf("expected 0 emits (killed), got %d", atomic.LoadInt64(&count))
	}
}

func TestKillSwitchSink_RuntimeToggle(t *testing.T) {
	var count int64
	recorder := &countRecorder{count: &count}
	ks := NewKillSwitchSink(SinkDatadog, recorder)

	// Start active
	_ = ks.Emit(context.Background(), Event{Name: "e1", Metadata: NewMetadata("s-1")})
	if atomic.LoadInt64(&count) != 1 {
		t.Fatalf("expected 1 emit, got %d", atomic.LoadInt64(&count))
	}

	// Kill
	ks.SetKilled(true)
	_ = ks.Emit(context.Background(), Event{Name: "e2", Metadata: NewMetadata("s-1")})
	_ = ks.Emit(context.Background(), Event{Name: "e3", Metadata: NewMetadata("s-1")})
	if atomic.LoadInt64(&count) != 1 {
		t.Errorf("expected still 1 emit after kill, got %d", atomic.LoadInt64(&count))
	}

	// Restore
	ks.SetKilled(false)
	if ks.IsKilled() {
		t.Error("expected IsKilled()=false after restore")
	}
	_ = ks.Emit(context.Background(), Event{Name: "e4", Metadata: NewMetadata("s-1")})
	if atomic.LoadInt64(&count) != 2 {
		t.Errorf("expected 2 emits after restore, got %d", atomic.LoadInt64(&count))
	}
}

func TestKillSwitchSink_InitialState(t *testing.T) {
	ks := NewKillSwitchSink(SinkDatadog, &countRecorder{})
	if ks.IsKilled() {
		t.Error("expected initial state to be non-killed")
	}

	ks2 := NewKillSwitchSink(SinkFirstParty, &countRecorder{})
	if ks2.IsKilled() {
		t.Error("expected initial state to be non-killed")
	}
}

func TestKillSwitchSink_ErrorPropagation(t *testing.T) {
	expectedErr := "sink error"
	errSink := &errorSink{msg: expectedErr}
	ks := NewKillSwitchSink(SinkDatadog, errSink)

	err := ks.Emit(context.Background(), Event{Name: "test", Metadata: NewMetadata("s-1")})
	if err == nil {
		t.Fatal("expected error from wrapped sink")
	}
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestKillSwitchSink_NoErrorWhenKilled(t *testing.T) {
	errSink := &errorSink{msg: "should not be reached"}
	ks := NewKillSwitchSink(SinkDatadog, errSink)
	ks.SetKilled(true)

	err := ks.Emit(context.Background(), Event{Name: "test", Metadata: NewMetadata("s-1")})
	if err != nil {
		t.Fatalf("expected nil error when killed, got %v", err)
	}
}

// countRecorder records the number of Emit calls without blocking.
type countRecorder struct {
	count *int64
}

func (r *countRecorder) Emit(_ context.Context, _ Event) error {
	if r.count != nil {
		atomic.AddInt64(r.count, 1)
	}
	return nil
}

// errorSink always returns an error on Emit.
type errorSink struct {
	msg string
}

func (s *errorSink) Emit(_ context.Context, _ Event) error {
	return &sinkError{msg: s.msg}
}

type sinkError struct {
	msg string
}

func (e *sinkError) Error() string { return e.msg }
