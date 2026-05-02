package magicdocs_test

import (
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/services/magicdocs"
)

func TestBuildUpdatePrompt_Basic(t *testing.T) {
	docContents := "## Architecture\nThis is the architecture doc."
	docPath := "/path/to/doc.md"
	docTitle := "Architecture"
	instructions := ""

	result := magicdocs.BuildUpdatePrompt(docContents, docPath, docTitle, instructions)

	if !strings.Contains(result, docContents) {
		t.Error("expected result to contain docContents")
	}
	if !strings.Contains(result, docPath) {
		t.Error("expected result to contain docPath")
	}
	if !strings.Contains(result, docTitle) {
		t.Error("expected result to contain docTitle")
	}
	if strings.Contains(result, "DOCUMENT-SPECIFIC UPDATE INSTRUCTIONS") {
		t.Error("expected result to NOT contain custom instructions section when instructions is empty")
	}
}

func TestBuildUpdatePrompt_WithInstructions(t *testing.T) {
	docContents := "## Architecture"
	docPath := "/path/to/doc.md"
	docTitle := "Architecture"
	instructions := "Keep this updated"

	result := magicdocs.BuildUpdatePrompt(docContents, docPath, docTitle, instructions)

	if !strings.Contains(result, "Keep this updated") {
		t.Error("expected result to contain instructions text 'Keep this updated'")
	}
	if !strings.Contains(result, "DOCUMENT-SPECIFIC UPDATE INSTRUCTIONS") {
		t.Error("expected result to contain 'DOCUMENT-SPECIFIC UPDATE INSTRUCTIONS' section")
	}
}

func TestBuildUpdatePrompt_VariableSubstitution(t *testing.T) {
	docContents := "## My Doc\nContent here."
	docPath := "/my/doc.md"
	docTitle := "My Doc"
	instructions := ""

	result := magicdocs.BuildUpdatePrompt(docContents, docPath, docTitle, instructions)

	if strings.Contains(result, "{{docPath}}") {
		t.Error("expected {{docPath}} placeholder to be replaced")
	}
	if strings.Contains(result, "{{docContents}}") {
		t.Error("expected {{docContents}} placeholder to be replaced")
	}
	if strings.Contains(result, "{{docTitle}}") {
		t.Error("expected {{docTitle}} placeholder to be replaced")
	}
}

func TestBuildUpdatePrompt_EmptyContent(t *testing.T) {
	docContents := ""
	docPath := "/path/to/doc.md"
	docTitle := "Empty Doc"
	instructions := ""

	result := magicdocs.BuildUpdatePrompt(docContents, docPath, docTitle, instructions)

	if strings.Contains(result, "{{docContents}}") {
		t.Error("expected {{docContents}} placeholder to be replaced, even for empty content")
	}
}
