package compact

import (
	"strings"
	"testing"
)

func TestGetCompactPrompt_Basic(t *testing.T) {
	prompt := GetCompactPrompt("")
	if !strings.Contains(prompt, "CRITICAL: Respond with TEXT ONLY") {
		t.Error("prompt missing no-tools preamble")
	}
	if !strings.Contains(prompt, "Primary Request and Intent") {
		t.Error("prompt missing Primary Request section")
	}
	if !strings.Contains(prompt, "REMINDER: Do NOT call any tools") {
		t.Error("prompt missing no-tools trailer")
	}
}

func TestGetCompactPrompt_CustomInstructions(t *testing.T) {
	prompt := GetCompactPrompt("Focus on Go code changes")
	if !strings.Contains(prompt, "Additional Instructions:") {
		t.Error("prompt missing additional instructions header")
	}
	if !strings.Contains(prompt, "Focus on Go code changes") {
		t.Error("prompt missing custom instructions content")
	}
}

func TestGetCompactPrompt_CustomInstructionsEmpty(t *testing.T) {
	prompt := GetCompactPrompt("   ")
	if strings.Contains(prompt, "Additional Instructions:") {
		t.Error("prompt should not include additional instructions for whitespace-only input")
	}
}

func TestGetCompactPrompt_NineSections(t *testing.T) {
	prompt := GetCompactPrompt("")
	sections := []string{
		"Primary Request and Intent",
		"Key Technical Concepts",
		"Files and Code Sections",
		"Errors and fixes",
		"Problem Solving",
		"All user messages",
		"Pending Tasks",
		"Current Work",
		"Optional Next Step",
	}
	for _, section := range sections {
		if !strings.Contains(prompt, section) {
			t.Errorf("prompt missing section: %s", section)
		}
	}
}

func TestFormatCompactSummary_WithAnalysisAndSummary(t *testing.T) {
	raw := `<analysis>
Let me think about this carefully.
The user wanted to fix a bug.
</analysis>

<summary>
1. Primary Request and Intent:
   Fix the login bug

2. Key Technical Concepts:
   - Authentication
</summary>`

	formatted := FormatCompactSummary(raw)
	if strings.Contains(formatted, "<analysis>") {
		t.Error("formatted output should not contain analysis tags")
	}
	if strings.Contains(formatted, "<summary>") {
		t.Error("formatted output should not contain summary tags")
	}
	if !strings.Contains(formatted, "Summary:") {
		t.Error("formatted output should contain 'Summary:' header")
	}
	if !strings.Contains(formatted, "Fix the login bug") {
		t.Error("formatted output should preserve summary content")
	}
}

func TestFormatCompactSummary_NoTags(t *testing.T) {
	raw := "Just plain text summary"
	formatted := FormatCompactSummary(raw)
	if formatted != raw {
		t.Errorf("expected unchanged text, got %q", formatted)
	}
}

func TestFormatCompactSummary_OnlySummaryTag(t *testing.T) {
	raw := `<summary>
Content here.
</summary>`
	formatted := FormatCompactSummary(raw)
	if strings.Contains(formatted, "<summary>") {
		t.Error("formatted output should not contain summary tags")
	}
	if !strings.Contains(formatted, "Summary:") {
		t.Error("formatted output should contain 'Summary:' header")
	}
}

func TestFormatCompactSummary_ExcessiveNewlines(t *testing.T) {
	raw := "Line 1\n\n\n\n\nLine 2"
	formatted := FormatCompactSummary(raw)
	if strings.Contains(formatted, "\n\n\n") {
		t.Error("formatted output should collapse multiple newlines to double")
	}
}
