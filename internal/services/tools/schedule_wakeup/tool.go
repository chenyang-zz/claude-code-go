package schedule_wakeup

import (
	"context"
	"fmt"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/runtime/repl"
)

const (
	// Name is the stable registry identifier for the ScheduleWakeup tool.
	Name = "ScheduleWakeup"

	toolDescription = `Schedule when to resume work in /loop dynamic mode — the user invoked /loop without an interval, asking you to self-pace iterations of a specific task.

Pass the same /loop prompt back via ` + "`" + `prompt` + "`" + ` each turn so the next firing repeats the task. For an autonomous /loop (no user prompt), pass the literal sentinel ` + "`" + `<<autonomous-loop-dynamic>>` + "`" + ` as ` + "`" + `prompt` + "`" + ` instead. Omit the call to end the loop.

## Picking delaySeconds

The prompt cache has a 5-minute TTL. Sleeping past 300 seconds means the next wake-up reads your full conversation context uncached — slower and more expensive. So the natural breakpoints:

- Under 5 minutes (60s–270s): cache stays warm. Right for active work — checking a build, polling for state that's about to change, watching a process you just started.
- 5 minutes to 1 hour (300s–3600s): pay the cache miss. Right when there's no point checking sooner.

Don't pick 300s. If you're tempted to "wait 5 minutes," either drop to 270s (stay in cache) or commit to 1200s+ (one cache miss buys a much longer wait).

For idle ticks with no specific signal to watch, default to 1200s–1800s (20–30 min).

Think about what you're actually waiting for, not just "how long should I sleep." If you kicked off an 8-minute build, sleeping 60s burns the cache 8 times before it finishes — sleep ~270s twice instead.

The runtime clamps to [60, 3600], so you don't need to clamp yourself.

## The reason field

One short sentence on what you chose and why. Goes to telemetry and is shown back to the user. Be specific.`
)

// Input is the typed request payload for the ScheduleWakeup tool.
type Input struct {
	// DelaySeconds is the number of seconds from now to resume.
	DelaySeconds int `json:"delaySeconds"`
	// Reason is a one-sentence explanation of why this delay was chosen.
	Reason string `json:"reason"`
	// Prompt is the /loop input to fire on wake-up, passed verbatim each turn.
	Prompt string `json:"prompt"`
}

// Output is the structured result returned by the ScheduleWakeup tool.
type Output struct {
	DelaySeconds int    `json:"delaySeconds"`
	Reason       string `json:"reason"`
	FireAt       string `json:"fireAt,omitempty"`
}

// Tool implements the ScheduleWakeup tool for /loop dynamic mode self-pacing.
type Tool struct {
	scheduler *repl.WakeupScheduler
}

// NewTool constructs a ScheduleWakeup tool instance with the given wakeup
// scheduler.
func NewTool(scheduler *repl.WakeupScheduler) *Tool {
	return &Tool{scheduler: scheduler}
}

// Name returns the stable registration name.
func (t *Tool) Name() string {
	return Name
}

// Description returns the tool summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return toolDescription
}

// InputSchema returns the ScheduleWakeup input contract.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
}

// IsReadOnly reports that ScheduleWakeup schedules a future action.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that independent invocations may run in parallel
// safely (last-write-wins overwrite).
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// RequiresUserInteraction reports that ScheduleWakeup does not require user
// approval — the agent self-paces loop iterations autonomously.
func (t *Tool) RequiresUserInteraction() bool {
	return false
}

// Invoke validates the input, clamps delaySeconds, and schedules the wakeup.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil || t.scheduler == nil {
		return coretool.Result{}, fmt.Errorf("schedule_wakeup tool: nil receiver or scheduler")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	if input.Prompt == "" {
		return coretool.Result{Error: "prompt must not be empty"}, nil
	}
	if input.Reason == "" {
		return coretool.Result{Error: "reason must not be empty"}, nil
	}

	delaySeconds := repl.ClampWakeupDelay(input.DelaySeconds)

	t.scheduler.Schedule(delaySeconds, input.Reason, input.Prompt)

	pending := t.scheduler.Pending()
	fireAt := ""
	if pending != nil {
		fireAt = pending.FireAt.Format("15:04:05")
	}

	output := Output{
		DelaySeconds: delaySeconds,
		Reason:       input.Reason,
		FireAt:       fireAt,
	}

	return coretool.Result{
		Output: fmt.Sprintf("Scheduled wakeup in %ds (%s). Reason: %s",
			delaySeconds, fireAt, input.Reason),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"delaySeconds": {
				Type:        coretool.ValueKindInteger,
				Description: "Seconds from now to wake up. Clamped to [60, 3600] by the runtime.",
				Required:    true,
			},
			"reason": {
				Type:        coretool.ValueKindString,
				Description: "One short sentence explaining the chosen delay. Be specific, e.g. 'checking long bun build'.",
				Required:    true,
			},
			"prompt": {
				Type:        coretool.ValueKindString,
				Description: "The /loop input to fire on wake-up. Pass the same /loop input verbatim each turn. For autonomous /loop use the sentinel <<autonomous-loop-dynamic>>. Omit the ScheduleWakeup call to end the loop.",
				Required:    true,
			},
		},
	}
}
