package prompts

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestTeamMemoryPromptSection_Name(t *testing.T) {
	s := TeamMemoryPromptSection{}
	if s.Name() != "auto_team_memory" {
		t.Errorf("Name() = %q, want %q", s.Name(), "auto_team_memory")
	}
}

func TestTeamMemoryPromptSection_IsVolatile(t *testing.T) {
	s := TeamMemoryPromptSection{}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
}

func TestTeamMemoryPromptSection_Compute_DisabledAutoMemory(t *testing.T) {
	// When auto memory is disabled the section returns empty.
	save := os.Getenv("CLAUDE_CODE_AUTO_MEMORY")
	defer os.Setenv("CLAUDE_CODE_AUTO_MEMORY", save)
	os.Setenv("CLAUDE_CODE_AUTO_MEMORY", "0")

	s := TeamMemoryPromptSection{}
	result, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string when auto memory is disabled, got %q", result)
	}
}

func TestTeamMemoryPromptSection_Compute_WithoutWorkingDir(t *testing.T) {
	// Simulate auto memory enabled but no working directory in context.
	save := os.Getenv("CLAUDE_CODE_AUTO_MEMORY")
	defer os.Setenv("CLAUDE_CODE_AUTO_MEMORY", save)
	os.Setenv("CLAUDE_CODE_AUTO_MEMORY", "1")

	s := TeamMemoryPromptSection{}
	result, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string when no working dir, got %q", result)
	}
}

func TestTeamMemoryPromptSection_Compute_IndividualMode(t *testing.T) {
	// Auto memory enabled, team memory sync disabled (env=0 / unset).
	saveAuto := os.Getenv("CLAUDE_CODE_AUTO_MEMORY")
	saveTeam := os.Getenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC")
	defer func() {
		os.Setenv("CLAUDE_CODE_AUTO_MEMORY", saveAuto)
		os.Setenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC", saveTeam)
	}()
	os.Setenv("CLAUDE_CODE_AUTO_MEMORY", "1")
	os.Setenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC", "0")

	tmpDir := t.TempDir()
	ctx := WithRuntimeContext(context.Background(), RuntimeContext{WorkingDir: tmpDir})

	s := TeamMemoryPromptSection{}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty prompt in individual mode")
	}

	// Individual mode: no team-related content.
	if strings.Contains(result, "team directory") || strings.Contains(result, "shared team") {
		t.Error("individual mode should not contain team directory references")
	}
	if strings.Contains(result, "Memory scope") {
		t.Error("individual mode should not have memory scope section")
	}
	if strings.Contains(result, "<scope>") {
		t.Error("individual mode should not have scope tags")
	}
	if strings.Contains(result, "saves private") || strings.Contains(result, "saves team") {
		t.Error("individual mode examples should use plain [saves ...] without scope prefix")
	}

	// Core sections that must be present in both modes.
	checks := []string{
		"# Memory",
		"Types of memory",
		"<types>",
		"<name>user</name>",
		"<name>feedback</name>",
		"<name>project</name>",
		"<name>reference</name>",
		"What NOT to save in memory",
		"How to save memories",
		"When to access memories",
		"Before recommending from memory",
		"Memory and other forms of persistence",
	}
	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("individual mode output missing %q", check)
		}
	}
}

