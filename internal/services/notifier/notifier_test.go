package notifier

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

// mockHookRunner records hook calls for verification.
type mockHookRunner struct {
	calls []hookCall
}

type hookCall struct {
	ctx              context.Context
	message          string
	title            string
	notificationType string
	cwd              string
}

func (m *mockHookRunner) RunNotificationHooks(ctx context.Context, message, title, notificationType, cwd string) {
	m.calls = append(m.calls, hookCall{ctx, message, title, notificationType, cwd})
}

func TestSend_ChannelNone(t *testing.T) {
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf, ChannelGetter: func() string { return ChannelNone }})
	err := svc.Send(context.Background(), NotificationOptions{Message: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no terminal output for channel=none, got %q", buf.String())
	}
}

func TestSend_ChannelDisabled(t *testing.T) {
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf, ChannelGetter: func() string { return ChannelDisabled }})
	err := svc.Send(context.Background(), NotificationOptions{Message: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no terminal output for channel=disabled, got %q", buf.String())
	}
}

func TestSend_ChannelITerm2(t *testing.T) {
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf, ChannelGetter: func() string { return ChannelITerm2 }})
	err := svc.Send(context.Background(), NotificationOptions{Message: "msg", Title: "title"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "\n\ntitle:\nmsg") {
		t.Errorf("expected iTerm2 body in output, got %q", out)
	}
	if !strings.Contains(out, BEL) {
		t.Errorf("expected BEL terminator, got %q", out)
	}
}

func TestSend_ChannelITerm2WithBell(t *testing.T) {
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf, ChannelGetter: func() string { return ChannelITerm2WithBell }})
	err := svc.Send(context.Background(), NotificationOptions{Message: "m"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "\n\nm") {
		t.Errorf("expected iTerm2 body, got %q", out)
	}
	if !strings.Contains(out, BEL) {
		t.Errorf("expected BEL from iTerm2, got %q", out)
	}
	// Bell is emitted as a separate raw BEL (not wrapped in DCS).
	bellCount := strings.Count(out, BEL)
	if bellCount < 2 {
		t.Errorf("expected at least 2 BEL (one from iTerm2 seq, one raw bell), got %d", bellCount)
	}
}

func TestSend_ChannelKitty(t *testing.T) {
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf, ChannelGetter: func() string { return ChannelKitty }})
	err := svc.Send(context.Background(), NotificationOptions{Message: "kmsg", Title: "ktitle"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, OSCKitty) {
		t.Errorf("expected Kitty OSC 99, got %q", out)
	}
	if !strings.Contains(out, ST) {
		t.Errorf("expected ST terminator, got %q", out)
	}
	if !strings.Contains(out, "d=0:p=title") {
		t.Errorf("expected Kitty title param, got %q", out)
	}
}

func TestSend_ChannelGhostty(t *testing.T) {
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf, ChannelGetter: func() string { return ChannelGhostty }})
	err := svc.Send(context.Background(), NotificationOptions{Message: "gmsg", Title: "gtitle"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, OSCGhostty) {
		t.Errorf("expected Ghostty OSC 777, got %q", out)
	}
	if !strings.Contains(out, "gtitle") {
		t.Errorf("expected Ghostty title, got %q", out)
	}
}

func TestSend_ChannelTerminalBell(t *testing.T) {
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf, ChannelGetter: func() string { return ChannelTerminalBell }})
	err := svc.Send(context.Background(), NotificationOptions{Message: "bell"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != BEL {
		t.Errorf("expected raw BEL, got %q", buf.String())
	}
}

func TestSend_ChannelAuto_iTerm(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	t.Setenv("KITTY_WINDOW_ID", "")
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf, ChannelGetter: func() string { return ChannelAuto }})
	err := svc.Send(context.Background(), NotificationOptions{Message: "auto"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), OSCITerm2) {
		t.Errorf("expected iTerm2 output for auto+iTerm, got %q", buf.String())
	}
}

func TestSend_ChannelAuto_kitty(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("KITTY_WINDOW_ID", "42")
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf, ChannelGetter: func() string { return ChannelAuto }})
	err := svc.Send(context.Background(), NotificationOptions{Message: "auto"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), OSCKitty) {
		t.Errorf("expected Kitty output for auto+kitty, got %q", buf.String())
	}
}

