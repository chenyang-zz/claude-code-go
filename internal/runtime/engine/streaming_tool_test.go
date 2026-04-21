package engine

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// fakeStreamingExecute tracks tool invocations for test assertions.
type fakeStreamingExecute struct {
	mu        sync.Mutex
	calls     []coretool.Call
	results   map[string]coretool.Result
	errors    map[string]error
	delays    map[string]time.Duration
	callOrder []string // records the order in which tools start
	run       func(ctx context.Context, call coretool.Call) (coretool.Result, error)
}

func newFakeStreamingExecute() *fakeStreamingExecute {
	return &fakeStreamingExecute{
		results: make(map[string]coretool.Result),
		errors:  make(map[string]error),
		delays:  make(map[string]time.Duration),
	}
}

func (f *fakeStreamingExecute) execute(ctx context.Context, call coretool.Call, out chan<- event.Event) (coretool.Result, error) {
	f.mu.Lock()
	f.calls = append(f.calls, call)
	f.callOrder = append(f.callOrder, call.Name)
	f.mu.Unlock()

	if f.run != nil {
		return f.run(ctx, call)
	}

	if d, ok := f.delays[call.Name]; ok {
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return coretool.Result{}, ctx.Err()
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.errors[call.Name]; ok {
		return coretool.Result{}, err
	}
	if r, ok := f.results[call.Name]; ok {
		return r, nil
	}
	return coretool.Result{Output: "ok"}, nil
}

func (f *fakeStreamingExecute) getCalls() []coretool.Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]coretool.Call(nil), f.calls...)
}

func (f *fakeStreamingExecute) getCallOrder() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.callOrder...)
}

func TestStreamingToolExecutor_AddToolExecutesImmediately(t *testing.T) {
	fake := newFakeStreamingExecute()
	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		fake.execute,
		func(string) bool { return false },
		out,
		10,
	)

	exec.AddTool(context.Background(), model.ToolUse{ID: "t1", Name: "Read", Input: map[string]any{"path": "/tmp/x"}})

	// AwaitAll should complete without blocking.
	results := exec.AwaitAll(context.Background())
	if len(results) == 0 {
		t.Fatal("expected at least one result event")
	}

	calls := fake.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Name != "Read" {
		t.Errorf("expected tool name Read, got %s", calls[0].Name)
	}
}

func TestStreamingToolExecutor_ConcurrencySafeToolsRunInParallel(t *testing.T) {
	fake := newFakeStreamingExecute()
	// Both tools take 50ms, if parallel total time ~50ms, if serial ~100ms.
	fake.delays["ToolA"] = 50 * time.Millisecond
	fake.delays["ToolB"] = 50 * time.Millisecond

	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		fake.execute,
		func(name string) bool { return true }, // all tools concurrency-safe
		out,
		10,
	)

	start := time.Now()
	exec.AddTool(context.Background(), model.ToolUse{ID: "a", Name: "ToolA"})
	exec.AddTool(context.Background(), model.ToolUse{ID: "b", Name: "ToolB"})

	results := exec.AwaitAll(context.Background())
	elapsed := time.Since(start)

	if len(results) < 2 {
		t.Fatalf("expected at least 2 result events, got %d", len(results))
	}

	// If tools ran in parallel, total time should be closer to 50ms than 100ms.
	if elapsed > 90*time.Millisecond {
		t.Errorf("tools appear to have run sequentially (elapsed=%v), expected parallel execution", elapsed)
	}
}

func TestStreamingToolExecutor_NonSafeToolsRunSequentially(t *testing.T) {
	fake := newFakeStreamingExecute()
	fake.delays["ToolA"] = 50 * time.Millisecond
	fake.delays["ToolB"] = 50 * time.Millisecond

	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		fake.execute,
		func(name string) bool { return false }, // all tools NOT concurrency-safe
		out,
		10,
	)

	start := time.Now()
	exec.AddTool(context.Background(), model.ToolUse{ID: "a", Name: "ToolA"})
	exec.AddTool(context.Background(), model.ToolUse{ID: "b", Name: "ToolB"})

	results := exec.AwaitAll(context.Background())
	elapsed := time.Since(start)

	if len(results) < 2 {
		t.Fatalf("expected at least 2 result events, got %d", len(results))
	}

	// If tools ran sequentially, total time should be close to 100ms.
	if elapsed < 90*time.Millisecond {
		t.Errorf("tools appear to have run in parallel (elapsed=%v), expected sequential execution", elapsed)
	}
}

func TestStreamingToolExecutor_DiscardDropsResults(t *testing.T) {
	fake := newFakeStreamingExecute()
	fake.delays["SlowTool"] = 100 * time.Millisecond

	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		fake.execute,
		func(string) bool { return false },
		out,
		10,
	)

	exec.AddTool(context.Background(), model.ToolUse{ID: "s", Name: "SlowTool"})
	exec.Discard()

	// AwaitAll should return immediately when discarded.
	done := make(chan struct{})
	go func() {
		exec.AwaitAll(context.Background())
		close(done)
	}()

	select {
	case <-done:
		// Expected: AwaitAll returned quickly.
	case <-time.After(200 * time.Millisecond):
		t.Fatal("AwaitAll should return immediately after discard")
	}
}

