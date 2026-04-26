package prompts

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/runtime/coordinator"
)

// CoordinatorSection renders the coordinator-mode guidance when coordinator mode is enabled.
type CoordinatorSection struct{}

// Name returns the section identifier.
func (s CoordinatorSection) Name() string { return "coordinator" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s CoordinatorSection) IsVolatile() bool { return true }

// Compute generates the coordinator section content when coordinator mode is enabled.
func (s CoordinatorSection) Compute(ctx context.Context) (string, error) {
	if !coordinator.IsCoordinatorMode() {
		return "", nil
	}

	data, _ := RuntimeContextFromContext(ctx)
	workerTools := coordinator.RenderWorkerToolsSummary(data.EnabledToolNames)
	return coordinator.GetCoordinatorSystemPrompt(workerTools), nil
}
