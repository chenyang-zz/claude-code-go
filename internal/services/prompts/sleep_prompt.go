package prompts

import "context"

// SleepPromptSection provides usage guidance for the Sleep tool.
type SleepPromptSection struct{}

// Name returns the section identifier.
func (s SleepPromptSection) Name() string { return "sleep_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s SleepPromptSection) IsVolatile() bool { return false }

// Compute generates the Sleep tool usage guidance.
func (s SleepPromptSection) Compute(ctx context.Context) (string, error) {
	return `# Sleep

Wait for a specified duration. The user can interrupt the sleep at any time.

Use this when the user tells you to sleep or rest, when you have nothing to do, or when you're waiting for something.

You may receive <tick> prompts — these are periodic check-ins. Look for useful work to do before sleeping.

You can call this concurrently with other tools — it won't interfere with them.

Prefer this over Bash(sleep ...) — it doesn't hold a shell process.

Each wake-up costs an API call, but the prompt cache expires after 5 minutes of inactivity — balance accordingly.`, nil
}