func TestStreamingToolExecutor_CollectResultsPreservesOrder(t *testing.T) {
	fake := newFakeStreamingExecute()
	// Make first tool slow, second tool fast — results should still come in order.
	fake.delays["SlowTool"] = 50 * time.Millisecond

	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		fake.execute,
		func(string) bool { return false }, // sequential execution
		out,
		10,
	)

	exec.AddTool(context.Background(), model.ToolUse{ID: "a", Name: "SlowTool"})
	exec.AddTool(context.Background(), model.ToolUse{ID: "b", Name: "FastTool"})

	// AwaitAll waits for all tools and returns their results in order.
	results := exec.AwaitAll(context.Background())
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// Verify order: SlowTool (a) should come before FastTool (b).
	payload0 := results[0].Payload.(event.ToolResultPayload)
	payload1 := results[1].Payload.(event.ToolResultPayload)
	if payload0.ID != "a" {
		t.Errorf("expected first result to be tool 'a', got %s", payload0.ID)
	}
	if payload1.ID != "b" {
		t.Errorf("expected second result to be tool 'b', got %s", payload1.ID)
	}
}

func TestStreamingToolExecutor_ToolUseCount(t *testing.T) {
	fake := newFakeStreamingExecute()
	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		fake.execute,
		func(string) bool { return false },
		out,
		10,
	)

	if count := exec.ToolUseCount(); count != 0 {
		t.Errorf("expected 0 tools, got %d", count)
	}

	exec.AddTool(context.Background(), model.ToolUse{ID: "a", Name: "ToolA"})
	if count := exec.ToolUseCount(); count != 1 {
		t.Errorf("expected 1 tool, got %d", count)
	}

	exec.AddTool(context.Background(), model.ToolUse{ID: "b", Name: "ToolB"})
	if count := exec.ToolUseCount(); count != 2 {
		t.Errorf("expected 2 tools, got %d", count)
	}
}

func TestStreamingToolExecutor_BuildToolResultMessage(t *testing.T) {
	fake := newFakeStreamingExecute()
	fake.results["ToolA"] = coretool.Result{Output: "result-a"}
	fake.results["ToolB"] = coretool.Result{Output: "result-b"}

	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		fake.execute,
		func(string) bool { return true },
		out,
		10,
	)

	exec.AddTool(context.Background(), model.ToolUse{ID: "a", Name: "ToolA"})
	exec.AddTool(context.Background(), model.ToolUse{ID: "b", Name: "ToolB"})
	exec.AwaitAll(context.Background())

	msg := exec.BuildToolResultMessage()
	if msg.Role != "user" {
		t.Errorf("expected role 'user', got %s", msg.Role)
	}
	if len(msg.Content) != 2 {
		t.Fatalf("expected 2 content parts, got %d", len(msg.Content))
	}
}

func TestStreamingToolExecutor_MaxConcurrentLimit(t *testing.T) {
	var activeCount atomic.Int32
	var maxActive atomic.Int32
	fake := newFakeStreamingExecute()
	fake.delays["Tool"] = 50 * time.Millisecond
	fake.run = func(ctx context.Context, call coretool.Call) (coretool.Result, error) {
		current := activeCount.Add(1)
		for {
			old := maxActive.Load()
			if current <= old || maxActive.CompareAndSwap(old, current) {
				break
			}
		}
		defer activeCount.Add(-1)
		select {
		case <-time.After(50 * time.Millisecond):
		case <-ctx.Done():
		}
		return coretool.Result{Output: "ok"}, nil
	}

	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		func(ctx context.Context, call coretool.Call, _ chan<- event.Event) (coretool.Result, error) {
			return fake.run(ctx, call)
		},
		func(string) bool { return true }, // all concurrency-safe
		out,
		2, // max 2 concurrent
	)

	exec.AddTool(context.Background(), model.ToolUse{ID: "1", Name: "Tool"})
	exec.AddTool(context.Background(), model.ToolUse{ID: "2", Name: "Tool"})
	exec.AddTool(context.Background(), model.ToolUse{ID: "3", Name: "Tool"})

	exec.AwaitAll(context.Background())

	if m := maxActive.Load(); m > 2 {
		t.Errorf("expected max 2 concurrent tools, got %d", m)
	}
}

func TestStreamingToolExecutor_PreservesResultsAcrossSliceReallocation(t *testing.T) {
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})

	fake := newFakeStreamingExecute()
	fake.run = func(ctx context.Context, call coretool.Call) (coretool.Result, error) {
		if call.ID == "1" {
			close(firstStarted)
			select {
			case <-releaseFirst:
			case <-ctx.Done():
				return coretool.Result{}, ctx.Err()
			}
		}
		return coretool.Result{Output: "result-" + call.ID}, nil
	}

	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		func(ctx context.Context, call coretool.Call, _ chan<- event.Event) (coretool.Result, error) {
			return fake.run(ctx, call)
		},
		func(string) bool { return true },
		out,
		10,
	)

	exec.AddTool(context.Background(), model.ToolUse{ID: "1", Name: "Tool"})
	<-firstStarted
	// Adding a second tool after the first goroutine starts forces append to grow
	// the backing slice from capacity 1, which previously stranded the first result.
	exec.AddTool(context.Background(), model.ToolUse{ID: "2", Name: "Tool"})
	close(releaseFirst)

	results := exec.AwaitAll(context.Background())
	if len(results) != 2 {
		t.Fatalf("expected 2 results after slice growth, got %d", len(results))
	}

	msg := exec.BuildToolResultMessage()
	if len(msg.Content) != 2 {
		t.Fatalf("expected 2 tool_result parts after slice growth, got %d", len(msg.Content))
	}
}
