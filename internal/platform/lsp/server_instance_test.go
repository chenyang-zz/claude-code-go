package lsp

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestNewServerInstance_InitialState(t *testing.T) {
	config := ServerConfig{
		Command: "test-lsp",
		Args:    []string{"--stdio"},
	}
	si := NewServerInstance("test-server", config)

	if si.Name != "test-server" {
		t.Errorf("expected Name=%q, got %q", "test-server", si.Name)
	}
	if si.Config.Command != "test-lsp" {
		t.Errorf("expected Command=%q, got %q", "test-lsp", si.Config.Command)
	}
	if si.State() != ServerStateStopped {
		t.Errorf("expected State=Stopped, got %s", si.State())
	}
	if si.IsHealthy() {
		t.Error("expected IsHealthy=false for stopped server")
	}
	if si.IsInitialized() {
		t.Error("expected IsInitialized=false for stopped server")
	}
}

func TestServerInstance_StartTime_InitiallyZero(t *testing.T) {
	si := NewServerInstance("test", ServerConfig{Command: "test"})
	if !si.StartTime().IsZero() {
		t.Error("expected StartTime to be zero")
	}
}

func TestServerInstance_LastError_InitiallyNil(t *testing.T) {
	si := NewServerInstance("test", ServerConfig{Command: "test"})
	if si.LastError() != nil {
		t.Error("expected LastError to be nil")
	}
}

func TestServerInstance_RestartCount_InitiallyZero(t *testing.T) {
	si := NewServerInstance("test", ServerConfig{Command: "test"})
	if si.RestartCount() != 0 {
		t.Errorf("expected RestartCount=0, got %d", si.RestartCount())
	}
}

func TestServerInstance_Capabilities_InitiallyNil(t *testing.T) {
	si := NewServerInstance("test", ServerConfig{Command: "test"})
	if si.Capabilities() != nil {
		t.Error("expected Capabilities to be nil")
	}
}

func TestServerInstance_IsHealthy_FalseWhenStopped(t *testing.T) {
	si := NewServerInstance("test", ServerConfig{Command: "test"})
	if si.IsHealthy() {
		t.Error("expected IsHealthy=false for stopped server")
	}
}

func TestServerInstance_SendNotification_ErrorWhenNotHealthy(t *testing.T) {
	si := NewServerInstance("test", ServerConfig{Command: "test"})
	err := si.SendNotification("textDocument/didOpen", map[string]any{})
	if err == nil {
		t.Error("expected error when sending notification to stopped server")
	}
}

func TestServerInstance_OnNotification_RegistersHandler(t *testing.T) {
	si := NewServerInstance("test", ServerConfig{Command: "test"})
	// Should not panic — just verify handler registration works
	called := false
	handler := func(raw json.RawMessage) {
		called = true
	}
	si.OnNotification("test/notification", handler)
	// Handler is registered; we can't trigger it without a real server,
	// but registration should not panic.
	_ = called
}

func TestServerInstance_State_ReflectsInitialStopped(t *testing.T) {
	si := NewServerInstance("test", ServerConfig{Command: "test"})
	if si.State() != ServerStateStopped {
		t.Errorf("expected State=Stopped, got %s", si.State())
	}
	if si.State().String() != "stopped" {
		t.Errorf("expected State.String()=%q, got %q", "stopped", si.State().String())
	}
}

func TestServerInstance_StateString(t *testing.T) {
	tests := []struct {
		state    ServerState
		expected string
	}{
		{ServerStateStopped, "stopped"},
		{ServerStateStarting, "starting"},
		{ServerStateRunning, "running"},
		{ServerStateError, "error"},
		{ServerState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("ServerState(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestServerInstance_StartTime_ReturnsZeroTime(t *testing.T) {
	si := NewServerInstance("test", ServerConfig{Command: "test"})
	st := si.StartTime()
	if !st.IsZero() {
		t.Error("expected zero start time for never-started instance")
	}
}

func TestServerInstance_StartTime_AfterMockStart(t *testing.T) {
	si := NewServerInstance("test", ServerConfig{Command: "test"})
	// Manually simulate a start by manipulating internal state through known means.
	// Since Start() requires a real process, we verify the getter works with the zero value.
	// The actual start behavior is tested via integration tests.
	if si.StartTime().IsZero() != true {
		t.Error("start time should be zero for never-started instance")
	}
	// Verify the getter doesn't panic on concurrent access
	done := make(chan bool)
	go func() {
		_ = si.StartTime()
		done <- true
	}()
	<-done
}

func TestIsContentModifiedError(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{fmt.Errorf("some random error"), false},
		{fmt.Errorf("lsp: content modified (code=-32801)"), true},
		{fmt.Errorf("lsp: request failed (code=-32801)"), true},
		{fmt.Errorf("lsp: something (code=-32603)"), false},
		{fmt.Errorf("connection refused"), false},
	}

	for _, tt := range tests {
		if got := isContentModifiedError(tt.err); got != tt.expected {
			t.Errorf("isContentModifiedError(%v) = %v, want %v", tt.err, got, tt.expected)
		}
	}
}

func TestServerInstance_ConcurrentStateAccess(t *testing.T) {
	si := NewServerInstance("test", ServerConfig{Command: "test"})
	done := make(chan bool)

	// Concurrent reads should not panic.
	for i := 0; i < 10; i++ {
		go func() {
			_ = si.State()
			_ = si.StartTime()
			_ = si.LastError()
			_ = si.RestartCount()
			_ = si.IsHealthy()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
