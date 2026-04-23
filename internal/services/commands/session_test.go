package commands

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// TestSessionCommandMetadata verifies /session keeps the source-compatible descriptor.
func TestSessionCommandMetadata(t *testing.T) {
	got := SessionCommand{}.Metadata()
	want := command.Metadata{
		Name:        "session",
		Description: "Show remote session URL and QR code",
		Usage:       "/session",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Metadata() = %#v, want %#v", got, want)
	}
}

// TestSessionCommandExecuteReportsRemoteFallback verifies the Go host exposes the stable non-remote fallback before remote mode exists.
func TestSessionCommandExecuteReportsRemoteFallback(t *testing.T) {
	result, err := SessionCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Not in remote mode. Start with `claude --remote` to use this command."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestSessionCommandExecuteRendersRemoteSession verifies `/session` surfaces the minimum remote URL and text QR output when remote mode is wired.
func TestSessionCommandExecuteRendersRemoteSession(t *testing.T) {
	result, err := SessionCommand{
		RemoteSession: coreconfig.RemoteSessionConfig{
			Enabled:   true,
			SessionID: "session_test123",
			URL:       "https://claude.ai/code/session_test123?m=0",
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(result.Output, "Remote session\n") {
		t.Fatalf("Execute() output = %q, want remote session heading", result.Output)
	}
	if !strings.Contains(result.Output, "Open in browser: https://claude.ai/code/session_test123?m=0") {
		t.Fatalf("Execute() output = %q, want remote session url", result.Output)
	}
	// Without a StateProvider, connection state section should not be present.
	if strings.Contains(result.Output, "Connection state:") {
		t.Fatalf("Execute() output unexpectedly contains connection state without provider")
	}
}

// stubRemoteStateProvider implements RemoteStateProvider for test assertions.
type stubRemoteStateProvider struct {
	activeCount       int
	closed            bool
	connectionState   string
	reconnectCount    int
	lastDisconnectErr error
}

func (s *stubRemoteStateProvider) ActiveSubscriptionCount() int { return s.activeCount }
func (s *stubRemoteStateProvider) IsClosed() bool               { return s.closed }
func (s *stubRemoteStateProvider) ConnectionState() string      { return s.connectionState }
func (s *stubRemoteStateProvider) ReconnectCount() int          { return s.reconnectCount }
func (s *stubRemoteStateProvider) LastDisconnectError() error   { return s.lastDisconnectErr }
func (s *stubRemoteStateProvider) LastDisconnectTime() time.Time { return time.Time{} }

// TestSessionCommandExecuteRendersRemoteSessionWithState verifies `/session` exposes subscription and lifecycle state when a provider is wired.
func TestSessionCommandExecuteRendersRemoteSessionWithState(t *testing.T) {
	result, err := SessionCommand{
		RemoteSession: coreconfig.RemoteSessionConfig{
			Enabled:   true,
			SessionID: "session_test123",
			URL:       "https://claude.ai/code/session_test123?m=0",
		},
		StateProvider: &stubRemoteStateProvider{activeCount: 2, closed: false},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(result.Output, "Connection state:") {
		t.Fatalf("Execute() output = %q, want connection state section", result.Output)
	}
	if !strings.Contains(result.Output, "Subscriptions active: 2") {
		t.Fatalf("Execute() output = %q, want active subscription count", result.Output)
	}
	if !strings.Contains(result.Output, "Lifecycle closed: no") {
		t.Fatalf("Execute() output = %q, want lifecycle closed=no", result.Output)
	}
}

// TestSessionCommandExecuteRendersRemoteSessionClosedState verifies `/session` reflects a closed lifecycle state.
func TestSessionCommandExecuteRendersRemoteSessionClosedState(t *testing.T) {
	result, err := SessionCommand{
		RemoteSession: coreconfig.RemoteSessionConfig{
			Enabled:   true,
			SessionID: "session_test123",
			URL:       "https://claude.ai/code/session_test123?m=0",
		},
		StateProvider: &stubRemoteStateProvider{activeCount: 0, closed: true},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(result.Output, "Subscriptions active: 0") {
		t.Fatalf("Execute() output = %q, want active subscription count 0", result.Output)
	}
	if !strings.Contains(result.Output, "Lifecycle closed: yes (closed)") {
		t.Fatalf("Execute() output = %q, want lifecycle closed=yes", result.Output)
	}
}

// TestSessionCommandExecuteRendersConnectionDetails verifies `/session` surfaces
// resilient stream connection details including status, reconnect count, and
// last disconnect error.
func TestSessionCommandExecuteRendersConnectionDetails(t *testing.T) {
	result, err := SessionCommand{
		RemoteSession: coreconfig.RemoteSessionConfig{
			Enabled:   true,
			SessionID: "session_test123",
			URL:       "https://claude.ai/code/session_test123?m=0",
		},
		StateProvider: &stubRemoteStateProvider{
			activeCount:       1,
			closed:            false,
			connectionState:   "disconnected",
			reconnectCount:    3,
			lastDisconnectErr: fmt.Errorf("read timeout"),
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(result.Output, "Status: disconnected") {
		t.Fatalf("Execute() output = %q, want status", result.Output)
	}
	if !strings.Contains(result.Output, "Reconnections: 3") {
		t.Fatalf("Execute() output = %q, want reconnect count", result.Output)
	}
	if !strings.Contains(result.Output, "Last disconnect: read timeout") {
		t.Fatalf("Execute() output = %q, want last disconnect error", result.Output)
	}
}

// stubRemoteSendStateProvider implements RemoteSendStateProvider for test assertions.
type stubRemoteSendStateProvider struct {
	sendCount    int64
	lastSendTime time.Time
}

func (s *stubRemoteSendStateProvider) SendCount() int64      { return s.sendCount }
func (s *stubRemoteSendStateProvider) LastSendTime() time.Time { return s.lastSendTime }

// TestSessionCommandExecuteRendersSendState verifies `/session` surfaces HTTP POST sender state.
func TestSessionCommandExecuteRendersSendState(t *testing.T) {
	result, err := SessionCommand{
		RemoteSession: coreconfig.RemoteSessionConfig{
			Enabled:   true,
			SessionID: "session_test123",
			URL:       "https://claude.ai/code/session_test123?m=0",
		},
		SendStateProvider: &stubRemoteSendStateProvider{sendCount: 5, lastSendTime: time.Now().Add(-30 * time.Second)},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(result.Output, "Write path:") {
		t.Fatalf("Execute() output = %q, want write path section", result.Output)
	}
	if !strings.Contains(result.Output, "HTTP POST sends: 5") {
		t.Fatalf("Execute() output = %q, want send count", result.Output)
	}
	if !strings.Contains(result.Output, "Last send:") {
		t.Fatalf("Execute() output = %q, want last send", result.Output)
	}
}

// TestSessionCommandExecuteRendersSendStateNever verifies `/session` shows "never" when no send occurred.
func TestSessionCommandExecuteRendersSendStateNever(t *testing.T) {
	result, err := SessionCommand{
		RemoteSession: coreconfig.RemoteSessionConfig{
			Enabled:   true,
			SessionID: "session_test123",
			URL:       "https://claude.ai/code/session_test123?m=0",
		},
		SendStateProvider: &stubRemoteSendStateProvider{sendCount: 0},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(result.Output, "Last send: never") {
		t.Fatalf("Execute() output = %q, want last send never", result.Output)
	}
}
