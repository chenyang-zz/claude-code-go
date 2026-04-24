package prompts

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Section computes one piece of the system prompt.
type Section interface {
	// Name returns the unique identifier of this section.
	Name() string
	// Compute generates the section content. Returning an empty string means
	// this section should be skipped for the current turn.
	Compute(ctx context.Context) (string, error)
	// IsVolatile reports whether this section must be recomputed every turn.
	// Volatile sections break the prompt cache when their value changes.
	IsVolatile() bool
}

// PromptBuilder assembles the system prompt from registered sections.
type PromptBuilder struct {
	sections []Section
	cache    map[string]string
	mu       sync.RWMutex
}

// NewPromptBuilder creates a new builder with optional sections.
func NewPromptBuilder(sections ...Section) *PromptBuilder {
	b := &PromptBuilder{
		cache: make(map[string]string),
	}
	for _, s := range sections {
		b.Register(s)
	}
	return b
}

// Register adds a section to the builder.
func (b *PromptBuilder) Register(s Section) {
	b.sections = append(b.sections, s)
}

// Build assembles the system prompt from all registered sections.
// Non-volatile sections are cached until Clear() is called.
func (b *PromptBuilder) Build(ctx context.Context) (string, error) {
	var parts []string
	updates := make(map[string]string)

	for _, section := range b.sections {
		var content string

		// Try cache for non-volatile sections.
		if !section.IsVolatile() {
			b.mu.RLock()
			content = b.cache[section.Name()]
			b.mu.RUnlock()
		}

		// Compute if not cached.
		if content == "" {
			computed, err := section.Compute(ctx)
			if err != nil {
				return "", fmt.Errorf("section %q: %w", section.Name(), err)
			}
			content = computed
			if !section.IsVolatile() {
				updates[section.Name()] = content
			}
		}

		if strings.TrimSpace(content) != "" {
			parts = append(parts, content)
		}
	}

	// Write computed values to cache outside the read loop.
	if len(updates) > 0 {
		b.mu.Lock()
		for k, v := range updates {
			b.cache[k] = v
		}
		b.mu.Unlock()
	}

	return strings.Join(parts, "\n\n"), nil
}

// Clear clears the section cache so that the next Build() recomputes all
// non-volatile sections.
func (b *PromptBuilder) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cache = make(map[string]string)
}
