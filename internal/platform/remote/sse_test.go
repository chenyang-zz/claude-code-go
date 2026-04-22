package remote

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestDialSSERecv verifies SSE connections emit normalized events.
func TestDialSSERecv(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected response writer to support flushing")
		}

		_, _ = fmt.Fprint(w, "id: 42\n")
		_, _ = fmt.Fprint(w, "event: client_event\n")
		_, _ = fmt.Fprint(w, "data: {\"type\":\"message\"}\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client, err := DialSSE(context.Background(), server.URL, nil, server.Client())
	if err != nil {
		t.Fatalf("DialSSE() error = %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	event, err := client.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv() error = %v", err)
	}
	if event.Transport != TransportSSE {
		t.Fatalf("event.Transport = %q, want %q", event.Transport, TransportSSE)
	}
	if event.Type != "client_event" {
		t.Fatalf("event.Type = %q, want %q", event.Type, "client_event")
	}
	if event.ID != "42" {
		t.Fatalf("event.ID = %q, want %q", event.ID, "42")
	}
	if got := string(event.Data); got != "{\"type\":\"message\"}" {
		t.Fatalf("event.Data = %q, want payload json", got)
	}
}

// TestDialSSERejectsInvalidScheme verifies non-http(s) endpoints fail fast.
func TestDialSSERejectsInvalidScheme(t *testing.T) {
	t.Parallel()

	_, err := DialSSE(context.Background(), "ws://localhost:1234/stream", nil, http.DefaultClient)
	if err == nil {
		t.Fatalf("DialSSE() error = nil, want invalid scheme error")
	}
}
