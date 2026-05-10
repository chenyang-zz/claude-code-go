package sessionmemory

import (
	"strings"
	"testing"
)

// TestBuildSessionMemoryUpdatePrompt verifies that {{notesPath}} and
// {{currentNotes}} placeholders are replaced by their corresponding values.
func TestBuildSessionMemoryUpdatePrompt(t *testing.T) {
	notes := "# Section 1\nSome content here"
	path := "/path/to/memory-notes.md"

	result := BuildSessionMemoryUpdatePrompt(notes, path)

	if !strings.Contains(result, notes) {
		t.Errorf("BuildSessionMemoryUpdatePrompt() output should contain currentNotes value %q", notes)
	}
	if !strings.Contains(result, path) {
		t.Errorf("BuildSessionMemoryUpdatePrompt() output should contain notesPath value %q", path)
	}
}

// TestBuildSessionMemoryUpdatePrompt_EmptyNotes verifies the prompt is still
// generated correctly when currentNotes is an empty string.
func TestBuildSessionMemoryUpdatePrompt_EmptyNotes(t *testing.T) {
	path := "/path/to/notes.md"

	result := BuildSessionMemoryUpdatePrompt("", path)

	if !strings.Contains(result, "IMPORTANT") {
		t.Error("BuildSessionMemoryUpdatePrompt() with empty notes should still contain the template instruction content")
	}
	if !strings.Contains(result, path) {
		t.Errorf("BuildSessionMemoryUpdatePrompt() with empty notes should contain notesPath %q", path)
	}
}

// TestSubstituteVariables verifies that a single {{name}} placeholder is
// replaced with its corresponding value.
func TestSubstituteVariables(t *testing.T) {
	result := substituteVariables("hello {{name}}", map[string]string{"name": "world"})
	expected := "hello world"
	if result != expected {
		t.Errorf("substituteVariables() = %q, want %q", result, expected)
	}
}

// TestSubstituteVariables_Multiple verifies that multiple placeholders in a
// single template are all replaced correctly.
func TestSubstituteVariables_Multiple(t *testing.T) {
	result := substituteVariables("{{a}} and {{b}}", map[string]string{"a": "x", "b": "y"})
	expected := "x and y"
	if result != expected {
		t.Errorf("substituteVariables() = %q, want %q", result, expected)
	}
}

// TestSubstituteVariables_Unknown verifies that placeholders without a
// matching entry in the vars map are preserved unchanged in the output.
func TestSubstituteVariables_Unknown(t *testing.T) {
	result := substituteVariables("{{missing}}", map[string]string{})
	expected := "{{missing}}"
	if result != expected {
		t.Errorf("substituteVariables() = %q, want %q", result, expected)
	}
}

// TestSubstituteVariables_DollarSign verifies that values containing '$'
// characters are not misinterpreted as regex backreferences by the
// replacement engine.
func TestSubstituteVariables_DollarSign(t *testing.T) {
	result := substituteVariables("value: {{dollar}}", map[string]string{"dollar": "$1"})
	expected := "value: $1"
	if result != expected {
		t.Errorf("substituteVariables() with $ in value = %q, want %q", result, expected)
	}
}

// TestAnalyzeSectionSizes verifies that markdown sections prefixed with
// "# " are detected and assigned positive token estimates.
func TestAnalyzeSectionSizes(t *testing.T) {
	content := "# Section1\ncontent body\nmore content\n# Section2\ndata\n"

	sections := analyzeSectionSizes(content)

	if _, ok := sections["# Section1"]; !ok {
		t.Error("analyzeSectionSizes() should contain '# Section1'")
	}
	if _, ok := sections["# Section2"]; !ok {
		t.Error("analyzeSectionSizes() should contain '# Section2'")
	}
	if sections["# Section1"] <= 0 {
		t.Error("analyzeSectionSizes() token count for Section1 should be > 0")
	}
	if sections["# Section2"] <= 0 {
		t.Error("analyzeSectionSizes() token count for Section2 should be > 0")
	}
}

// TestGenerateSectionReminders_OverLimit verifies that sections exceeding
// MAX_SECTION_LENGTH trigger non-empty condensation reminders.
func TestGenerateSectionReminders_OverLimit(t *testing.T) {
	sectionSizes := map[string]int{
		"# Section1": 5000, // well over MAX_SECTION_LENGTH (2000)
	}

	reminders := generateSectionReminders(sectionSizes, 0)

	if reminders == "" {
		t.Error("generateSectionReminders() should return non-empty for over-limit sections")
	}
	if !strings.Contains(reminders, "Section1") {
		t.Error("generateSectionReminders() should mention the oversized section name")
	}
}

// TestGenerateSectionReminders_Normal verifies that sections within normal
// size limits and total tokens within MAX_TOTAL_SESSION_TOKENS produce an
// empty reminder string.
func TestGenerateSectionReminders_Normal(t *testing.T) {
	sectionSizes := map[string]int{
		"# Section1": 100,
		"# Section2": 200,
	}

	reminders := generateSectionReminders(sectionSizes, 100)

	if reminders != "" {
		t.Errorf("generateSectionReminders() should be empty for normal sections, got: %q", reminders)
	}
}

// TestTruncateSessionMemoryForCompact verifies that content with a section
// exceeding the per-section character limit is truncated appropriately and
// the wasTruncated flag is set to true.
func TestTruncateSessionMemoryForCompact(t *testing.T) {
	// Build a section with content that exceeds MAX_SECTION_LENGTH * 4 bytes.
	// MAX_SECTION_LENGTH = 2000, so maxCharsPerSection = 8000.
	longLine := strings.Repeat("a", 100)
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, longLine)
	}
	content := "# Section1\n" + strings.Join(lines, "\n")

	truncatedContent, wasTruncated := TruncateSessionMemoryForCompact(content)

	if !wasTruncated {
		t.Error("TruncateSessionMemoryForCompact() wasTruncated should be true for content exceeding the limit")
	}
	if len(truncatedContent) >= len(content) {
		t.Error("TruncateSessionMemoryForCompact() truncatedContent should be shorter than original content")
	}
}
