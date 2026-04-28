package list

import (
	"context"
	"fmt"
	"strings"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	cronshared "github.com/sheepzhao/claude-code-go/internal/services/tools/cron/shared"
)

const (
	// Name is the stable registry identifier for the CronList tool.
	Name = "CronList"
)

// toolDescription describes the CronList tool.
const toolDescription = `List all cron jobs scheduled via CronCreate in this session.`

// JobEntry represents one cron job in the list output.
type JobEntry struct {
	// ID is the job identifier.
	ID string `json:"id"`
	// Cron is the raw cron expression.
	Cron string `json:"cron"`
	// Prompt is the scheduled prompt text.
	Prompt string `json:"prompt"`
	// Recurring reports whether the job is recurring.
	Recurring bool `json:"recurring,omitempty"`
	// Durable reports whether the job persists across sessions.
	Durable bool `json:"durable,omitempty"`
}

// Input is the typed request payload for the CronList tool — it accepts no arguments.
type Input struct{}

// Output is the structured result returned by the CronList tool.
type Output struct {
	// Jobs lists the currently active cron tasks.
	Jobs []JobEntry `json:"jobs"`
}

// Tool implements the CronList tool.
type Tool struct {
	store *cronshared.Store
}

// NewTool constructs a CronList tool instance with the shared task store.
func NewTool(store *cronshared.Store) *Tool {
	return &Tool{store: store}
}

// Name returns the stable registration name.
func (t *Tool) Name() string {
	return Name
}

// Description returns the tool summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return toolDescription
}

// InputSchema returns an empty input contract — CronList accepts no arguments.
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{},
	}
}

// IsReadOnly reports that CronList never mutates external state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that independent invocations may run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// Invoke returns all currently active cron tasks from the shared store.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("cron_list tool: nil receiver")
	}

	// Decode empty input to validate the contract.
	_, err := coretool.DecodeInput[Input](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	tasks := t.store.List()
	jobs := make([]JobEntry, len(tasks))
	for i, task := range tasks {
		jobs[i] = JobEntry{
			ID:        task.ID,
			Cron:      task.Cron,
			Prompt:    task.Prompt,
			Recurring: task.Recurring,
			Durable:   task.Durable,
		}
	}

	output := Output{Jobs: jobs}

	return coretool.Result{
		Output: formatListOutput(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// formatListOutput builds a human-readable summary of the active cron jobs.
func formatListOutput(output Output) string {
	if len(output.Jobs) == 0 {
		return "No scheduled jobs."
	}

	var b strings.Builder
	for i, j := range output.Jobs {
		if i > 0 {
			b.WriteByte('\n')
		}
		recurrence := " (one-shot)"
		if j.Recurring {
			recurrence = " (recurring)"
		}
		durable := ""
		if !j.Durable {
			durable = " [session-only]"
		}
		fmt.Fprintf(&b, "%s — %s%s%s: %s", j.ID, j.Cron, recurrence, durable, truncate(j.Prompt, 80))
	}
	return b.String()
}

// truncate shortens a string to maxLen characters, appending "..." when truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
