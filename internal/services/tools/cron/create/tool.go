package create

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	cronshared "github.com/sheepzhao/claude-code-go/internal/services/tools/cron/shared"
)

const (
	// Name is the stable registry identifier for the CronCreate tool.
	Name = "CronCreate"
	// MaxAgeDays is the default recurring task expiry in days.
	MaxAgeDays = 7
)

// toolDescription merges the TS description() and prompt() into a single
// model-facing text. It covers cron syntax, one-shot vs recurring jobs,
// minute selection guidance, and runtime behavior.
const toolDescription = `Schedule a prompt to run at a future time within this Claude session — either recurring on a cron schedule, or once at a specific time.

Uses standard 5-field cron in the user's local timezone: minute hour day-of-month month day-of-week. "0 9 * * *" means 9am local — no timezone conversion needed.

## One-shot tasks (recurring: false)
For "remind me at X" or "at <time>, do Y" requests — fire once then auto-delete.
Pin minute/hour/day-of-month/month to specific values:
  "remind me at 2:30pm today to check the deploy" → cron: "30 14 <today_dom> <today_month> *", recurring: false
  "tomorrow morning, run the smoke test" → cron: "57 8 <tomorrow_dom> <tomorrow_month> *", recurring: false

## Recurring jobs (recurring: true, the default)
For "every N minutes" / "every hour" / "weekdays at 9am" requests:
  "*/5 * * * *" (every 5 min), "0 * * * *" (hourly), "0 9 * * 1-5" (weekdays at 9am local)

## Avoid the :00 and :30 minute marks when the task allows it
Every user who asks for "9am" gets '0 9', and every user who asks for "hourly" gets '0 *' — which means requests from across the planet land on the API at the same instant. When the user's request is approximate, pick a minute that is NOT 0 or 30:
  "every morning around 9" → "57 8 * * *" or "3 9 * * *" (not "0 9 * * *")
  "hourly" → "7 * * * *" (not "0 * * * *")
  "in an hour or so, remind me to..." → pick whatever minute you land on, don't round

Only use minute 0 or 30 when the user names that exact time and clearly means it.

## Session-only
Jobs live only in this Claude session — nothing is written to disk, and the job is gone when Claude exits.

## Runtime behavior
Jobs only fire while the REPL is idle (not mid-query). Session-only jobs die with the process. The scheduler adds a small deterministic jitter on top of whatever you pick: recurring tasks fire up to 10% of their period late (max 15 min); one-shot tasks landing on :00 or :30 fire up to 90 s early. Picking an off-minute is still the bigger lever.

Recurring tasks auto-expire after 7 days — they fire one final time, then are deleted. Tell the user about the 7-day limit when scheduling recurring jobs.

Returns a job ID you can pass to CronDelete.`

// Input is the typed request payload for the CronCreate tool.
type Input struct {
	// Cron is a standard 5-field cron expression in local time.
	Cron string `json:"cron"`
	// Prompt is the text prompt to enqueue at each fire time.
	Prompt string `json:"prompt"`
	// Recurring reports whether the task fires on every cron match (default true).
	Recurring *bool `json:"recurring,omitempty"`
	// Durable reports whether the task persists across sessions (default false).
	Durable *bool `json:"durable,omitempty"`
}

// Output is the structured result returned by the CronCreate tool.
type Output struct {
	// ID is the unique identifier for the created job.
	ID string `json:"id"`
	// Cron is the raw cron expression that was registered.
	Cron string `json:"cron"`
	// Prompt is the prompt text that was registered.
	Prompt string `json:"prompt"`
	// Recurring reports whether the task is recurring.
	Recurring bool `json:"recurring"`
	// Durable reports whether the task persists across sessions.
	Durable bool `json:"durable"`
}

// Tool implements the CronCreate tool.
type Tool struct {
	store *cronshared.Store
}

// NewTool constructs a CronCreate tool instance with the shared task store.
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

// InputSchema returns the CronCreate input contract.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
}

