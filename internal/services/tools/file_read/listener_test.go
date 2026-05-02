package file_read

import (
	"sync"
	"testing"
)

func TestReadListener_RegisterAndNotify(t *testing.T) {
	// Clean up from other tests.
	ClearReadListeners()

	var (
		calledFile string
		calledContent string
	)
	listener := ReadListener(func(filePath string, content string) {
		calledFile = filePath
		calledContent = content
	})

	RegisterReadListener(listener)
	notifyReadListeners("/tmp/test.go", "package main")

	if calledFile != "/tmp/test.go" {
		t.Errorf("expected filePath %q, got %q", "/tmp/test.go", calledFile)
	}
	if calledContent != "package main" {
		t.Errorf("expected content %q, got %q", "package main", calledContent)
	}
}

func TestReadListener_MultipleListeners(t *testing.T) {
	ClearReadListeners()

	var mu sync.Mutex
	var calls []string
	makeListener := func(name string) ReadListener {
		return func(filePath string, content string) {
			mu.Lock()
			calls = append(calls, name)
			mu.Unlock()
		}
	}

	RegisterReadListener(makeListener("A"))
	RegisterReadListener(makeListener("B"))
	notifyReadListeners("/tmp/test.go", "hello")

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("expected 2 listeners called, got %d", len(calls))
	}
	if calls[0] != "A" || calls[1] != "B" {
		t.Errorf("expected listeners [A, B], got %v", calls)
	}
}

func TestReadListener_Unsubscribe(t *testing.T) {
	ClearReadListeners()

	callCount := 0
	listener := ReadListener(func(filePath string, content string) {
		callCount++
	})

	unsub := RegisterReadListener(listener)
	unsub()
	notifyReadListeners("/tmp/test.go", "hello")

	if callCount != 0 {
		t.Errorf("expected 0 calls after unsubscribe, got %d", callCount)
	}
}

func TestReadListener_ClearAll(t *testing.T) {
	ClearReadListeners()

	callCount := 0
	listener := ReadListener(func(filePath string, content string) {
		callCount++
	})

	RegisterReadListener(listener)
	ClearReadListeners()
	notifyReadListeners("/tmp/test.go", "hello")

	if callCount != 0 {
		t.Errorf("expected 0 calls after ClearReadListeners, got %d", callCount)
	}
}

func TestReadListener_NilListener(t *testing.T) {
	ClearReadListeners()

	// RegisterReadListener(nil) should return a no-op unsubscribe that doesn't panic.
	unsub := RegisterReadListener(nil)
	// This should not panic.
	unsub()

	// notifyReadListeners should also not panic (no real listeners registered).
	notifyReadListeners("/tmp/test.go", "hello")
}
