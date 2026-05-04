package preventsleep

import (
	"context"
	"os/exec"
	"runtime"
	"sync"
	"testing"
	"time"
)

// fakeSpawner returns a spawner that produces a short-lived sleep command.
func fakeSpawner() func(ctx context.Context) *exec.Cmd {
	return func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "3600")
	}
}

func resetState() {
	mu.Lock()
	defer mu.Unlock()
	refCount = 0
	if proc != nil {
		_ = proc.Process.Kill()
		proc = nil
	}
	if cancelRestartLoop != nil {
		cancelRestartLoop()
		cancelRestartLoop = nil
	}
	cleanupRegistered = false
}

func TestStartStop_ReferenceCount(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("reference count logic only runs on darwin")
	}

	resetState()
	defer resetState()

	oldSpawner := spawner
	spawner = fakeSpawner()
	defer func() { spawner = oldSpawner }()

	Start()
	Start()
	Start()

	mu.Lock()
	if refCount != 3 {
		t.Errorf("expected refCount 3, got %d", refCount)
	}
	mu.Unlock()

	Stop()
	Stop()

	mu.Lock()
	if refCount != 1 {
		t.Errorf("expected refCount 1 after two stops, got %d", refCount)
	}
	mu.Unlock()

	Stop()

	mu.Lock()
	if refCount != 0 {
		t.Errorf("expected refCount 0 after all stops, got %d", refCount)
	}
	if proc != nil {
		t.Error("expected proc nil after refcount reaches 0")
	}
	mu.Unlock()
}

func TestStop_BeyondZero(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("only relevant on darwin")
	}

	resetState()
	defer resetState()

	mu.Lock()
	refCount = 0
	mu.Unlock()

	Stop()

	mu.Lock()
	if refCount != 0 {
		t.Errorf("expected refCount to stay at 0, got %d", refCount)
	}
	mu.Unlock()
}

func TestForceStop(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("only relevant on darwin")
	}

	resetState()
	defer resetState()

	oldSpawner := spawner
	spawner = fakeSpawner()
	defer func() { spawner = oldSpawner }()

	Start()
	Start()

	mu.Lock()
	if refCount != 2 {
		t.Fatalf("expected refCount 2, got %d", refCount)
	}
	mu.Unlock()

	ForceStop()

	mu.Lock()
	if refCount != 0 {
		t.Errorf("expected refCount 0 after ForceStop, got %d", refCount)
	}
	if proc != nil {
		t.Error("expected proc nil after ForceStop")
	}
	mu.Unlock()
}

func TestForceStop_Idempotent(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("only relevant on darwin")
	}

	resetState()
	defer resetState()

	ForceStop()
	ForceStop()

	mu.Lock()
	if refCount != 0 {
		t.Errorf("expected refCount 0 after double ForceStop, got %d", refCount)
	}
	mu.Unlock()
}

func TestInit_ReturnsCleanup(t *testing.T) {
	resetState()
	defer resetState()

	cleanup := Init(InitOptions{})
	if cleanup == nil {
		t.Fatal("expected non-nil cleanup from Init")
	}
	// Cleanup should be safe to call multiple times.
	cleanup()
	cleanup()
}

func TestInit_CleanupRegisteredTwice(t *testing.T) {
	resetState()
	defer resetState()

	cleanup1 := Init(InitOptions{})
	cleanup2 := Init(InitOptions{})

	// Second init should still return a valid cleanup.
	if cleanup2 == nil {
		t.Fatal("expected non-nil cleanup on second Init")
	}

	cleanup1()
	cleanup2()
}

func TestNonDarwin_StartStop(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skip on darwin")
	}

	// On non-darwin, Start/Stop/ForceStop are no-ops and must not panic.
	Start()
	Start()
	Stop()
	ForceStop()
}

func TestNonDarwin_Init(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skip on darwin")
	}

	resetState()
	defer resetState()

	cleanup := Init(InitOptions{})
	if cleanup == nil {
		t.Fatal("expected non-nil cleanup even on non-darwin")
	}
	cleanup()
}

func TestStartStop_Concurrent(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("only relevant on darwin")
	}

	resetState()
	defer resetState()

	oldSpawner := spawner
	spawner = fakeSpawner()
	defer func() { spawner = oldSpawner }()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Start()
		}()
	}
	wg.Wait()

	mu.Lock()
	if refCount != 20 {
		t.Errorf("expected refCount 20 after concurrent starts, got %d", refCount)
	}
	mu.Unlock()

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Stop()
		}()
	}
	wg.Wait()

	mu.Lock()
	if refCount != 0 {
		t.Errorf("expected refCount 0 after concurrent stops, got %d", refCount)
	}
	mu.Unlock()
}

func TestRestartLoop(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("only relevant on darwin")
	}

	resetState()
	defer resetState()

	spawnCount := 0
	oldSpawner := spawner
	spawner = func(ctx context.Context) *exec.Cmd {
		spawnCount++
		return exec.CommandContext(ctx, "sleep", "3600")
	}
	defer func() { spawner = oldSpawner }()

	Start()
	defer ForceStop()

	// Wait a bit for the restart loop to tick (4 min is too long for a test,
	// but we can verify the loop was started by checking proc is non-nil).
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if proc == nil {
		t.Error("expected proc non-nil after Start")
	}
	if cancelRestartLoop == nil {
		t.Error("expected restart loop running")
	}
	mu.Unlock()

	// Force a restart manually by simulating what the loop does.
	mu.Lock()
	killCaffeinateLocked()
	spawnCaffeinateLocked()
	mu.Unlock()

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if proc == nil {
		t.Error("expected proc non-nil after manual restart")
	}
	mu.Unlock()
}

func TestItoa(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{123, "123"},
		{-5, "-5"},
		{300, "300"},
	}
	for _, c := range cases {
		got := itoa(c.in)
		if got != c.want {
			t.Errorf("itoa(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