func TestSend_ChannelAuto_AppleTerminal(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "Apple_Terminal")
	t.Setenv("KITTY_WINDOW_ID", "")
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf, ChannelGetter: func() string { return ChannelAuto }})
	err := svc.Send(context.Background(), NotificationOptions{Message: "auto"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for auto+Apple_Terminal (no_method_available), got %q", buf.String())
	}
}

func TestSend_ChannelAuto_unknown(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf, ChannelGetter: func() string { return ChannelAuto }})
	err := svc.Send(context.Background(), NotificationOptions{Message: "auto"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for auto+unknown, got %q", buf.String())
	}
}

func TestSend_HookRunnerCalled(t *testing.T) {
	m := &mockHookRunner{}
	svc := Init(InitOptions{
		HookRunner:    m,
		ChannelGetter: func() string { return ChannelNone },
		CWDGetter:     func() string { return "/tmp/project" },
	})
	ctx := context.Background()
	err := svc.Send(ctx, NotificationOptions{Message: "m", Title: "t", NotificationType: "test_type"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.calls) != 1 {
		t.Fatalf("expected 1 hook call, got %d", len(m.calls))
	}
	c := m.calls[0]
	if c.message != "m" {
		t.Errorf("expected message 'm', got %q", c.message)
	}
	if c.title != "t" {
		t.Errorf("expected title 't', got %q", c.title)
	}
	if c.notificationType != "test_type" {
		t.Errorf("expected type 'test_type', got %q", c.notificationType)
	}
	if c.cwd != "/tmp/project" {
		t.Errorf("expected cwd '/tmp/project', got %q", c.cwd)
	}
}

func TestSend_HookRunnerNil(t *testing.T) {
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf, ChannelGetter: func() string { return ChannelNone }})
	err := svc.Send(context.Background(), NotificationOptions{Message: "m"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSend_ChannelGetterNil(t *testing.T) {
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf})
	err := svc.Send(context.Background(), NotificationOptions{Message: "m"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output when channelGetter is nil (defaults to none), got %q", buf.String())
	}
}

func TestSend_ChannelGetterReturnsEmpty(t *testing.T) {
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf, ChannelGetter: func() string { return "" }})
	err := svc.Send(context.Background(), NotificationOptions{Message: "m"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty channel, got %q", buf.String())
	}
}

func TestSend_TmuxPassthrough(t *testing.T) {
	t.Setenv("TMUX", "1")
	defer os.Unsetenv("TMUX")
	buf := new(bytes.Buffer)
	svc := Init(InitOptions{Writer: buf, ChannelGetter: func() string { return ChannelITerm2 }})
	err := svc.Send(context.Background(), NotificationOptions{Message: "tmux"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, ESC+"Ptmux;") {
		t.Errorf("expected tmux DCS wrapper prefix, got %q", out)
	}
}

func TestDetectTerminal_Priority(t *testing.T) {
	// KITTY_WINDOW_ID wins over TERM_PROGRAM.
	t.Setenv("KITTY_WINDOW_ID", "1")
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	if got := DetectTerminal(); got != TerminalKitty {
		t.Errorf("expected Kitty (KITTY_WINDOW_ID wins), got %v", got)
	}
}

func TestDetectTerminal_iTerm(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	if got := DetectTerminal(); got != TerminalITerm {
		t.Errorf("expected iTerm, got %v", got)
	}
}

func TestDetectTerminal_ghostty(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM_PROGRAM", "ghostty")
	if got := DetectTerminal(); got != TerminalGhostty {
		t.Errorf("expected Ghostty, got %v", got)
	}
}

func TestDetectTerminal_AppleTerminal(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM_PROGRAM", "Apple_Terminal")
	if got := DetectTerminal(); got != TerminalAppleTerminal {
		t.Errorf("expected AppleTerminal, got %v", got)
	}
}

func TestDetectTerminal_unknown(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM_PROGRAM", "xterm")
	if got := DetectTerminal(); got != TerminalUnknown {
		t.Errorf("expected Unknown, got %v", got)
	}
}

func TestNotificationOptions_DefaultTitle(t *testing.T) {
	if DefaultTitle != "Claude Code" {
		t.Errorf("expected DefaultTitle 'Claude Code', got %q", DefaultTitle)
	}
}
