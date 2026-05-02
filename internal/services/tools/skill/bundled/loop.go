package bundled

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

const defaultLoopInterval = "10m"
const defaultMaxAgeDays = 7

const loopUsage = `Usage: /loop [interval] <prompt>

Run a prompt or slash command on a recurring schedule, or as a self-paced dynamic loop.

Intervals: Ns, Nm, Nh, Nd (e.g. 5m, 30m, 2h, 1d). Minimum granularity is 1 minute.

Two modes:
1. Fixed interval: /loop 5m /foo — schedules with CronCreate at the given interval
2. Dynamic (no interval): /loop check the deploy — runs iteratively at your own pace using ScheduleWakeup

Examples:
  /loop 5m /babysit-prs        (every 5 min via CronCreate)
  /loop 30m check the deploy   (every 30 min via CronCreate)
  /loop 1h /standup 1          (every hour via CronCreate)
  /loop check the deploy       (dynamic self-paced via ScheduleWakeup)`

var leadingTokenRe = regexp.MustCompile(`^\d+[smhd]$`)

func buildLoopPrompt(args string) string {
	return fmt.Sprintf(`# /loop — schedule a recurring prompt

Parse the input below into [interval] <prompt…> and schedule it with CronCreate.

## Parsing (in priority order)

1. **Leading token**: if the first whitespace-delimited token matches `+"`^\\d+[smhd]$`"+` (e.g. 5m, 2h), that's the interval; the rest is the prompt.
2. **Trailing "every" clause**: otherwise, if the input ends with `+"`every <N><unit>`"+` or `+"`every <N> <unit-word>`"+`, extract that as the interval and strip it from the prompt.
3. **Default**: otherwise, interval is %s and the entire input is the prompt.

If the resulting prompt is empty, show usage and stop.

## Interval → cron

Supported suffixes: s (seconds, rounded up to nearest minute, min 1), m (minutes), h (hours), d (days).

| Interval pattern      | Cron expression     | Notes                                    |
|-----------------------|---------------------|------------------------------------------|
| Nm where N <= 59      | */N * * * *         | every N minutes                          |
| Nm where N >= 60      | 0 */H * * *         | round to hours (H = N/60)                |
| Nh where N <= 23      | 0 */N * * *         | every N hours                            |
| Nd                    | 0 0 */N * *         | every N days at midnight local           |
| Ns                    | treat as ceil(N/60)m| cron minimum granularity is 1 minute     |

## Action

1. Call CronCreate with:
   - cron: the expression from the table above
   - prompt: the parsed prompt, verbatim
   - recurring: true
2. Briefly confirm: what's scheduled, the cron expression, the human-readable cadence, that recurring tasks auto-expire after %d days, and that they can cancel sooner with CronDelete (include the job ID).
3. Then immediately execute the parsed prompt now — don't wait for the first cron fire.

## Input

%s`, defaultLoopInterval, defaultMaxAgeDays, args)
}

func buildLoopDynamicPrompt(args string) string {
	if args == "" {
		args = "<<autonomous-loop-dynamic>>"
	}
	return fmt.Sprintf(`# /loop — self-paced dynamic loop

Run the following task iteratively at your own pace. After each iteration, call ScheduleWakeup with the appropriate delay to continue the loop. Omit the ScheduleWakeup call to end the loop.

## How to pace

- If you're actively checking something (build progress, poll frequency): use 60s–270s to keep the prompt cache warm.
- If nothing needs checking soon: default to 1200s–1800s (20–30 min).
- Don't pick 300s (it pays the cache-miss cost without amortizing).

## Task

%s`, args)
}

func registerLoopSkill() {
	skill.RegisterBundledSkill(skill.BundledSkillDefinition{
		Name:          "loop",
		Description:   "Run a prompt or slash command on a recurring interval (e.g. /loop 5m /foo, defaults to 10m)",
		WhenToUse:     "When the user wants to set up a recurring task, poll for status, or run something repeatedly on an interval. Do NOT invoke for one-off tasks.",
		ArgumentHint:  "[interval] <prompt>",
		UserInvocable: true,
		GetPromptForCommand: func(args string) (string, error) {
			trimmed := strings.TrimSpace(args)
			if trimmed == "" {
				return buildLoopDynamicPrompt(""), nil
			}

			// Parse interval from first token
			parts := strings.Fields(trimmed)
			if len(parts) > 0 && leadingTokenRe.MatchString(parts[0]) {
				interval := parts[0]
				prompt := strings.TrimSpace(strings.TrimPrefix(trimmed, interval))
				if prompt == "" {
					return loopUsage, nil
				}
				_ = interval
				return buildLoopPrompt(trimmed), nil
			}

			// Check for trailing "every" clause — if found, use CronCreate path.
			lower := strings.ToLower(trimmed)
			if strings.Contains(lower, " every ") || strings.HasSuffix(lower, "every") {
				return buildLoopPrompt(trimmed), nil
			}

			// No interval → dynamic self-paced loop using ScheduleWakeup.
			return buildLoopDynamicPrompt(trimmed), nil
		},
	})
}
