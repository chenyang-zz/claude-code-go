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

	client, err := DialSSE(context.Background(), server.URL, nil, server.Client(), 0)
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

	_, err := DialSSE(context.Background(), "ws://localhost:1234/stream", nil, http.DefaultClient, 0)
	if err == nil {
		t.Fatalf("DialSSE() error = nil, want invalid scheme error")
	}
}

// TestDialSSE_InitialSequenceNumHeaders verifies that a non-zero
// initialSequenceNum produces from_sequence_num query parameter and
// Last-Event-ID header.
func TestDialSSE_InitialSequenceNumHeaders(t *testing.T) {
	t.Parallel()

	var gotQuery, gotHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("from_sequence_num")
		gotHeader = r.Header.Get("Last-Event-ID")
		w.Header().Set("content-type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = fmt.Fprint(w, "data: ok\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client, err := DialSSE(context.Background(), server.URL, nil, server.Client(), 42)
	if err != nil {
		t.Fatalf("DialSSE() error = %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = client.Recv(ctx)

	if gotQuery != "42" {
		t.Fatalf("from_sequence_num = %q, want %q", gotQuery, "42")
	}
	if gotHeader != "42" {
		t.Fatalf("Last-Event-ID = %q, want %q", gotHeader, "42")
	}
}

// TestSSEClient_GetLastSequenceNum verifies that sequence numbers parsed from
// event IDs update the high-water mark.
func TestSSEClient_GetLastSequenceNum(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = fmt.Fprint(w, "id: 10\nevent: msg\ndata: a\n\n")
		flusher.Flush()
		// Small delay so the client has a chance to read the first event
		// before the second one arrives.
		time.Sleep(50 * time.Millisecond)
		_, _ = fmt.Fprint(w, "id: 20\nevent: msg\ndata: b\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client, err := DialSSE(context.Background(), server.URL, nil, server.Client(), 0)
	if err != nil {
		t.Fatalf("DialSSE() error = %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := client.Recv(ctx); err != nil {
		t.Fatalf("first Recv error = %v", err)
	}
	if got := client.GetLastSequenceNum(); got != 10 {
		t.Fatalf("after first event lastSequenceNum = %d, want 10", got)
	}

	if _, err := client.Recv(ctx); err != nil {
		t.Fatalf("second Recv error = %v", err)
	}
	if got := client.GetLastSequenceNum(); got != 20 {
		t.Fatalf("after second event lastSequenceNum = %d, want 20", got)
	}
}

// TestSSEClient_Deduplication verifies that duplicate sequence numbers are
// detected but events are still delivered.
func TestSSEClient_Deduplication(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = fmt.Fprint(w, "id: 5\nevent: msg\ndata: first\n\n")
		_, _ = fmt.Fprint(w, "id: 5\nevent: msg\ndata: dup\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client, err := DialSSE(context.Background(), server.URL, nil, server.Client(), 0)
	if err != nil {
		t.Fatalf("DialSSE() error = %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	e1, err := client.Recv(ctx)
	if err != nil {
		t.Fatalf("first Recv error = %v", err)
	}
	if string(e1.Data) != "first" {
		t.Fatalf("first event data = %q, want %q", e1.Data, "first")
	}

	e2, err := client.Recv(ctx)
	if err != nil {
		t.Fatalf("second Recv error = %v", err)
	}
	// Duplicate events are still delivered; dedup is logged only.
	if string(e2.Data) != "dup" {
		t.Fatalf("second event data = %q, want %q", e2.Data, "dup")
	}

	// Both events should have updated lastSequenceNum (same value).
	if got := client.GetLastSequenceNum(); got != 5 {
		t.Fatalf("lastSequenceNum = %d, want 5", got)
	}
}

// TestSSEClient_SequenceNumPruning verifies that the seenSequenceNums set is
// pruned when it grows beyond the maximum size.
func TestSSEClient_SequenceNumPruning(t *testing.T) {
	t.Parallel()

	// Emit 1002 events with monotonically increasing sequence numbers.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for i := int64(1); i <= 1002; i++ {
			_, _ = fmt.Fprintf(w, "id: %d\nevent: msg\ndata: x\n\n", i)
		}
		flusher.Flush()
	}))
	defer server.Close()

	client, err := DialSSE(context.Background(), server.URL, nil, server.Client(), 0)
	if err != nil {
		t.Fatalf("DialSSE() error = %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < 1002; i++ {
		if _, err := client.Recv(ctx); err != nil {
			t.Fatalf("Recv #%d error = %v", i+1, err)
		}
	}

	if got := client.GetLastSequenceNum(); got != 1002 {
		t.Fatalf("lastSequenceNum = %d, want 1002", got)
	}

	// After pruning, the set should contain at most ~1000 entries.
	// The exact count depends on the threshold = 1002 - 200 = 802.
	// Entries < 802 are pruned, so we expect ~200 entries.
	client.seqMu.RLock()
	count := len(client.seenSequenceNums)
	client.seqMu.RUnlock()

	if count > 1000 {
		t.Fatalf("seenSequenceNums size = %d, want <= 1000 after pruning", count)
	}
	if count < 150 {
		t.Fatalf("seenSequenceNums size = %d, too few after pruning", count)
	}
}
