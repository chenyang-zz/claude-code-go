package analytics

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestDatadogSink_Emit_Success(t *testing.T) {
	var captured []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		if r.Header.Get("DD-API-KEY") != "test-key" {
			t.Errorf("expected DD-API-KEY header, got %s", r.Header.Get("DD-API-KEY"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}
		// Capture body for later inspection
		var err error
		captured, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sink := NewDatadogSink(server.URL, "test-key", "external", testLogger())
	event := Event{
		Name: EventToolUsed,
		Metadata: NewMetadata("sess-1").WithLabels(map[string]any{
			"model":    "claude-opus",
			"provider": "anthropic",
			"version":  "2.0.0",
		}),
		Payload: ToolUsedEvent{
			ToolName: "Bash",
			Duration: time.Second,
			Success:  true,
		},
	}

	err := sink.Emit(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify payload structure
	var payload map[string]any
	if err := json.Unmarshal(captured, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if payload["ddsource"] != "go" {
		t.Errorf("expected ddsource=go, got %v", payload["ddsource"])
	}
	if payload["message"] != EventToolUsed {
		t.Errorf("expected message=%s, got %v", EventToolUsed, payload["message"])
	}
	if payload["service"] != "claude-code" {
		t.Errorf("expected service=claude-code, got %v", payload["service"])
	}
	if payload["hostname"] != "claude-code" {
		t.Errorf("expected hostname=claude-code, got %v", payload["hostname"])
	}
	if payload["env"] != "external" {
		t.Errorf("expected env=external, got %v", payload["env"])
	}

	// Verify labels are merged as top-level keys
	if payload["model"] != "claude-opus" {
		t.Errorf("expected model=claude-opus, got %v", payload["model"])
	}
	if payload["provider"] != "anthropic" {
		t.Errorf("expected provider=anthropic, got %v", payload["provider"])
	}

	// Verify ddtags contains event name and tag fields
	ddtags, ok := payload["ddtags"].(string)
	if !ok {
		t.Fatal("ddtags should be a string")
	}
	if !strings.Contains(ddtags, "event:"+EventToolUsed) {
		t.Errorf("ddtags should contain event:%s", EventToolUsed)
	}
	if !strings.Contains(ddtags, "model:claude-opus") {
		t.Errorf("ddtags should contain model:claude-opus")
	}

	// Verify event-type-specific fields
	if payload["toolName"] != "Bash" {
		t.Errorf("expected toolName=Bash, got %v", payload["toolName"])
	}
	if payload["success"] != true {
		t.Errorf("expected success=true, got %v", payload["success"])
	}
}

func TestDatadogSink_Emit_Non2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	sink := NewDatadogSink(server.URL, "test-key", "external", testLogger())
	err := sink.Emit(context.Background(), Event{Name: "test.event", Metadata: NewMetadata("s-1")})
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected error to mention status 403, got %v", err)
	}
}

func TestDatadogSink_Emit_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	sink := NewDatadogSink(server.URL, "test-key", "external", testLogger())
	err := sink.Emit(context.Background(), Event{Name: "test.event", Metadata: NewMetadata("s-1")})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention status 500, got %v", err)
	}
}

func TestDatadogSink_Emit_NetworkError(t *testing.T) {
	// Use an unreachable address to simulate network error
	sink := NewDatadogSink("http://127.0.0.1:1", "test-key", "external", testLogger())
	sink.client.Timeout = 100 * time.Millisecond
	err := sink.Emit(context.Background(), Event{Name: "test.event", Metadata: NewMetadata("s-1")})
	if err == nil {
		t.Fatal("expected error for unreachable endpoint")
	}
}

