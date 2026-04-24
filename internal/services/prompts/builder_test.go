package prompts

import (
	"context"
	"errors"
	"testing"
)

type staticSection struct {
	name     string
	content  string
	volatile bool
}

func (s staticSection) Name() string                       { return s.name }
func (s staticSection) Compute(ctx context.Context) (string, error) { return s.content, nil }
func (s staticSection) IsVolatile() bool                   { return s.volatile }

type countingSection struct {
	name      string
	callCount int
	volatile  bool
}

func (s *countingSection) Name() string { return s.name }
func (s *countingSection) Compute(ctx context.Context) (string, error) {
	s.callCount++
	return "content", nil
}
func (s *countingSection) IsVolatile() bool { return s.volatile }

type errorSection struct{ name string }

func (s errorSection) Name() string { return s.name }
func (s errorSection) Compute(ctx context.Context) (string, error) {
	return "", errors.New("compute failed")
}
func (s errorSection) IsVolatile() bool { return false }

func TestPromptBuilder_Build(t *testing.T) {
	ctx := context.Background()

	b := NewPromptBuilder(
		staticSection{name: "s1", content: "Section one"},
		staticSection{name: "s2", content: "Section two"},
	)

	result, err := b.Build(ctx)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	expected := "Section one\n\nSection two"
	if result != expected {
		t.Errorf("Build() = %q, want %q", result, expected)
	}
}

func TestPromptBuilder_Cache(t *testing.T) {
	ctx := context.Background()

	section := &countingSection{name: "cached", volatile: false}
	b := NewPromptBuilder(section)

	// First build should compute.
	_, err := b.Build(ctx)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if section.callCount != 1 {
		t.Errorf("First Build: callCount = %d, want 1", section.callCount)
	}

	// Second build should use cache.
	_, err = b.Build(ctx)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if section.callCount != 1 {
		t.Errorf("Second Build: callCount = %d, want 1 (cached)", section.callCount)
	}

	// After Clear, should recompute.
	b.Clear()
	_, err = b.Build(ctx)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if section.callCount != 2 {
		t.Errorf("After Clear: callCount = %d, want 2", section.callCount)
	}
}

func TestPromptBuilder_Volatile(t *testing.T) {
	ctx := context.Background()

	section := &countingSection{name: "volatile", volatile: true}
	b := NewPromptBuilder(section)

	// Volatile sections should recompute every time.
	_, _ = b.Build(ctx)
	_, _ = b.Build(ctx)

	if section.callCount != 2 {
		t.Errorf("Volatile section callCount = %d, want 2", section.callCount)
	}
}

func TestPromptBuilder_EmptySectionSkipped(t *testing.T) {
	ctx := context.Background()

	b := NewPromptBuilder(
		staticSection{name: "s1", content: "First"},
		staticSection{name: "empty", content: "   "},
		staticSection{name: "s2", content: "Second"},
	)

	result, err := b.Build(ctx)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	expected := "First\n\nSecond"
	if result != expected {
		t.Errorf("Build() = %q, want %q", result, expected)
	}
}

func TestPromptBuilder_ComputeError(t *testing.T) {
	ctx := context.Background()

	b := NewPromptBuilder(errorSection{name: "bad"})

	_, err := b.Build(ctx)
	if err == nil {
		t.Error("Expected error from failing section, got nil")
	}
}

func TestPromptBuilder_NilSectionSkipped(t *testing.T) {
	ctx := context.Background()

	b := NewPromptBuilder(
		staticSection{name: "s1", content: "First"},
		staticSection{name: "null", content: ""},
		staticSection{name: "s2", content: "Second"},
	)

	result, err := b.Build(ctx)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	expected := "First\n\nSecond"
	if result != expected {
		t.Errorf("Build() = %q, want %q", result, expected)
	}
}

func TestNewPromptBuilder_WithSections(t *testing.T) {
	ctx := context.Background()

	b := NewPromptBuilder(
		staticSection{name: "a", content: "A"},
		staticSection{name: "b", content: "B"},
	)

	result, err := b.Build(ctx)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	expected := "A\n\nB"
	if result != expected {
		t.Errorf("Build() = %q, want %q", result, expected)
	}
}
