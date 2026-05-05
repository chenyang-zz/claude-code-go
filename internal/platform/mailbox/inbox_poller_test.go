package mailbox

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestNewInboxPoller(t *testing.T) {
	p := NewInboxPoller("test-agent", "test-team", "/tmp", 0, nil)
	if p == nil {
		t.Fatal("expected non-nil poller")
	}
	if p.agentName != "test-agent" {
		t.Errorf("agentName = %q, want %q", p.agentName, "test-agent")
	}
	if p.teamName != "test-team" {
		t.Errorf("teamName = %q, want %q", p.teamName, "test-team")
	}
	if p.interval != 1*time.Second {
		t.Errorf("interval = %v, want 1s", p.interval)
	}
	if p.running {
		t.Error("poller should not be running initially")
	}
}

func TestInboxPoller_StartStop(t *testing.T) {
	p := NewInboxPoller("test-agent", "test-team", "/tmp", 100*time.Millisecond, nil)

	p.Start()
	if !p.IsRunning() {
		t.Error("poller should be running after Start()")
	}

	// Double start should be a no-op
	p.Start()
	if !p.IsRunning() {
		t.Error("poller should still be running")
	}

	p.Stop()
	if p.IsRunning() {
		t.Error("poller should not be running after Stop()")
	}

	// Double stop should be a no-op
	p.Stop()
	if p.IsRunning() {
		t.Error("poller should still not be running")
	}
}

func TestInboxPoller_CallbackInvoked(t *testing.T) {
	// Use a non-existent directory so ReadUnreadMessages returns error,
	// but the poller should still run without panicking
	var callCount atomic.Int32
	p := NewInboxPoller("test-agent", "nonexistent-team", "/nonexistent", 50*time.Millisecond,
		func(msg []Message) {
			callCount.Add(1)
		})

	p.Start()
	time.Sleep(120 * time.Millisecond)
	p.Stop()

	// Callback may have been invoked 2-3 times depending on timing
	// (but ReadUnreadMessages will fail, so callCount stays 0)
	// This test just verifies no panic
}

func TestInboxPoller_CallbackOnMessages(t *testing.T) {
	// Use a non-existent inbox to verify poller doesn't panic
	// on file-not-found errors from ReadUnreadMessages
	var received atomic.Int32
	p := NewInboxPoller("ghost-agent", "test-team", "/tmp/ghost", 50*time.Millisecond,
		func(msg []Message) {
			received.Add(int32(len(msg)))
		})

	p.Start()
	time.Sleep(120 * time.Millisecond)
	p.Stop()

	// The callback may or may not be called depending on file existence
	// but the poller should not panic or hang
}

func TestInboxPoller_PollAfterStop(t *testing.T) {
	p := NewInboxPoller("test-agent", "test-team", "/tmp", 50*time.Millisecond,
		func(msg []Message) {})

	p.Start()
	time.Sleep(50 * time.Millisecond)
	p.Stop()

	// After stop, the goroutine should exit
	time.Sleep(100 * time.Millisecond)

	// Verify no deadlock or panic
}
