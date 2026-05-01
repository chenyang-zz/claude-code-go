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

// mockGoogleAuthenticator is a test double that returns a fixed token or error.
type mockGoogleAuthenticator struct {
	token string
	err   error
}

func (m *mockGoogleAuthenticator) GetToken(_ context.Context) (string, error) {
	return m.token, m.err
}

func TestDefaultGoogleAuthenticator_EnvVar(t *testing.T) {
	os.Setenv("GOOGLE_ACCESS_TOKEN", "test-token-123")
	defer os.Unsetenv("GOOGLE_ACCESS_TOKEN")

	auth := &DefaultGoogleAuthenticator{}
	token, err := auth.GetToken(context.Background())
	if err != nil {
		t.Fatalf("GetToken() error = %v, want nil", err)
	}
	if token != "test-token-123" {
		t.Fatalf("GetToken() = %q, want test-token-123", token)
	}
}

func TestDefaultGoogleAuthenticator_NoToken(t *testing.T) {
	os.Unsetenv("GOOGLE_ACCESS_TOKEN")

	auth := &DefaultGoogleAuthenticator{}
	_, err := auth.GetToken(context.Background())
	if err == nil {
		t.Fatal("GetToken() error = nil, want non-nil")
	}
}

func TestNewGoogleAuthenticator_SkipAuth(t *testing.T) {
	auth := newGoogleAuthenticator(true)
	token, err := auth.GetToken(context.Background())
	if err != nil {
		t.Fatalf("GetToken() error = %v, want nil", err)
	}
	if token != "" {
		t.Fatalf("GetToken() = %q, want empty", token)
	}
}

func TestNewGoogleAuthenticator_Default(t *testing.T) {
	os.Setenv("GOOGLE_ACCESS_TOKEN", "default-token")
	defer os.Unsetenv("GOOGLE_ACCESS_TOKEN")

	auth := newGoogleAuthenticator(false)
	token, err := auth.GetToken(context.Background())
	if err != nil {
		t.Fatalf("GetToken() error = %v, want nil", err)
	}
	if token != "default-token" {
		t.Fatalf("GetToken() = %q, want default-token", token)
	}
}

func TestGetVertexProjectID_EnvVar(t *testing.T) {
	os.Setenv("GCLOUD_PROJECT", "project-from-gcloud")
	defer os.Unsetenv("GCLOUD_PROJECT")

	if got := getVertexProjectID(); got != "project-from-gcloud" {
		t.Fatalf("getVertexProjectID() = %q, want project-from-gcloud", got)
	}
}

func TestGetVertexProjectID_Fallback(t *testing.T) {
	os.Unsetenv("GCLOUD_PROJECT")
	os.Unsetenv("GOOGLE_CLOUD_PROJECT")
	os.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "project-from-env")
	defer os.Unsetenv("ANTHROPIC_VERTEX_PROJECT_ID")

	if got := getVertexProjectID(); got != "project-from-env" {
		t.Fatalf("getVertexProjectID() = %q, want project-from-env", got)
	}
}

func TestGetVertexProjectID_Empty(t *testing.T) {
	os.Unsetenv("GCLOUD_PROJECT")
	os.Unsetenv("GOOGLE_CLOUD_PROJECT")
	os.Unsetenv("ANTHROPIC_VERTEX_PROJECT_ID")

	if got := getVertexProjectID(); got != "" {
		t.Fatalf("getVertexProjectID() = %q, want empty", got)
	}
}

func TestClientVertexStream_MissingProjectID(t *testing.T) {
	os.Unsetenv("GCLOUD_PROJECT")
	os.Unsetenv("GOOGLE_CLOUD_PROJECT")
	os.Unsetenv("ANTHROPIC_VERTEX_PROJECT_ID")

	client := NewClient(Config{
		VertexEnabled:  true,
		VertexSkipAuth: true,
	})

	_, err := client.Stream(context.Background(), model.Request{
		Model: "claude-3-7-sonnet@20250219",
	})
	if err == nil {
		t.Fatal("Stream() error = nil, want error for missing project ID")
	}
}

func TestClientVertexStream_AuthFailure(t *testing.T) {
	client := NewClient(Config{
		VertexEnabled:   true,
		VertexProjectID: "test-project",
		VertexRegion:    "us-east5",
		VertexAuth:      &mockGoogleAuthenticator{err: errors.New("auth failed")},
	})

	_, err := client.Stream(context.Background(), model.Request{
		Model: "claude-3-7-sonnet@20250219",
	})
	if err == nil {
		t.Fatal("Stream() error = nil, want error for auth failure")
	}
}

func TestClientVertexStream_ResolvesRegionFromModel(t *testing.T) {
	os.Setenv("VERTEX_REGION_CLAUDE_3_7_SONNET", "us-central1")
	defer os.Unsetenv("VERTEX_REGION_CLAUDE_3_7_SONNET")

	got := resolveVertexRegion("claude-3-7-sonnet@20250219")
	if got != "us-central1" {
		t.Fatalf("resolveVertexRegion = %q, want us-central1", got)
	}
}

func TestClientVertexStream_NoTaskBudgetForVertex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("anthropic-beta"); got != "" {
			t.Fatalf("anthropic-beta = %q, want empty for Vertex", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		if _, ok := body["output_config"]; ok {
			t.Fatal("output_config should not be present for Vertex")
		}

		w.Header().Set("content-type", "text/event-stream")
	}))
	defer server.Close()

	client := NewClient(Config{
		VertexEnabled:   true,
		VertexProjectID: "test-project",
		VertexRegion:    "us-east5",
		VertexSkipAuth:  true,
		BaseURL:         server.URL,
		HTTPClient:      server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-3-7-sonnet@20250219",
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

func TestClientVertexStream_SendsToVertexEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantSuffix := "/v1/projects/test-project/locations/us-east5/publishers/anthropic/models/claude-3-7-sonnet@20250219:streamRawPredict"
		if !strings.HasSuffix(r.URL.Path, wantSuffix) {
			t.Fatalf("request path = %q, want suffix %q", r.URL.Path, wantSuffix)
		}

		if got := r.Header.Get("authorization"); got != "Bearer vertex-token" {
			t.Fatalf("authorization = %q, want Bearer vertex-token", got)
		}

		if got := r.Header.Get("anthropic-version"); got != "" {
			t.Fatalf("anthropic-version = %q, want empty for Vertex", got)
		}

		if got := r.Header.Get("x-api-key"); got != "" {
			t.Fatalf("x-api-key = %q, want empty for Vertex", got)
		}

		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"delta\":{\"type\":\"text_delta\",\"text\":\"hello vertex\"}}\n\n"))
	}))
	defer server.Close()

	client := NewClient(Config{
		VertexEnabled:   true,
		VertexProjectID: "test-project",
		VertexRegion:    "us-east5",
		VertexAuth:      &mockGoogleAuthenticator{token: "vertex-token"},
		BaseURL:         server.URL,
		HTTPClient:      server.Client(),
	})

	stream, err := client.Stream(context.Background(), model.Request{
		Model: "claude-3-7-sonnet@20250219",
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	evt := <-stream
	if evt.Type != model.EventTypeTextDelta || evt.Text != "hello vertex" {
		t.Fatalf("Stream() first event = %#v, want text delta hello vertex", evt)
	}
}
