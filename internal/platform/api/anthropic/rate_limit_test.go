package anthropic

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

func TestClientStreamInvokesRateLimitConsumerOnSuccess(t *testing.T) {
	var (
		mu       sync.Mutex
		captured http.Header
		status   int
		errSeen  error
	)
	consumer := func(headers http.Header, s int, err error) {
		mu.Lock()
		defer mu.Unlock()
		captured = headers
		status = s
		errSeen = err
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("anthropic-ratelimit-unified-status", "allowed")
		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("event: message_stop\n"))
		_, _ = w.Write([]byte("data: {}\n\n"))
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		HTTPClient:        server.Client(),
		RateLimitConsumer: consumer,
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-6",
		Messages: []message.Message{{
			Role: message.RoleUser,
			Content: []message.ContentPart{{Type: "text", Text: "hi"}},
		}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	for range stream {
		// drain
	}

	mu.Lock()
	defer mu.Unlock()
	if captured == nil {
		t.Fatal("consumer was not invoked")
	}
	if got := captured.Get("anthropic-ratelimit-unified-status"); got != "allowed" {
		t.Fatalf("captured status header = %q", got)
	}
	if status != http.StatusOK {
		t.Fatalf("captured status = %d, want 200", status)
	}
	if errSeen != nil {
		t.Fatalf("captured err = %v, want nil", errSeen)
	}
}

func TestClientStreamRateLimitConsumerSurvivesPanic(t *testing.T) {
	consumer := func(_ http.Header, _ int, _ error) {
		panic("intentional panic for test")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("event: message_stop\n"))
		_, _ = w.Write([]byte("data: {}\n\n"))
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL,
		HTTPClient:        server.Client(),
		RateLimitConsumer: consumer,
	})

	// Stream should not panic even though the consumer does.
	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-6",
		Messages: []message.Message{{
			Role: message.RoleUser,
			Content: []message.ContentPart{{Type: "text", Text: "hi"}},
		}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	for range stream {
	}
}

func TestClientStreamErrorAnnotatorRewritesErrorMessage(t *testing.T) {
	annotator := func(err error, modelName string) error {
		if modelName != "claude-sonnet-4-6" {
			return nil
		}
		return errors.New("you've hit your session limit")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("anthropic-ratelimit-unified-status", "rejected")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"too many requests"}}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:                  "test-key",
		BaseURL:                 server.URL,
		HTTPClient:              server.Client(),
		RateLimitErrorAnnotator: annotator,
	})

	_, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-6",
		Messages: []message.Message{{
			Role: message.RoleUser,
			Content: []message.ContentPart{{Type: "text", Text: "hi"}},
		}},
	})
	if err == nil {
		t.Fatal("expected error from rate-limited response")
	}
	if err.Error() != "you've hit your session limit" {
		t.Fatalf("annotator did not rewrite error: got %q", err.Error())
	}
}

func TestClientStreamErrorAnnotatorReturnsNilLeavesErrorIntact(t *testing.T) {
	annotator := func(_ error, _ string) error { return nil }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"type":"server_error","message":"boom"}}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:                  "test-key",
		BaseURL:                 server.URL,
		HTTPClient:              server.Client(),
		RateLimitErrorAnnotator: annotator,
	})

	_, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-6",
		Messages: []message.Message{{
			Role: message.RoleUser,
			Content: []message.ContentPart{{Type: "text", Text: "hi"}},
		}},
	})
	if err == nil {
		t.Fatal("expected upstream error to flow through")
	}
}