func TestTeamMemoryPromptSection_Compute_CombinedMode(t *testing.T) {
	// Auto memory enabled + team memory sync enabled.
	saveAuto := os.Getenv("CLAUDE_CODE_AUTO_MEMORY")
	saveTeam := os.Getenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC")
	defer func() {
		os.Setenv("CLAUDE_CODE_AUTO_MEMORY", saveAuto)
		os.Setenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC", saveTeam)
	}()
	os.Setenv("CLAUDE_CODE_AUTO_MEMORY", "1")
	os.Setenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC", "1")

	tmpDir := t.TempDir()
	ctx := WithRuntimeContext(context.Background(), RuntimeContext{WorkingDir: tmpDir})

	s := TeamMemoryPromptSection{}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty prompt in combined mode")
	}

	// Combined mode: must have scope and team content.
	checks := []string{
		"# Memory",
		"shared team directory",
		"Memory scope",
		"always private",
		"default to private",
		"strongly bias toward team",
		"usually team",
		"<scope>",
		"Types of memory",
		"<types>",
		"<name>user</name>",
		"<name>feedback</name>",
		"<name>project</name>",
		"<name>reference</name>",
		"What NOT to save in memory",
		"MUST avoid saving sensitive data within shared team memories",
		"How to save memories",
		"When to access memories",
		"Before recommending from memory",
		"Memory and other forms of persistence",
	}
	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("combined mode output missing %q", check)
		}
	}

	// Examples should use scope prefixes.
	if !strings.Contains(result, "saves private user memory") {
		t.Error("combined mode should use [saves private ...] prefix in examples")
	}
	if !strings.Contains(result, "saves team feedback memory") {
		t.Error("combined mode should use [saves team ...] prefix in examples")
	}
}

func TestTeamMemoryPromptSection_Compute_TypeCompleteness(t *testing.T) {
	saveAuto := os.Getenv("CLAUDE_CODE_AUTO_MEMORY")
	saveTeam := os.Getenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC")
	defer func() {
		os.Setenv("CLAUDE_CODE_AUTO_MEMORY", saveAuto)
		os.Setenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC", saveTeam)
	}()
	os.Setenv("CLAUDE_CODE_AUTO_MEMORY", "1")
	os.Setenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC", "1")

	tmpDir := t.TempDir()
	ctx := WithRuntimeContext(context.Background(), RuntimeContext{WorkingDir: tmpDir})

	s := TeamMemoryPromptSection{}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all four types include description, when_to_save, how_to_use, examples.
	types := []string{"user", "feedback", "project", "reference"}
	for _, typ := range types {
		typeBlock := extractTypeBlock(result, typ)
		if typeBlock == "" {
			t.Errorf("missing type block for %q", typ)
			continue
		}
		for _, tag := range []string{"<description>", "<when_to_save>", "<how_to_use>", "<examples>"} {
			if !strings.Contains(typeBlock, tag) {
				t.Errorf("type %q block missing %s", typ, tag)
			}
		}
	}

	// The combined mode must have a scope for each type.
	for _, typ := range types {
		typeBlock := extractTypeBlock(result, typ)
		if !strings.Contains(typeBlock, "<scope>") {
			t.Errorf("combined mode: type %q missing <scope>", typ)
		}
	}
}

func TestTeamMemoryPromptSection_Compute_DriftCaveat(t *testing.T) {
	saveAuto := os.Getenv("CLAUDE_CODE_AUTO_MEMORY")
	defer os.Setenv("CLAUDE_CODE_AUTO_MEMORY", saveAuto)
	os.Setenv("CLAUDE_CODE_AUTO_MEMORY", "1")

	tmpDir := t.TempDir()
	ctx := WithRuntimeContext(context.Background(), RuntimeContext{WorkingDir: tmpDir})

	s := TeamMemoryPromptSection{}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	drift := "Memory records can become stale over time"
	if !strings.Contains(result, drift) {
		t.Error("output should include memory drift caveat")
	}

	// The verify-before-recommending guidance.
	verify := `"The memory says X exists" is not the same as "X exists now."`
	if !strings.Contains(result, verify) {
		t.Error("output should include the verify-before-recommending guidance")
	}
}

func TestTeamMemoryPromptSection_Compute_TwoStepSaveFlow(t *testing.T) {
	saveAuto := os.Getenv("CLAUDE_CODE_AUTO_MEMORY")
	saveTeam := os.Getenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC")
	defer func() {
		os.Setenv("CLAUDE_CODE_AUTO_MEMORY", saveAuto)
		os.Setenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC", saveTeam)
	}()
	os.Setenv("CLAUDE_CODE_AUTO_MEMORY", "1")
	os.Setenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC", "1")

	tmpDir := t.TempDir()
	ctx := WithRuntimeContext(context.Background(), RuntimeContext{WorkingDir: tmpDir})

	s := TeamMemoryPromptSection{}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Two-step save flow.
	if !strings.Contains(result, "Step 1") || !strings.Contains(result, "Step 2") {
		t.Error("combined mode should include two-step save flow")
	}
	if !strings.Contains(result, "MEMORY.md") {
		t.Error("should reference MEMORY.md entrypoint")
	}

	// Frontmatter template.
	if !strings.Contains(result, "frontmatter") {
		t.Error("should include frontmatter template")
	}
	if !strings.Contains(result, "type: {{user, feedback, project, reference}}") {
		t.Error("frontmatter should list all four types")
	}
}

