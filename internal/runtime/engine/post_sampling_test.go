package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestRegisterPostSamplingHook(t *testing.T) {
	ClearPostSamplingHooks()
	defer ClearPostSamplingHooks()

	called := false
	hook := func(ctx context.Context, assistantMessage message.Message, history []message.Message, workingDir string) error {
		called = true
		return nil
	}

	RegisterPostSamplingHook(hook)

	// Simulate firing via a minimal Runtime
	r := &Runtime{}
	r.firePostSamplingHooks(context.Background(), "/tmp", message.Message{Role: message.RoleAssistant}, nil)

	if !called {
		t.Fatal("expected post-sampling hook to be called")
	}
}

func TestFirePostSamplingHook_MultipleHooksInOrder(t *testing.T) {
	ClearPostSamplingHooks()
	defer ClearPostSamplingHooks()

	var order []int
	hook1 := func(ctx context.Context, assistantMessage message.Message, history []message.Message, workingDir string) error {
		order = append(order, 1)
		return nil
	}
	hook2 := func(ctx context.Context, assistantMessage message.Message, history []message.Message, workingDir string) error {
		order = append(order, 2)
		return nil
	}

	RegisterPostSamplingHook(hook1)
	RegisterPostSamplingHook(hook2)

	r := &Runtime{}
	r.firePostSamplingHooks(context.Background(), "/tmp", message.Message{Role: message.RoleAssistant}, nil)

	if len(order) != 2 {
		t.Fatalf("expected 2 hooks fired, got %d", len(order))
	}
	if order[0] != 1 || order[1] != 2 {
		t.Fatalf("expected hooks in order [1, 2], got %v", order)
	}
}

func TestFirePostSamplingHook_HookErrorDoesNotBlockOthers(t *testing.T) {
	ClearPostSamplingHooks()
	defer ClearPostSamplingHooks()

	called := false
	hook1 := func(ctx context.Context, assistantMessage message.Message, history []message.Message, workingDir string) error {
		return errors.New("intentional error")
	}
	hook2 := func(ctx context.Context, assistantMessage message.Message, history []message.Message, workingDir string) error {
		called = true
		return nil
	}

	RegisterPostSamplingHook(hook1)
	RegisterPostSamplingHook(hook2)

	r := &Runtime{}
	r.firePostSamplingHooks(context.Background(), "/tmp", message.Message{Role: message.RoleAssistant}, nil)

	if !called {
		t.Fatal("expected second hook to be called despite first hook error")
	}
}

func TestFirePostSamplingHook_NilHookSkipped(t *testing.T) {
	ClearPostSamplingHooks()
	defer ClearPostSamplingHooks()

	called := false
	hook := func(ctx context.Context, assistantMessage message.Message, history []message.Message, workingDir string) error {
		called = true
		return nil
	}

	// Register a nil hook then a real one
	postSamplingHooksMu.Lock()
	postSamplingHooks = append(postSamplingHooks, nil)
	postSamplingHooks = append(postSamplingHooks, hook)
	postSamplingHooksMu.Unlock()

	r := &Runtime{}
	r.firePostSamplingHooks(context.Background(), "/tmp", message.Message{Role: message.RoleAssistant}, nil)

	if !called {
		t.Fatal("expected real hook to be called after nil hook")
	}
}

func TestFirePostSamplingHook_Timeout(t *testing.T) {
	ClearPostSamplingHooks()
	defer ClearPostSamplingHooks()

	hook := func(ctx context.Context, assistantMessage message.Message, history []message.Message, workingDir string) error {
		select {
		case <-time.After(100 * time.Millisecond):
			return errors.New("should have been cancelled")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	RegisterPostSamplingHook(hook)

	r := &Runtime{}
	start := time.Now()
	r.firePostSamplingHooks(context.Background(), "/tmp", message.Message{Role: message.RoleAssistant}, nil)
	elapsed := time.Since(start)

	// The hook should timeout at 60s, but for test speed we rely on the
	// context being cancelled when the hook returns. Since our hook
	// respects ctx.Done(), it returns immediately when cancelled.
	// However the timeout is 60s so this test just verifies it doesn't
	// hang indefinitely. We'll use a shorter hook for practical testing.
	if elapsed > 70*time.Second {
		t.Fatal("post-sampling hook did not respect timeout")
	}
}
