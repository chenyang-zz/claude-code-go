package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// mockAWSAuthenticator is a test double for AWS authentication.
type mockAWSAuthenticator struct {
	err error
}

func (m *mockAWSAuthenticator) SignRequest(req *http.Request, _ string, _ []byte) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

// mockBearerAWSAuthenticator is a test double that injects a Bearer token.
type mockBearerAWSAuthenticator struct{}

func (m *mockBearerAWSAuthenticator) SignRequest(req *http.Request, _ string, _ []byte) error {
	req.Header.Set("authorization", "Bearer bedrock-bearer-token")
	return nil
}

func TestDefaultAWSAuthenticator_MissingCredentials(t *testing.T) {
	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	auth := &DefaultAWSAuthenticator{}
	req, _ := http.NewRequest("POST", "https://example.com", nil)
	err := auth.SignRequest(req, "us-east-1", nil)
	if err == nil {
		t.Fatal("SignRequest() error = nil, want error for missing credentials")
	}
}

func TestDefaultAWSAuthenticator_SignsRequest(t *testing.T) {
	os.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	defer os.Unsetenv("AWS_ACCESS_KEY_ID")
	defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")
	os.Unsetenv("AWS_SESSION_TOKEN")

	auth := &DefaultAWSAuthenticator{}
	req, _ := http.NewRequest("POST", "https://bedrock-runtime.us-east-1.amazonaws.com/model/test/invoke-with-response-stream", nil)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/vnd.amazon.eventstream")

	err := auth.SignRequest(req, "us-east-1", []byte(`{"test":"body"}`))
	if err != nil {
		t.Fatalf("SignRequest() error = %v, want nil", err)
	}

	authz := req.Header.Get("authorization")
	if authz == "" {
		t.Fatal("authorization header is empty")
	}
	if !strings.HasPrefix(authz, "AWS4-HMAC-SHA256") {
		t.Fatalf("authorization = %q, want AWS4-HMAC-SHA256 prefix", authz)
	}
	if !strings.Contains(authz, "Credential=AKIAIOSFODNN7EXAMPLE/") {
		t.Fatalf("authorization missing expected credential")
	}

	amzDate := req.Header.Get("x-amz-date")
	if amzDate == "" {
		t.Fatal("x-amz-date header is empty")
	}

	if req.Header.Get("x-amz-security-token") != "" {
		t.Fatal("x-amz-security-token should not be set without session token")
	}
}

func TestDefaultAWSAuthenticator_WithSessionToken(t *testing.T) {
	os.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	os.Setenv("AWS_SESSION_TOKEN", "session-token-123")
	defer os.Unsetenv("AWS_ACCESS_KEY_ID")
	defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")
	defer os.Unsetenv("AWS_SESSION_TOKEN")

	auth := &DefaultAWSAuthenticator{}
	req, _ := http.NewRequest("POST", "https://example.com", nil)

	err := auth.SignRequest(req, "us-east-1", nil)
	if err != nil {
		t.Fatalf("SignRequest() error = %v, want nil", err)
	}

	if got := req.Header.Get("x-amz-security-token"); got != "session-token-123" {
		t.Fatalf("x-amz-security-token = %q, want session-token-123", got)
	}
}

func TestNoopAWSAuthenticator(t *testing.T) {
	auth := &noopAWSAuthenticator{}
	req, _ := http.NewRequest("POST", "https://example.com", nil)

	err := auth.SignRequest(req, "us-east-1", nil)
	if err != nil {
		t.Fatalf("SignRequest() error = %v, want nil", err)
	}

	if req.Header.Get("authorization") != "" {
		t.Fatal("authorization should not be set by noop authenticator")
	}
}

func TestBearerTokenAWSAuthenticator(t *testing.T) {
	auth := &bearerTokenAWSAuthenticator{token: "test-bearer-token"}
	req, _ := http.NewRequest("POST", "https://example.com", nil)

	err := auth.SignRequest(req, "us-east-1", nil)
	if err != nil {
		t.Fatalf("SignRequest() error = %v, want nil", err)
	}

	if got := req.Header.Get("authorization"); got != "Bearer test-bearer-token" {
		t.Fatalf("authorization = %q, want Bearer test-bearer-token", got)
	}
}