func TestTeamMemoryPromptSection_Compute_PersistenceDifferentiation(t *testing.T) {
	saveAuto := os.Getenv("CLAUDE_CODE_AUTO_MEMORY")
	defer os.Setenv("CLAUDE_CODE_AUTO_MEMORY", saveAuto)
	os.Setenv("CLAUDE_CODE_AUTO_MEMORY", "1")

	tmpDir := t.TempDir()
	ctx := WithRuntimeContext(context.Background(), RuntimeContext{WorkingDir: tmpDir})

	s := TeamMemoryPromptSection{}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Memory vs plan vs tasks differentiation.
	if !strings.Contains(result, "plan instead of memory") {
		t.Error("should differentiate memory from plan")
	}
	if !strings.Contains(result, "tasks instead of memory") {
		t.Error("should differentiate memory from tasks")
	}
}

func TestTeamMemoryPromptSection_Compute_NoScopeWhenIndividual(t *testing.T) {
	saveAuto := os.Getenv("CLAUDE_CODE_AUTO_MEMORY")
	saveTeam := os.Getenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC")
	defer func() {
		os.Setenv("CLAUDE_CODE_AUTO_MEMORY", saveAuto)
		os.Setenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC", saveTeam)
	}()
	os.Setenv("CLAUDE_CODE_AUTO_MEMORY", "1")
	os.Setenv("CLAUDE_FEATURE_TEAM_MEMORY_SYNC", "0")

	tmpDir := t.TempDir()
	ctx := WithRuntimeContext(context.Background(), RuntimeContext{WorkingDir: tmpDir})

	s := TeamMemoryPromptSection{}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Individual mode must NOT have team-specific elements.
	for _, forbidden := range []string{
		"<scope>",
		"shared team directory",
		"team memory is shared",
		"MUST avoid saving sensitive data within shared team memories",
	} {
		if strings.Contains(result, forbidden) {
			t.Errorf("individual mode should not contain: %q", forbidden)
		}
	}
}

func TestTeamMemoryPromptSection_Compute_WarnSuppressionGate(t *testing.T) {
	saveAuto := os.Getenv("CLAUDE_CODE_AUTO_MEMORY")
	defer os.Setenv("CLAUDE_CODE_AUTO_MEMORY", saveAuto)
	os.Setenv("CLAUDE_CODE_AUTO_MEMORY", "1")

	tmpDir := t.TempDir()
	ctx := WithRuntimeContext(context.Background(), RuntimeContext{WorkingDir: tmpDir})

	s := TeamMemoryPromptSection{}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The explicit-save gate: asking to save a PR list should be challenged.
	if !strings.Contains(result, "ask what was *surprising* or *non-obvious*") {
		t.Error("should include explicit-save gate for activity-log noise")
	}

	// The ignore-instruction anti-pattern prevention.
	ignore := "proceed as if MEMORY.md were empty"
	if !strings.Contains(result, ignore) {
		t.Error("should instruct model to proceed as if MEMORY.md were empty when user says ignore")
	}
}

// extractTypeBlock returns the <type>...</type> block for the given type name.
func extractTypeBlock(output, typeName string) string {
	start := strings.Index(output, "<name>"+typeName+"</name>")
	if start == -1 {
		return ""
	}
	// Walk back to the enclosing <type>.
	blockStart := strings.LastIndex(output[:start], "<type>")
	end := strings.Index(output[start:], "</type>")
	if end == -1 {
		return ""
	}
	return output[blockStart : start+end+len("</type>")]
}