func TestDatadogSink_buildPayload_ToolUsed(t *testing.T) {
	sink := NewDatadogSink("http://example.com", "key", "ant", testLogger())
	event := Event{
		Name:     EventToolUsed,
		Metadata: NewMetadata("s-1"),
		Payload: ToolUsedEvent{
			ToolName: "Read",
			Duration: 500 * time.Millisecond,
			Success:  false,
			ErrorMsg: "not found",
		},
	}
	payload := sink.buildPayload(event)

	if payload["message"] != EventToolUsed {
		t.Errorf("expected message=%s", EventToolUsed)
	}
	if payload["toolName"] != "Read" {
		t.Errorf("expected toolName=Read, got %v", payload["toolName"])
	}
	if payload["success"] != false {
		t.Errorf("expected success=false, got %v", payload["success"])
	}
	if payload["error"] != "not found" {
		t.Errorf("expected error=not found, got %v", payload["error"])
	}
}

func TestDatadogSink_buildPayload_ErrorEvent(t *testing.T) {
	sink := NewDatadogSink("http://example.com", "key", "ant", testLogger())
	event := Event{
		Name:     EventError,
		Metadata: NewMetadata("s-1"),
		Payload: ErrorEvent{
			Category:  "api",
			ErrorType: "timeout",
			ToolName:  "Bash",
		},
	}
	payload := sink.buildPayload(event)

	if payload["errorCategory"] != "api" {
		t.Errorf("expected errorCategory=api, got %v", payload["errorCategory"])
	}
	if payload["errorType"] != "timeout" {
		t.Errorf("expected errorType=timeout, got %v", payload["errorType"])
	}
	if payload["toolName"] != "Bash" {
		t.Errorf("expected toolName=Bash, got %v", payload["toolName"])
	}
}

func TestDatadogSink_buildPayload_CommandEvent(t *testing.T) {
	sink := NewDatadogSink("http://example.com", "key", "ant", testLogger())
	event := Event{
		Name:     EventCommand,
		Metadata: NewMetadata("s-1"),
		Payload: CommandEvent{
			CommandName: "/help",
			Success:     true,
		},
	}
	payload := sink.buildPayload(event)

	if payload["commandName"] != "/help" {
		t.Errorf("expected commandName=/help, got %v", payload["commandName"])
	}
	if payload["success"] != true {
		t.Errorf("expected success=true, got %v", payload["success"])
	}
}

func TestDatadogSink_buildPayload_SessionEvent(t *testing.T) {
	sink := NewDatadogSink("http://example.com", "key", "ant", testLogger())
	event := Event{
		Name:     EventSession,
		Metadata: NewMetadata("s-1"),
		Payload: SessionEvent{
			Action: "started",
		},
	}
	payload := sink.buildPayload(event)

	if payload["sessionAction"] != "started" {
		t.Errorf("expected sessionAction=started, got %v", payload["sessionAction"])
	}
}

func TestDatadogSink_buildPayload_DDTags(t *testing.T) {
	sink := NewDatadogSink("http://example.com", "key", "ant", testLogger())
	event := Event{
		Name: "custom.event",
		Metadata: NewMetadata("s-1").WithLabels(map[string]any{
			"model":    "claude-opus",    // tag field → in ddtags
			"provider": "anthropic",      // tag field → in ddtags
			"foo":      "bar",            // non-tag field → top-level only
		}),
	}
	payload := sink.buildPayload(event)

	ddtags, ok := payload["ddtags"].(string)
	if !ok {
		t.Fatal("ddtags should be a string")
	}

	if !strings.Contains(ddtags, "event:custom.event") {
		t.Errorf("ddtags should contain event:custom.event, got %s", ddtags)
	}
	if !strings.Contains(ddtags, "model:claude-opus") {
		t.Errorf("ddtags should contain model:claude-opus, got %s", ddtags)
	}
	if strings.Contains(ddtags, "foo:bar") {
		t.Errorf("ddtags should NOT contain foo:bar (non-tag field), got %s", ddtags)
	}

	// Non-tag fields should still be top-level keys
	if payload["foo"] != "bar" {
		t.Errorf("expected foo=bar at top level, got %v", payload["foo"])
	}
}

