package autodream

import (
	"strings"
	"testing"
)

func TestBuildConsolidationPrompt_Basic(t *testing.T) {
	prompt := buildConsolidationPrompt("/tmp/memory", "/tmp/transcripts", "")

	if prompt == "" {
		t.Error("expected non-empty prompt")
	}

	// Verify all four phases are present.
	phases := []string{
		"Phase 1",
		"Phase 2",
		"Phase 3",
		"Phase 4",
	}
	for _, phase := range phases {
		if !strings.Contains(prompt, phase) {
			t.Errorf("expected prompt to contain %s", phase)
		}
	}

	// Verify memory root and transcript dir are injected.
	if !strings.Contains(prompt, "/tmp/memory") {
		t.Error("expected prompt to contain memory root")
	}
	if !strings.Contains(prompt, "/tmp/transcripts") {
		t.Error("expected prompt to contain transcript dir")
	}
}

func TestBuildConsolidationPrompt_WithExtra(t *testing.T) {
	extra := "\n\n**Tool constraints:** read-only bash only.\n\nSessions: 3"
	prompt := buildConsolidationPrompt("/tmp/memory", "/tmp/transcripts", extra)

	if !strings.Contains(prompt, extra) {
		t.Error("expected prompt to contain extra text")
	}
}

func TestBuildConsolidationPrompt_SessionListInjection(t *testing.T) {
	extra := "\n\nSessions since last consolidation (3):\n- session-1\n- session-2\n- session-3"
	prompt := buildConsolidationPrompt("/tmp/memory", "/tmp/transcripts", extra)

	if !strings.Contains(prompt, "session-1") {
		t.Error("expected prompt to contain session list")
	}
	if !strings.Contains(prompt, "session-2") {
		t.Error("expected prompt to contain session list")
	}
}
