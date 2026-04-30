package remote_trigger

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func TestName(t *testing.T) {
	rt := NewTool()
	if rt.Name() != Name {
		t.Fatalf("Name() = %q, want %q", rt.Name(), Name)
	}
}

func TestDescription(t *testing.T) {
	rt := NewTool()
	if rt.Description() == "" {
		t.Fatal("Description() returned empty string")
	}
}

func TestInputSchema(t *testing.T) {
	rt := NewTool()
	schema := rt.InputSchema()

	action, ok := schema.Properties["action"]
	if !ok {
		t.Fatal("InputSchema missing 'action' property")
	}
	if action.Type != tool.ValueKindString || !action.Required {
		t.Fatalf("action field: type=%s required=%v, want string required=true", action.Type, action.Required)
	}

	triggerID, ok := schema.Properties["trigger_id"]
	if !ok {
		t.Fatal("InputSchema missing 'trigger_id' property")
	}
	if triggerID.Type != tool.ValueKindString || triggerID.Required {
		t.Fatalf("trigger_id field: type=%s required=%v, want string required=false", triggerID.Type, triggerID.Required)
	}

	body, ok := schema.Properties["body"]
	if !ok {
		t.Fatal("InputSchema missing 'body' property")
	}
	if body.Type != tool.ValueKindObject || body.Required {
		t.Fatalf("body field: type=%s required=%v, want object required=false", body.Type, body.Required)
	}
}

func TestIsReadOnly(t *testing.T) {
	rt := NewTool()
	if rt.IsReadOnly() {
		t.Fatal("IsReadOnly() = true, want false (create/update/run mutate remote state)")
	}
}

func TestIsConcurrencySafe(t *testing.T) {
	rt := NewTool()
	if !rt.IsConcurrencySafe() {
		t.Fatal("IsConcurrencySafe() = false, want true")
	}
}

func TestInvoke_MissingAuthToken(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_OAUTH_TOKEN")
	os.Unsetenv("CLAUDE_CODE_ORGANIZATION_UUID")

	rt := NewTool()
	result, err := rt.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"action": "list"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "Not authenticated") {
		t.Fatalf("Invoke() error = %q, want 'Not authenticated'", result.Error)
	}
}

func TestInvoke_MissingOrgUUID(t *testing.T) {
	os.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "test-token")
	os.Unsetenv("CLAUDE_CODE_ORGANIZATION_UUID")

	rt := NewTool()
	result, err := rt.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"action": "list"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "organization UUID") {
		t.Fatalf("Invoke() error = %q, want 'organization UUID'", result.Error)
	}
}

func TestInvoke_InvalidAction(t *testing.T) {
	os.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "test-token")
	os.Setenv("CLAUDE_CODE_ORGANIZATION_UUID", "test-org")

	rt := NewTool()
	result, err := rt.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"action": "delete"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "invalid action") {
		t.Fatalf("Invoke() error = %q, want 'invalid action'", result.Error)
	}
}

func TestInvoke_GetMissingTriggerID(t *testing.T) {
	os.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "test-token")
	os.Setenv("CLAUDE_CODE_ORGANIZATION_UUID", "test-org")

	rt := NewTool()
	for _, action := range []string{"get", "update", "run"} {
		result, err := rt.Invoke(context.Background(), tool.Call{
			Input: map[string]any{"action": action},
		})
		if err != nil {
			t.Fatalf("Invoke(%s) error = %v", action, err)
		}
		if !strings.Contains(result.Error, "requires trigger_id") {
			t.Fatalf("Invoke(%s) error = %q, want 'requires trigger_id'", action, result.Error)
		}
	}
}

func TestInvoke_CreateMissingBody(t *testing.T) {
	os.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "test-token")
	os.Setenv("CLAUDE_CODE_ORGANIZATION_UUID", "test-org")

	rt := NewTool()
	for _, action := range []string{"create", "update"} {
		result, err := rt.Invoke(context.Background(), tool.Call{
			Input: map[string]any{"action": action, "trigger_id": "test-1"},
		})
		if err != nil {
			t.Fatalf("Invoke(%s) error = %v", action, err)
		}
		if !strings.Contains(result.Error, "requires body") {
			t.Fatalf("Invoke(%s) error = %q, want 'requires body'", action, result.Error)
		}
	}
}

