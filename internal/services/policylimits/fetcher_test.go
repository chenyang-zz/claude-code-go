package policylimits

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
)

func TestFetch_Success(t *testing.T) {
	restrictions := map[string]Restriction{
		"allow_remote_sessions": {Allowed: false},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "key" {
			t.Error("expected x-api-key header")
		}
		_ = json.NewEncoder(w).Encode(PolicyLimitsResponse{Restrictions: restrictions})
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := Fetch(ctx, "key", "", ts.URL, "")
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Restrictions) != 1 {
		t.Fatalf("expected 1 restriction, got %d", len(resp.Restrictions))
	}
}

func TestFetch_304(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != `"abc"` {
			t.Errorf("unexpected If-None-Match: %q", r.Header.Get("If-None-Match"))
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := Fetch(ctx, "key", "", ts.URL, "abc")
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if resp != nil {
		t.Error("304 should return nil response")
	}
}

func TestFetch_404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := Fetch(ctx, "key", "", ts.URL, "")
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response for 404")
	}
	if len(resp.Restrictions) != 0 {
		t.Error("404 should return empty restrictions")
	}
}

func TestFetch_OAuthBearer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer token" {
			t.Errorf("expected Bearer token, got %q", auth)
		}
		beta := r.Header.Get("anthropic-beta")
		if beta != oauth.OAuthBetaHeader {
			t.Errorf("expected anthropic-beta %q, got %q", oauth.OAuthBetaHeader, beta)
		}
		_ = json.NewEncoder(w).Encode(PolicyLimitsResponse{})
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := Fetch(ctx, "", "token", ts.URL, "")
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
}

func TestFetch_NoAuth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := Fetch(ctx, "", "", "", "")
	if err == nil {
		t.Error("expected error when no auth provided")
	}
}

func TestFetch_DefaultBaseURL(t *testing.T) {
	// This test just verifies no panic when baseURL is empty
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := Fetch(ctx, "key", "", "", "")
	if err == nil {
		t.Error("expected error when hitting default base URL with no server")
	}
}

func TestExponentialBackoff(t *testing.T) {
	d0 := exponentialBackoff(0)
	if d0 < baseDelay || d0 > baseDelay+time.Duration(float64(baseDelay)*jitterFrac) {
		t.Errorf("attempt 0 backoff out of range: %v", d0)
	}

	d4 := exponentialBackoff(4)
	if d4 > maxDelay {
		t.Errorf("attempt 4 backoff should not exceed maxDelay: %v", d4)
	}
}
