package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestInternalEvent_IsSubagentEvent(t *testing.T) {
	tests := []struct {
		name   string
		agentID string
		want   bool
	}{
		{"foreground event", "", false},
		{"subagent event", "agent-123", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := InternalEvent{AgentID: tt.agentID}
			if got := evt.IsSubagentEvent(); got != tt.want {
				t.Errorf("IsSubagentEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCCRClient_ReadInternalEvents_Success(t *testing.T) {
	page1 := listInternalEventsResponse{
		Data: []InternalEvent{
			{EventID: "evt-1", EventType: "test", IsCompaction: false, CreatedAt: "2026-01-01T00:00:00Z"},
			{EventID: "evt-2", EventType: "test", IsCompaction: true, CreatedAt: "2026-01-01T00:01:00Z"},
		},
		NextCursor: "cursor-2",
	}
	page2 := listInternalEventsResponse{
		Data: []InternalEvent{
			{EventID: "evt-3", EventType: "test", CreatedAt: "2026-01-01T00:02:00Z"},
		},
	}

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/worker/internal-events" {
			t.Errorf("expected path /worker/internal-events, got %s", r.URL.Path)
		}

		cursor := r.URL.Query().Get("cursor")
		if cursor == "" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(page1)
			return
		}
		if cursor == "cursor-2" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(page2)
			return
		}
		t.Errorf("unexpected cursor: %s", cursor)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "test-session")
	events, err := client.ReadInternalEvents(context.Background())
	if err != nil {
		t.Fatalf("ReadInternalEvents() error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].EventID != "evt-1" {
		t.Errorf("expected evt-1, got %s", events[0].EventID)
	}
	if events[1].EventID != "evt-2" {
		t.Errorf("expected evt-2, got %s", events[1].EventID)
	}
	if events[2].EventID != "evt-3" {
		t.Errorf("expected evt-3, got %s", events[2].EventID)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

func TestCCRClient_ReadSubagentInternalEvents_Params(t *testing.T) {
	var capturedParams url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedParams = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(listInternalEventsResponse{
			Data: []InternalEvent{
				{EventID: "evt-s1", EventType: "test", AgentID: "agent-1"},
			},
		})
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "test-session")
	events, err := client.ReadSubagentInternalEvents(context.Background())
	if err != nil {
		t.Fatalf("ReadSubagentInternalEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", events[0].AgentID)
	}
	if capturedParams == nil || capturedParams.Get("subagents") != "true" {
		t.Errorf("expected subagents=true param, got %v", capturedParams)
	}
}

func TestCCRClient_ReadInternalEvents_AuthHeader(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(listInternalEventsResponse{Data: []InternalEvent{}})
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "test-session", WithHeader("Authorization", "Bearer test-token"))
	_, err := client.ReadInternalEvents(context.Background())
	if err != nil {
		t.Fatalf("ReadInternalEvents() error = %v", err)
	}
	if receivedAuth != "Bearer test-token" {
		t.Errorf("expected Bearer test-token, got %s", receivedAuth)
	}
}

func TestCCRClient_ReadInternalEvents_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid token"}`))
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "test-session")
	_, err := client.ReadInternalEvents(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	se, ok := IsSendError(err)
	if !ok {
		t.Fatalf("expected SendError, got %T", err)
	}
	if se.Kind != SendErrorAuth {
		t.Errorf("expected auth error, got %s", se.Kind.String())
	}
}

func TestCCRClient_ReadInternalEvents_NilClient(t *testing.T) {
	var client *CCRClient
	_, err := client.ReadInternalEvents(context.Background())
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestCCRClient_ReadInternalEvents_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(listInternalEventsResponse{Data: []InternalEvent{}})
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "test-session")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.ReadInternalEvents(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestGroupInternalEventsByAgent(t *testing.T) {
	events := []InternalEvent{
		{EventID: "e1", AgentID: "agent-a"},
		{EventID: "e2", AgentID: "agent-b"},
		{EventID: "e3", AgentID: "agent-a"},
		{EventID: "e4"},
	}

	groups := GroupInternalEventsByAgent(events)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if len(groups["agent-a"]) != 2 {
		t.Errorf("expected 2 events for agent-a, got %d", len(groups["agent-a"]))
	}
	if len(groups["agent-b"]) != 1 {
		t.Errorf("expected 1 event for agent-b, got %d", len(groups["agent-b"]))
	}
	if len(groups[""]) != 1 {
		t.Errorf("expected 1 foreground event, got %d", len(groups[""]))
	}
}

func TestGroupInternalEventsByAgent_Empty(t *testing.T) {
	groups := GroupInternalEventsByAgent(nil)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}
