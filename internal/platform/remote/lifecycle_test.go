package remote

import (
	"context"
	"testing"
	"time"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

type fakeStreamFactory struct {
	stream EventStream
	err    error
	called bool
}

func (f *fakeStreamFactory) Open(ctx context.Context, session coreconfig.RemoteSessionConfig) (EventStream, error) {
	_ = ctx
	_ = session
	f.called = true
	if f.err != nil {
		return nil, f.err
	}
	return f.stream, nil
}

// TestLifecycleManagerSubscribe verifies lifecycle subscribe returns an unsubscribe handle that closes resources.
func TestLifecycleManagerSubscribe(t *testing.T) {
	t.Parallel()

	stream := newFakeStream()
	factory := &fakeStreamFactory{stream: stream}
	manager := NewLifecycleManager(NewSubscriptionManager(), factory)

	unsubscribe, err := manager.Subscribe(context.Background(), coreconfig.RemoteSessionConfig{
		Enabled:   true,
		SessionID: "session_test",
		StreamURL: "ws://localhost/test",
	}, nil)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	if !factory.called {
		t.Fatalf("stream factory should be called")
	}

	if err := unsubscribe(); err != nil {
		t.Fatalf("unsubscribe() error = %v", err)
	}

	// Allow teardown goroutine to release stream resources.
	deadline := time.Now().Add(2 * time.Second)
	for !stream.isClosed() {
		if time.Now().After(deadline) {
			t.Fatalf("stream should be closed after unsubscribe")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
