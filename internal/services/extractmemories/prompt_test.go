package extractmemories

import (
	"strings"
	"testing"
)

func TestBuildExtractAutoOnlyPrompt(t *testing.T) {
	t.Run("basic prompt structure", func(t *testing.T) {
		prompt := BuildExtractAutoOnlyPrompt(5, "", false)

		// Check key sections are present.
		if !strings.Contains(prompt, "memory extraction subagent") {
			t.Error("expected role introduction")
		}
		if !strings.Contains(prompt, "~5 messages") {
			t.Error("expected message count")
		}
		if !strings.Contains(prompt, "Available tools:") {
			t.Error("expected available tools section")
		}
		if !strings.Contains(prompt, "Read, Grep, Glob, read-only Bash") {
			t.Error("expected tool list")
		}
		if !strings.Contains(prompt, "limited turn budget") {
			t.Error("expected turn budget warning")
		}
		if !strings.Contains(prompt, "## Types of memory") {
			t.Error("expected types section")
		}
		if !strings.Contains(prompt, "## What NOT to save in memory") {
			t.Error("expected what-not-to-save section")
		}
		if !strings.Contains(prompt, "## How to save memories") {
			t.Error("expected how-to-save section")
		}
		if !strings.Contains(prompt, "two-step process") {
			t.Error("expected two-step save process")
		}
		if !strings.Contains(prompt, "If the user explicitly asks you to remember something") {
			t.Error("expected explicit-remember instruction")
		}
	})

	t.Run("with existing memories", func(t *testing.T) {
		existing := "- [user] user_role.md (2026-05-01T12:00:00Z): Senior Go developer"
		prompt := BuildExtractAutoOnlyPrompt(10, existing, false)

		if !strings.Contains(prompt, "## Existing memory files") {
			t.Error("expected existing memories section")
		}
		if !strings.Contains(prompt, "Senior Go developer") {
			t.Error("expected existing memory description")
		}
		if !strings.Contains(prompt, "Check this list before writing") {
			t.Error("expected check-before-writing instruction")
		}
	})

	t.Run("skip index mode", func(t *testing.T) {
		prompt := BuildExtractAutoOnlyPrompt(5, "", true)

		if strings.Contains(prompt, "two-step process") {
			t.Error("should not contain two-step process when skipIndex=true")
		}
		if !strings.Contains(prompt, "## How to save memories") {
			t.Error("expected how-to-save section")
		}
		// SkipIndex mode should NOT mention MEMORY.md index.
		if strings.Contains(prompt, "MEMORY.md") {
			t.Error("should not mention MEMORY.md when skipIndex=true")
		}
	})

	t.Run("large message count", func(t *testing.T) {
		prompt := BuildExtractAutoOnlyPrompt(100, "", false)
		if !strings.Contains(prompt, "~100 messages") {
			t.Error("expected message count 100")
		}
	})
}

func TestBuildExtractCombinedPrompt(t *testing.T) {
	t.Run("delegates to auto-only when team memory disabled", func(t *testing.T) {
		combined := BuildExtractCombinedPrompt(5, "test existing", true, false)
		autoOnly := BuildExtractAutoOnlyPrompt(5, "test existing", true)

		if combined != autoOnly {
			t.Error("expected combined to equal auto-only when team memory disabled")
		}
	})

	t.Run("falls back to auto-only when team memory enabled (not yet implemented)", func(t *testing.T) {
		combined := BuildExtractCombinedPrompt(3, "", false, true)
		autoOnly := BuildExtractAutoOnlyPrompt(3, "", false)

		if combined != autoOnly {
			t.Error("expected combined to fall back to auto-only (TEAMMEM not implemented)")
		}
	})
}

func TestBuildOpener(t *testing.T) {
	t.Run("without existing memories", func(t *testing.T) {
		opener := buildOpener(5, "")

		if strings.Contains(opener, "## Existing memory files") {
			t.Error("should not contain existing memories section when empty")
		}
		if strings.Contains(opener, "Check this list before writing") {
			t.Error("should not contain check instruction when no existing memories")
		}
		if !strings.Contains(opener, "~5 messages") {
			t.Error("expected message count")
		}
	})

	t.Run("with existing memories", func(t *testing.T) {
		existing := "- [user] test.md: A test memory"
		opener := buildOpener(10, existing)

		if !strings.Contains(opener, "## Existing memory files") {
			t.Error("expected existing memories section")
		}
		if !strings.Contains(opener, "A test memory") {
			t.Error("expected memory content")
		}
	})
}

func TestMemoryTypeConstants(t *testing.T) {
	if !strings.Contains(TYPES_SECTION_INDIVIDUAL, "<name>user</name>") {
		t.Error("TYPES_SECTION_INDIVIDUAL should contain user type")
	}
	if !strings.Contains(TYPES_SECTION_INDIVIDUAL, "<name>feedback</name>") {
		t.Error("TYPES_SECTION_INDIVIDUAL should contain feedback type")
	}
	if !strings.Contains(TYPES_SECTION_INDIVIDUAL, "<name>project</name>") {
		t.Error("TYPES_SECTION_INDIVIDUAL should contain project type")
	}
	if !strings.Contains(TYPES_SECTION_INDIVIDUAL, "<name>reference</name>") {
		t.Error("TYPES_SECTION_INDIVIDUAL should contain reference type")
	}

	if !strings.Contains(WHAT_NOT_TO_SAVE_SECTION, "Code patterns, conventions, architecture") {
		t.Error("WHAT_NOT_TO_SAVE_SECTION should contain code patterns exclusion")
	}
	if !strings.Contains(WHAT_NOT_TO_SAVE_SECTION, "git log") {
		t.Error("WHAT_NOT_TO_SAVE_SECTION should mention git log")
	}

	if !strings.Contains(MEMORY_FRONTMATTER_EXAMPLE, "---") {
		t.Error("MEMORY_FRONTMATTER_EXAMPLE should contain YAML frontmatter")
	}
	if !strings.Contains(MEMORY_FRONTMATTER_EXAMPLE, "user, feedback, project, reference") {
		t.Error("MEMORY_FRONTMATTER_EXAMPLE should list memory types")
	}
}
