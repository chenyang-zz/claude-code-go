package repl

import (
	"testing"
	"time"
)

func TestNewWakeupScheduler(t *testing.T) {
	w := NewWakeupScheduler()
	if w == nil {
		t.Fatal("NewWakeupScheduler returned nil")
	}
	if w.Pending() != nil {
		t.Error("new scheduler should have no pending wakeup")
	}
	w.Stop()
}

func TestWakeupScheduler_Fires(t *testing.T) {
	w := NewWakeupScheduler()
	defer w.Stop()

	w.Schedule(1, "test fire", "test prompt")

	select {
	case prompt := <-w.FireChan():
		if prompt != "test prompt" {
			t.Errorf("expected prompt %q, got %q", "test prompt", prompt)
		}
	case <-time.After(3 * time.Second):
		t.Error("wakeup did not fire within 3 seconds")
	}

	if w.Pending() != nil {
		t.Error("pending should be nil after fire")
	}
}

func TestWakeupScheduler_Cancel(t *testing.T) {
	w := NewWakeupScheduler()
	defer w.Stop()

	w.Schedule(10, "test cancel", "cancel prompt")
	if w.Pending() == nil {
		t.Fatal("expected pending wakeup after Schedule")
	}

	w.Cancel()
	if w.Pending() != nil {
		t.Error("pending should be nil after Cancel")
	}
}

func TestWakeupScheduler_Overwrite(t *testing.T) {
	w := NewWakeupScheduler()
	defer w.Stop()

	w.Schedule(10, "first", "first prompt")
	w.Schedule(1, "second", "second prompt")

	// Only the second should fire (overwrite).
	select {
	case prompt := <-w.FireChan():
		if prompt != "second prompt" {
			t.Errorf("expected %q, got %q", "second prompt", prompt)
		}
	case <-time.After(3 * time.Second):
		t.Error("second wakeup did not fire within 3 seconds")
	}

	// Verify first timer was cancelled (no double fire).
	select {
	case prompt := <-w.FireChan():
		t.Errorf("unexpected second fire with prompt: %s", prompt)
	case <-time.After(500 * time.Millisecond):
		// expected — no second fire
	}
}

func TestWakeupScheduler_Pending(t *testing.T) {
	w := NewWakeupScheduler()
	defer w.Stop()

	w.Schedule(600, "check reason", "loop prompt")
	p := w.Pending()
	if p == nil {
		t.Fatal("expected non-nil pending after Schedule")
	}
	if p.Reason != "check reason" {
		t.Errorf("expected reason %q, got %q", "check reason", p.Reason)
	}
	if p.Prompt != "loop prompt" {
		t.Errorf("expected prompt %q, got %q", "loop prompt", p.Prompt)
	}
	if p.DelaySeconds != 600 {
		t.Errorf("expected delay 600, got %d", p.DelaySeconds)
	}
}

func TestClampWakeupDelay(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, MinWakeupDelaySeconds},
		{30, MinWakeupDelaySeconds},
		{59, MinWakeupDelaySeconds},
		{60, 60},
		{120, 120},
		{3600, 3600},
		{3601, MaxWakeupDelaySeconds},
		{7200, MaxWakeupDelaySeconds},
	}

	for _, tc := range tests {
		got := ClampWakeupDelay(tc.input)
		if got != tc.expected {
			t.Errorf("ClampWakeupDelay(%d) = %d, want %d", tc.input, got, tc.expected)
		}
	}
}