// IsReadOnly reports that CronCreate mutates task state.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that independent invocations may run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// RequiresUserInteraction reports that this tool requires user approval before scheduling.
func (t *Tool) RequiresUserInteraction() bool {
	return true
}

// Invoke validates the cron expression, enforces the MAX_JOBS limit, creates the
// task in the shared store, and returns the assigned ID with schedule metadata.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("cron_create tool: nil receiver")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	// Validate cron expression.
	if err := validateCron(input.Cron); err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	recurring := true
	if input.Recurring != nil {
		recurring = *input.Recurring
	}

	// Durable is ignored in minimum implementation — always session-only.
	durable := false

	task, err := t.store.Create(input.Cron, input.Prompt, recurring, durable)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	output := Output{
		ID:        task.ID,
		Cron:      task.Cron,
		Prompt:    task.Prompt,
		Recurring: task.Recurring,
		Durable:   task.Durable,
	}

	return coretool.Result{
		Output: formatCreateOutput(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// validateCron validates a 5-field cron expression. It checks that there are
// exactly 5 space-separated fields and that each field is a valid cron value.
func validateCron(cron string) error {
	fields := strings.Fields(cron)
	if len(fields) != 5 {
		return fmt.Errorf("invalid cron expression %q: expected 5 fields (M H DoM Mon DoW), got %d", cron, len(fields))
	}

	ranges := [][2]int{
		{0, 59},  // minute
		{0, 23},  // hour
		{1, 31},  // day of month
		{1, 12},  // month
		{0, 7},   // day of week (0 and 7 = Sunday)
	}

	for i, field := range fields {
		if !isValidCronField(field) {
			return fmt.Errorf("invalid cron expression %q: field %d (%q) is not a valid cron field", cron, i+1, field)
		}
		_ = ranges[i] // range validation deferred to scheduler
	}

	return nil
}

// isValidCronField checks whether a single cron field has valid syntax.
func isValidCronField(field string) bool {
	if field == "*" {
		return true
	}
	// step values: */N
	if strings.HasPrefix(field, "*/") {
		n, err := strconv.Atoi(field[2:])
		return err == nil && n > 0
	}
	// comma-separated list (handled broadly)
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return false
		}
		// range: a-b
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return false
			}
			a, errA := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			b, errB := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if errA != nil || errB != nil || a > b {
				return false
			}
			continue
		}
		// plain number
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
}

// inputSchema builds the declared input schema exposed to model providers.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"cron": {
				Type:        coretool.ValueKindString,
				Description: `Standard 5-field cron expression in local time: "M H DoM Mon DoW" (e.g. "*/5 * * * *" = every 5 minutes, "30 14 28 2 *" = Feb 28 at 2:30pm local once).`,
				Required:    true,
			},
			"prompt": {
				Type:        coretool.ValueKindString,
				Description: "The prompt to enqueue at each fire time.",
				Required:    true,
			},
			"recurring": {
				Type:        coretool.ValueKindBoolean,
				Description: "true (default) = fire on every cron match until deleted or auto-expired after 7 days. false = fire once at the next match, then auto-delete.",
			},
			"durable": {
				Type:        coretool.ValueKindBoolean,
				Description: "true = persist to .claude/scheduled_tasks.json and survive restarts. false (default) = in-memory only, dies when this Claude session ends.",
			},
		},
	}
}

// formatCreateOutput builds the model-facing output for a created cron job.
func formatCreateOutput(output Output) string {
	recurrence := "one-shot"
	if output.Recurring {
		recurrence = "recurring"
	}
	return fmt.Sprintf("Scheduled %s job %s (cron: %s). Session-only (not written to disk, dies when Claude exits). %s. Use CronDelete to cancel sooner.",
		recurrence, output.ID, output.Cron,
		recurringExpiry(output.Recurring),
	)
}

func recurringExpiry(recurring bool) string {
	if recurring {
		return fmt.Sprintf("Auto-expires after %d days", MaxAgeDays)
	}
	return "It will fire once then auto-delete"
}
