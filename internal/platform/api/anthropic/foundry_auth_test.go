package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

func TestAPIKeyFoundryAuthenticator(t *testing.T) {
	auth := &apiKeyFoundryAuthenticator{apiKey: "test-api-key-123"}
	req, _ := http.NewRequest("POST", "https://example.com", nil)

	err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error = %v, want nil", err)
	}

	if got := req.Header.Get("api-key"); got != "test-api-key-123" {
		t.Fatalf("api-key = %q, want test-api-key-123", got)
	}
}

func TestAPIKeyFoundryAuthenticator_MissingKey(t *testing.T) {
	auth := &apiKeyFoundryAuthenticator{}
	req, _ := http.NewRequest("POST", "https://example.com", nil)

	err := auth.Authenticate(req)
	if err == nil {
		t.Fatal("Authenticate() error = nil, want error for missing API key")
	}
}

func TestNoopFoundryAuthenticator(t *testing.T) {
	auth := &noopFoundryAuthenticator{}
	req, _ := http.NewRequest("POST", "https://example.com", nil)

	err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error = %v, want nil", err)
	}

	if req.Header.Get("api-key") != "" {
		t.Fatal("api-key should not be set by noop authenticator")
	}
}

func TestNewFoundryAuthenticator_Priority(t *testing.T) {
	// 1. Skip auth takes highest priority.
	auth := newFoundryAuthenticator(true, "config-key-123")
	req, _ := http.NewRequest("POST", "https://example.com", nil)
	err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("SignRequest() error = %v", err)
	}
	if req.Header.Get("api-key") != "" {
		t.Fatal("skip auth should not set api-key")
	}

	// 2. Config API key takes priority over env var.
	os.Setenv("ANTHROPIC_FOUNDRY_API_KEY", "env-key-456")
	defer os.Unsetenv("ANTHROPIC_FOUNDRY_API_KEY")
	auth = newFoundryAuthenticator(false, "config-key-123")
	req, _ = http.NewRequest("POST", "https://example.com", nil)
	err = auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if got := req.Header.Get("api-key"); got != "config-key-123" {
		t.Fatalf("config API key priority: api-key = %q, want config-key-123", got)
	}

	// 3. Env var when no config key.
	auth = newFoundryAuthenticator(false, "")
	req, _ = http.NewRequest("POST", "https://example.com", nil)
	err = auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if got := req.Header.Get("api-key"); got != "env-key-456" {
		t.Fatalf("env API key fallback: api-key = %q, want env-key-456", got)
	}

	// 4. Missing credentials when no key and not skipped.
	os.Unsetenv("ANTHROPIC_FOUNDRY_API_KEY")
	auth = newFoundryAuthenticator(false, "")
	req, _ = http.NewRequest("POST", "https://example.com", nil)
	err = auth.Authenticate(req)
	if err == nil {
		t.Fatal("Authenticate() error = nil, want error for missing credentials")
	}
}

func TestClientFoundryStream_MissingEndpoint(t *testing.T) {
	client := NewClient(Config{
		FoundryEnabled:  true,
		FoundrySkipAuth: true,
	})

	_, err := client.Stream(context.TODO(), model.Request{})
	if err == nil {
		t.Fatal("Stream() error = nil, want error for missing endpoint")
	}
}

func TestClientFoundryStream_MissingAPIKey(t *testing.T) {
	client := NewClient(Config{
		FoundryEnabled:  true,
		FoundryBaseURL:  "https://test-resource.services.ai.azure.com",
		FoundrySkipAuth: false,
	})

	_, err := client.Stream(context.TODO(), model.Request{})
	if err == nil {
		t.Fatal("Stream() error = nil, want error for missing API key")
	}
}

func TestClientFoundryStream_SendsToFoundryEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantSuffix := "/anthropic/v1/messages"
		if !strings.HasSuffix(r.URL.Path, wantSuffix) {
			t.Fatalf("request path = %q, want suffix %q", r.URL.Path, wantSuffix)
		}

		if got := r.Header.Get("api-key"); got != "foundry-test-key" {
			t.Fatalf("api-key = %q, want foundry-test-key", got)
		}

		if got := r.Header.Get("anthropic-version"); got != "" {
			t.Fatalf("anthropic-version = %q, want empty for Foundry", got)
		}

		if got := r.Header.Get("x-api-key"); got != "" {
			t.Fatalf("x-api-key = %q, want empty for Foundry", got)
		}

		if got := r.Header.Get("accept"); got != "text/event-stream" {
			t.Fatalf("accept = %q, want text/event-stream", got)
		}

		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"delta\":{\"type\":\"text_delta\",\"text\":\"hello foundry\"}}\n\n"))
	}))
	defer server.Close()

	client := NewClient(Config{
		FoundryEnabled:  true,
		FoundryBaseURL:  server.URL,
		FoundryAPIKey:   "foundry-test-key",
		FoundrySkipAuth: false,
		BaseURL:         server.URL,
		HTTPClient:      server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-5",
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	evt := <-stream
	if evt.Type != model.EventTypeTextDelta || evt.Text != "hello foundry" {
		t.Fatalf("Stream() first event = %#v, want text delta hello foundry", evt)
	}
}

func TestClientFoundryStream_NoTaskBudgetForFoundry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("anthropic-beta"); got != "" {
			t.Fatalf("anthropic-beta = %q, want empty for Foundry", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		if _, ok := body["output_config"]; ok {
			t.Fatal("output_config should not be present for Foundry")
		}

		w.Header().Set("content-type", "text/event-stream")
	}))
	defer server.Close()

	client := NewClient(Config{
		FoundryEnabled:  true,
		FoundryBaseURL:  server.URL,
		FoundrySkipAuth: true,
		BaseURL:         server.URL,
		HTTPClient:      server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-test",
		TaskBudget: &model.TaskBudgetParam{
			Type:  "tokens",
			Total: 500_000,
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	for range stream {
	}
}

func TestClientFoundryStream_MapsModelID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Foundry uses model names directly, no mapping needed.
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if got := body["model"]; got != "claude-sonnet-4-5" {
			t.Fatalf("model = %v, want claude-sonnet-4-5", got)
		}
		w.Header().Set("content-type", "text/event-stream")
	}))
	defer server.Close()

	client := NewClient(Config{
		FoundryEnabled:  true,
		FoundryBaseURL:  server.URL,
		FoundrySkipAuth: true,
		BaseURL:         server.URL,
		HTTPClient:      server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-5",
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	for range stream {
	}
}
