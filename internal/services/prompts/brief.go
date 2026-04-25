package prompts

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// BriefSection surfaces concise guidance when brief mode is enabled.
type BriefSection struct{}

// Name returns the section identifier.
func (s BriefSection) Name() string { return "brief" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s BriefSection) IsVolatile() bool { return false }

// Compute generates the brief-mode guidance when the feature is enabled.
func (s BriefSection) Compute(ctx context.Context) (string, error) {
	if !featureflag.IsEnabled("KAIROS") && !featureflag.IsEnabled("KAIROS_BRIEF") {
		return "", nil
	}

	return `# Brief mode

Keep responses concise and high-signal. Surface the important trade-offs or choices early, and avoid over-explaining when the user only needs the next useful step.`, nil
}
