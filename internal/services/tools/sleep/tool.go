package sleep

import (
	"context"
	"fmt"
	"time"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registry identifier for the Sleep tool.
	Name = "Sleep"
	// DefaultDurationSeconds is the default sleep duration when none is provided.
	DefaultDurationSeconds = 1.0
	// MinDurationSeconds is the minimum allowed sleep duration.
	MinDurationSeconds = 0.1
	// MaxDurationSeconds is the maximum allowed sleep duration (1 hour).
	MaxDurationSeconds = 3600.0
)

// toolDescription merges the TS prompt.ts description and guidance into a single
// model-facing text. It covers the tool's purpose, concurrency safety, and the
// recommendation to prefer this over Bash(sleep).
const toolDescription = `Wait for a specified duration. The user can interrupt the sleep at any time.

Use this when the user tells you to sleep or rest, when you have nothing to do, or when you're waiting for something.

You can call this concurrently with other tools — it won't interfere with them.

Prefer this over Bash(sleep) — it doesn't hold a shell process.

Each wake-up costs an API call, but the prompt cache expires after 5 minutes of inactivity — balance accordingly.`

// Input is the typed request payload for the Sleep tool.
type Input struct {
	// Duration is the sleep duration in seconds.
	Duration float64 `json:"duration,omitempty"`
}

// Output is the structured result returned by the Sleep tool.
type Output struct {
	// Duration is the actual duration slept in seconds.
	Duration float64 `json:"duration"`
	// Interrupted reports whether the sleep was interrupted by context cancellation.
	Interrupted bool `json:"interrupted"`
}

// Tool implements the Sleep tool.
type Tool struct{}

// NewTool constructs a Sleep tool instance.
func NewTool() *Tool {
	return &Tool{}
}

// Name returns the stable registration name.
func (t *Tool) Name() string {
	return Name
}

// Description returns the tool summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return toolDescription
}

// InputSchema returns the Sleep input contract.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
}

// IsReadOnly reports that Sleep does not mutate external state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that independent invocations may run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// Invoke sleeps for the requested duration. It uses a select with the context
// done channel to support interruption. On normal completion, it returns the
// duration slept. On context cancellation, it returns an interrupted result.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("sleep tool: nil receiver")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	duration := DefaultDurationSeconds
	if input.Duration != 0 {
		duration = input.Duration
	}

	if duration < MinDurationSeconds {
		return coretool.Result{Error: fmt.Sprintf("duration must be at least %.1f seconds, got %.1f", MinDurationSeconds, duration)}, nil
	}
	if duration > MaxDurationSeconds {
		return coretool.Result{Error: fmt.Sprintf("duration must be at most %.0f seconds (1 hour), got %.1f", MaxDurationSeconds, duration)}, nil
	}

	logger.DebugCF("sleep", "sleeping", map[string]any{
		"duration_seconds": duration,
	})

	select {
	case <-ctx.Done():
		logger.DebugCF("sleep", "interrupted", nil)
		return coretool.Result{
			Output: fmt.Sprintf("Sleep interrupted after %.1f seconds.", duration),
			Meta: map[string]any{
				"data": Output{
					Duration:    duration,
					Interrupted: true,
				},
			},
		}, nil
	case <-time.After(time.Duration(duration * float64(time.Second))):
		logger.DebugCF("sleep", "completed", nil)
		return coretool.Result{
			Output: fmt.Sprintf("Slept for %.1f seconds.", duration),
			Meta: map[string]any{
				"data": Output{
					Duration:    duration,
					Interrupted: false,
				},
			},
		}, nil
	}
}

// inputSchema builds the declared input schema exposed to model providers.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"duration": {
				Type:        coretool.ValueKindNumber,
				Description: fmt.Sprintf("Sleep duration in seconds (default: %.1f, min: %.1f, max: %.0f). The sleep can be interrupted at any time.", DefaultDurationSeconds, MinDurationSeconds, MaxDurationSeconds),
				Required:    false,
			},
		},
	}
}