func TestInvoke_InvalidTriggerIDFormat(t *testing.T) {
	os.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "test-token")
	os.Setenv("CLAUDE_CODE_ORGANIZATION_UUID", "test-org")

	rt := NewTool()
	result, err := rt.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"action": "get", "trigger_id": "has spaces"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "invalid trigger_id") {
		t.Fatalf("Invoke() error = %q, want 'invalid trigger_id'", result.Error)
	}
}

func TestInvoke_InvalidSchema(t *testing.T) {
	rt := NewTool()
	result, err := rt.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"action": "list", "unknown_field": "value"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "unexpected field") {
		t.Fatalf("Invoke() error = %q, want 'unexpected field'", result.Error)
	}
}

// setupMockServer starts an httptest server and sets the required env vars to point at it.
func setupMockServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	os.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "test-token")
	os.Setenv("CLAUDE_CODE_ORGANIZATION_UUID", "test-org")
	os.Setenv("CLAUDE_CODE_API_BASE_URL", server.URL)

	cleanup := func() {
		server.Close()
		os.Unsetenv("CLAUDE_CODE_OAUTH_TOKEN")
		os.Unsetenv("CLAUDE_CODE_ORGANIZATION_UUID")
		os.Unsetenv("CLAUDE_CODE_API_BASE_URL")
	}
	return server, cleanup
}

func TestInvoke_ListSuccess(t *testing.T) {
	_, cleanup := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/code/triggers" {
			t.Errorf("expected /v1/code/triggers, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer token")
		}
		if r.Header.Get("anthropic-beta") != "ccr-triggers-2026-01-30" {
			t.Errorf("expected ccr-triggers beta header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"triggers": []any{}})
	})
	defer cleanup()

	rt := NewTool()
	result, err := rt.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"action": "list"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error = %q, want empty", result.Error)
	}
	if !strings.HasPrefix(result.Output, "HTTP 200") {
		t.Fatalf("Invoke() output = %q, want HTTP 200 prefix", result.Output)
	}
}

func TestInvoke_GetSuccess(t *testing.T) {
	_, cleanup := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/code/triggers/my-trigger" {
			t.Errorf("expected /v1/code/triggers/my-trigger, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"trigger_id": "my-trigger"})
	})
	defer cleanup()

	rt := NewTool()
	result, err := rt.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"action": "get", "trigger_id": "my-trigger"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error = %q, want empty", result.Error)
	}
}

func TestInvoke_CreateSuccess(t *testing.T) {
	_, cleanup := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/code/triggers" {
			t.Errorf("expected /v1/code/triggers, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"trigger_id": "new-trigger"})
	})
	defer cleanup()

	rt := NewTool()
	result, err := rt.Invoke(context.Background(), tool.Call{
		Input: map[string]any{
			"action": "create",
			"body":   map[string]any{"name": "test", "schedule": "0 9 * * *"},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error = %q, want empty", result.Error)
	}
	if !strings.HasPrefix(result.Output, "HTTP 201") {
		t.Fatalf("Invoke() output = %q, want HTTP 201 prefix", result.Output)
	}
}

func TestInvoke_UpdateSuccess(t *testing.T) {
	_, cleanup := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/code/triggers/existing" {
			t.Errorf("expected /v1/code/triggers/existing, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"trigger_id": "existing", "updated": true})
	})
	defer cleanup()

	rt := NewTool()
	result, err := rt.Invoke(context.Background(), tool.Call{
		Input: map[string]any{
			"action":     "update",
			"trigger_id": "existing",
			"body":       map[string]any{"schedule": "0 12 * * *"},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error = %q, want empty", result.Error)
	}
}

func TestInvoke_RunSuccess(t *testing.T) {
	_, cleanup := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/code/triggers/run-me/run" {
			t.Errorf("expected /v1/code/triggers/run-me/run, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]any{"status": "triggered"})
	})
	defer cleanup()

	rt := NewTool()
	result, err := rt.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"action": "run", "trigger_id": "run-me"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error = %q, want empty", result.Error)
	}
}

func TestInvoke_HTTPErrorStatus(t *testing.T) {
	_, cleanup := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"error": "not found"})
	})
	defer cleanup()

	rt := NewTool()
	result, err := rt.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"action": "get", "trigger_id": "nonexistent"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.HasPrefix(result.Output, "HTTP 404") {
		t.Fatalf("Invoke() output = %q, want HTTP 404 prefix", result.Output)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error = %q, want empty (errors should be returned as HTTP status)", result.Error)
	}
}
