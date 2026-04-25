package prompts

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// ProactiveSection surfaces autonomous-work guidance when proactive mode is enabled.
type ProactiveSection struct{}

// Name returns the section identifier.
func (s ProactiveSection) Name() string { return "proactive" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s ProactiveSection) IsVolatile() bool { return false }

// Compute generates the proactive-mode guidance when the feature is enabled.
func (s ProactiveSection) Compute(ctx context.Context) (string, error) {
	if !featureflag.IsEnabled("PROACTIVE") && !featureflag.IsEnabled("KAIROS") {
		return "", nil
	}

	return `# Autonomous work

You are running autonomously. Keep making useful progress with the tools you have, stay proactive about obvious next steps, and avoid idle status narration.`, nil
}
