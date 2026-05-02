package magicdocs_test

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/services/magicdocs"
)

func TestDetectMagicDocHeader_WithTitle(t *testing.T) {
	content := "# MAGIC DOC: My Project"
	info := magicdocs.DetectMagicDocHeader(content)
	if info == nil {
		t.Fatal("expected non-nil info for valid Magic Doc header")
	}
	if info.Title != "My Project" {
		t.Errorf("expected Title %q, got %q", "My Project", info.Title)
	}
	if info.Instructions != "" {
		t.Errorf("expected empty Instructions, got %q", info.Instructions)
	}
}

func TestDetectMagicDocHeader_WithTitleAndInstructions(t *testing.T) {
	content := "# MAGIC DOC: Architecture\n_Keep this updated_"
	info := magicdocs.DetectMagicDocHeader(content)
	if info == nil {
		t.Fatal("expected non-nil info for valid Magic Doc header")
	}
	if info.Title != "Architecture" {
		t.Errorf("expected Title %q, got %q", "Architecture", info.Title)
	}
	if info.Instructions != "Keep this updated" {
		t.Errorf("expected Instructions %q, got %q", "Keep this updated", info.Instructions)
	}
}

func TestDetectMagicDocHeader_WithAsteriskInstructions(t *testing.T) {
	content := "# MAGIC DOC: Guide\n*Follow these rules*"
	info := magicdocs.DetectMagicDocHeader(content)
	if info == nil {
		t.Fatal("expected non-nil info for valid Magic Doc header")
	}
	if info.Title != "Guide" {
		t.Errorf("expected Title %q, got %q", "Guide", info.Title)
	}
	if info.Instructions != "Follow these rules" {
		t.Errorf("expected Instructions %q, got %q", "Follow these rules", info.Instructions)
	}
}

func TestDetectMagicDocHeader_NoHeader(t *testing.T) {
	content := "Just a regular markdown file\nWith some content"
	info := magicdocs.DetectMagicDocHeader(content)
	if info != nil {
		t.Errorf("expected nil for content without Magic Doc header, got %+v", info)
	}
}

func TestDetectMagicDocHeader_HeaderNotFirstLine(t *testing.T) {
	content := "Some preamble\n# MAGIC DOC: Hidden"
	info := magicdocs.DetectMagicDocHeader(content)
	if info != nil {
		t.Errorf("expected nil for header not on first line, got %+v", info)
	}
}

func TestDetectMagicDocHeader_EmptyContent(t *testing.T) {
	content := ""
	info := magicdocs.DetectMagicDocHeader(content)
	if info != nil {
		t.Errorf("expected nil for empty content, got %+v", info)
	}
}

func TestDetectMagicDocHeader_WithBlankLineBeforeInstructions(t *testing.T) {
	content := "# MAGIC DOC: API\n\n_Updated regularly_"
	info := magicdocs.DetectMagicDocHeader(content)
	if info == nil {
		t.Fatal("expected non-nil info for valid Magic Doc header")
	}
	if info.Title != "API" {
		t.Errorf("expected Title %q, got %q", "API", info.Title)
	}
	if info.Instructions != "Updated regularly" {
		t.Errorf("expected Instructions %q, got %q", "Updated regularly", info.Instructions)
	}
}
