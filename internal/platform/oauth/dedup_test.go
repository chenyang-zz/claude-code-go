package oauth

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRefreshDeduper_SingleCallerInvokesFnOnce(t *testing.T) {
	d := NewRefreshDeduper()
	var calls int32

	tokens, err := d.Do("key-A", func() (*OAuthTokens, error) {
		atomic.AddInt32(&calls, 1)
		return &OAuthTokens{AccessToken: "tok-A"}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens == nil || tokens.AccessToken != "tok-A" {
		t.Fatalf("unexpected tokens: %+v", tokens)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected fn called once, got %d", got)
	}
	if got := d.PendingCount(); got != 0 {
		t.Fatalf("expected pending map cleared after Do, got %d entries", got)
	}
}

func TestRefreshDeduper_ConcurrentCallersShareSingleInvocation(t *testing.T) {
	d := NewRefreshDeduper()
	var calls int32

	// Block fn until all callers have entered the deduper, then release.
	release := make(chan struct{})

	const callers = 32
	results := make(chan *OAuthTokens, callers)
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	wg.Add(callers)
	for i := 0; i < callers; i++ {
		go func() {
			defer wg.Done()
			tokens, err := d.Do("shared-key", func() (*OAuthTokens, error) {
				atomic.AddInt32(&calls, 1)
				<-release
				return &OAuthTokens{AccessToken: "tok-shared"}, nil
			})
			results <- tokens
			errs <- err
		}()
	}

	// Give the goroutines time to all subscribe to the same key.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	close(results)
	close(errs)

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected fn called exactly once across %d callers, got %d", callers, got)
	}
	for tokens := range results {
		if tokens == nil || tokens.AccessToken != "tok-shared" {
			t.Fatalf("expected every caller to observe shared tokens, got %+v", tokens)
		}
	}
	for err := range errs {
		if err != nil {
			t.Fatalf("unexpected error from caller: %v", err)
		}
	}
}

func TestRefreshDeduper_DifferentKeysAreIndependent(t *testing.T) {
	d := NewRefreshDeduper()
	var calls int32

	// fn for key-A blocks until released so that key-B must proceed
	// independently.
	releaseA := make(chan struct{})
	doneA := make(chan struct{})

	go func() {
		_, _ = d.Do("key-A", func() (*OAuthTokens, error) {
			atomic.AddInt32(&calls, 1)
			<-releaseA
			return &OAuthTokens{AccessToken: "tok-A"}, nil
		})
		close(doneA)
	}()

	// Wait until A has registered.
	for i := 0; i < 50 && d.PendingCount() == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}

	// B should not block on A.
	tokensB, errB := d.Do("key-B", func() (*OAuthTokens, error) {
		atomic.AddInt32(&calls, 1)
		return &OAuthTokens{AccessToken: "tok-B"}, nil
	})
	if errB != nil {
		t.Fatalf("unexpected error from key-B: %v", errB)
	}
	if tokensB == nil || tokensB.AccessToken != "tok-B" {
		t.Fatalf("expected tok-B, got %+v", tokensB)
	}

	close(releaseA)
	<-doneA

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected one invocation per distinct key, got %d", got)
	}
}

func TestRefreshDeduper_PropagatesErrorAndReleasesPending(t *testing.T) {
	d := NewRefreshDeduper()
	wantErr := errors.New("boom")

	tokens, err := d.Do("key-err", func() (*OAuthTokens, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error to propagate, got %v", err)
	}
	if tokens != nil {
		t.Fatalf("expected nil tokens on error, got %+v", tokens)
	}
	if got := d.PendingCount(); got != 0 {
		t.Fatalf("pending entry must be cleared after error, got %d", got)
	}

	// Subsequent call with the same key should invoke fn again.
	var called bool
	_, _ = d.Do("key-err", func() (*OAuthTokens, error) {
		called = true
		return &OAuthTokens{AccessToken: "tok-recovered"}, nil
	})
	if !called {
		t.Fatal("expected fn to be invoked again after previous error cleared")
	}
}
