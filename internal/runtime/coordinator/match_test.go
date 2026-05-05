package coordinator

import (
	"os"
	"testing"
)

func TestMatchSessionMode_NoSessionMode(t *testing.T) {
	warning := MatchSessionMode(SessionModeUnset)
	if warning != "" {
		t.Fatalf("MatchSessionMode(unset) = %q, want empty", warning)
	}
}

func TestMatchSessionMode_MatchingCoordinator(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "1")
	warning := MatchSessionMode(SessionModeCoordinator)
	if warning != "" {
		t.Fatalf("MatchSessionMode(coordinator) = %q, want empty", warning)
	}
}

func TestMatchSessionMode_MatchingNormal(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "")
	warning := MatchSessionMode(SessionModeNormal)
	if warning != "" {
		t.Fatalf("MatchSessionMode(normal) = %q, want empty", warning)
	}
}

func TestMatchSessionMode_SwitchToCoordinator(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "")
	warning := MatchSessionMode(SessionModeCoordinator)
	if warning != "Entered coordinator mode to match resumed session." {
		t.Fatalf("MatchSessionMode(coordinator) = %q, want mode switch warning", warning)
	}
	if os.Getenv("CLAUDE_CODE_COORDINATOR_MODE") != "1" {
		t.Fatalf("env not set to 1 after switch")
	}
}

func TestMatchSessionMode_SwitchToNormal(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "1")
	warning := MatchSessionMode(SessionModeNormal)
	if warning != "Exited coordinator mode to match resumed session." {
		t.Fatalf("MatchSessionMode(normal) = %q, want mode switch warning", warning)
	}
	if os.Getenv("CLAUDE_CODE_COORDINATOR_MODE") != "" {
		t.Fatalf("env not cleared after switch")
	}
}

func TestParseSessionMode(t *testing.T) {
	tests := []struct {
		input string
		want  SessionMode
	}{
		{"coordinator", SessionModeCoordinator},
		{"normal", SessionModeNormal},
		{"", SessionModeUnset},
		{"  coordinator  ", SessionModeCoordinator},
		{"COORDINATOR", SessionModeCoordinator},
		{"NORMAL", SessionModeNormal},
		{"unknown", SessionModeUnset},
	}

	for _, tt := range tests {
		got := ParseSessionMode(tt.input)
		if got != tt.want {
			t.Errorf("ParseSessionMode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