func TestNewAWSAuthenticator_Priority(t *testing.T) {
	// 1. Bearer token takes highest priority.
	os.Setenv("AWS_BEARER_TOKEN_BEDROCK", "bearer-123")
	defer os.Unsetenv("AWS_BEARER_TOKEN_BEDROCK")

	auth := newAWSAuthenticator(false)
	req, _ := http.NewRequest("POST", "https://example.com", nil)
	err := auth.SignRequest(req, "us-east-1", nil)
	if err != nil {
		t.Fatalf("SignRequest() error = %v", err)
	}
	if got := req.Header.Get("authorization"); got != "Bearer bearer-123" {
		t.Fatalf("bearer token auth: authorization = %q, want Bearer bearer-123", got)
	}

	// 2. Skip auth when explicitly requested (even with bearer token env var).
	auth = newAWSAuthenticator(true)
	req, _ = http.NewRequest("POST", "https://example.com", nil)
	err = auth.SignRequest(req, "us-east-1", nil)
	if err != nil {
		t.Fatalf("SignRequest() error = %v", err)
	}
	if req.Header.Get("authorization") != "" {
		t.Fatal("skip auth should not set authorization")
	}

	// 3. Default AWS credentials when no bearer token and not skipped.
	os.Unsetenv("AWS_BEARER_TOKEN_BEDROCK")
	os.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	defer os.Unsetenv("AWS_ACCESS_KEY_ID")
	defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	auth = newAWSAuthenticator(false)
	req, _ = http.NewRequest("POST", "https://example.com", nil)
	err = auth.SignRequest(req, "us-east-1", nil)
	if err != nil {
		t.Fatalf("SignRequest() error = %v", err)
	}
	if !strings.HasPrefix(req.Header.Get("authorization"), "AWS4-HMAC-SHA256") {
		t.Fatal("default auth should use AWS Signature V4")
	}
}

func TestClientBedrockStream_MissingModelID(t *testing.T) {
	client := NewClient(Config{
		BedrockEnabled:  true,
		BedrockSkipAuth: true,
	})

	_, err := client.Stream(context.Background(), model.Request{
		Model: "unknown-model-with-no-mapping",
	})
	if err == nil {
		t.Fatal("Stream() error = nil, want error for missing model ID")
	}
}

func TestClientBedrockStream_AuthFailure(t *testing.T) {
	client := NewClient(Config{
		BedrockEnabled: true,
		BedrockRegion:  "us-east-1",
		BedrockModelID: "us.anthropic.claude-test-v1:0",
		BedrockAuth:    &mockAWSAuthenticator{err: errors.New("auth failed")},
	})

	_, err := client.Stream(context.Background(), model.Request{
		Model: "claude-test",
	})
	if err == nil {
		t.Fatal("Stream() error = nil, want error for auth failure")
	}
}

func TestClientBedrockStream_SendsToBedrockEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantSuffix := "/model/us.anthropic.claude-sonnet-4-5-v1:0/invoke-with-response-stream"
		if !strings.HasSuffix(r.URL.Path, wantSuffix) {
			t.Fatalf("request path = %q, want suffix %q", r.URL.Path, wantSuffix)
		}

		if got := r.Header.Get("authorization"); got != "Bearer bedrock-bearer-token" {
			t.Fatalf("authorization = %q, want Bearer bedrock-bearer-token", got)
		}

		if got := r.Header.Get("anthropic-version"); got != "" {
			t.Fatalf("anthropic-version = %q, want empty for Bedrock", got)
		}

		if got := r.Header.Get("x-api-key"); got != "" {
			t.Fatalf("x-api-key = %q, want empty for Bedrock", got)
		}

		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"delta\":{\"type\":\"text_delta\",\"text\":\"hello bedrock\"}}\n\n"))
	}))
	defer server.Close()

	client := NewClient(Config{
		BedrockEnabled: true,
		BedrockRegion:  "us-east-1",
		BedrockModelID: "us.anthropic.claude-sonnet-4-5-v1:0",
		BedrockAuth:    &mockBearerAWSAuthenticator{},
		BaseURL:        server.URL,
		HTTPClient:     server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-5",
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	evt := <-stream
	if evt.Type != model.EventTypeTextDelta || evt.Text != "hello bedrock" {
		t.Fatalf("Stream() first event = %#v, want text delta hello bedrock", evt)
	}
}

func TestClientBedrockStream_NoTaskBudgetForBedrock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("anthropic-beta"); got != "" {
			t.Fatalf("anthropic-beta = %q, want empty for Bedrock", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		if _, ok := body["output_config"]; ok {
			t.Fatal("output_config should not be present for Bedrock")
		}

		w.Header().Set("content-type", "text/event-stream")
	}))
	defer server.Close()

	client := NewClient(Config{
		BedrockEnabled:  true,
		BedrockRegion:   "us-east-1",
		BedrockModelID:  "us.anthropic.claude-test-v1:0",
		BedrockSkipAuth: true,
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

func TestClientBedrockStream_MapsModelID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantSuffix := "/model/us.anthropic.claude-opus-4-6-v1/invoke-with-response-stream"
		if !strings.HasSuffix(r.URL.Path, wantSuffix) {
			t.Fatalf("request path = %q, want suffix %q", r.URL.Path, wantSuffix)
		}
		w.Header().Set("content-type", "text/event-stream")
	}))
	defer server.Close()

	client := NewClient(Config{
		BedrockEnabled:  true,
		BedrockRegion:   "us-east-1",
		BedrockSkipAuth: true,
		BaseURL:         server.URL,
		HTTPClient:      server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-opus-4-6",
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	for range stream {
	}
}
