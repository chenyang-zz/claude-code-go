package prompts

import "context"

// CronPromptSection provides usage guidance for the Cron scheduling tools.
type CronPromptSection struct{}

// Name returns the section identifier.
func (s CronPromptSection) Name() string { return "cron_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s CronPromptSection) IsVolatile() bool { return false }

// Compute generates the Cron tools usage guidance.
func (s CronPromptSection) Compute(ctx context.Context) (string, error) {
	return `# Cron Scheduling Tools

## CronCreate

Schedule a prompt to be enqueued at a future time. Use for both recurring schedules and one-shot reminders.

Uses standard 5-field cron in the user's local timezone: minute hour day-of-month month day-of-week. "0 9 * * *" means 9am local — no timezone conversion needed.

### One-shot tasks (recurring: false)

For "remind me at X" or "at <time>, do Y" requests — fire once then auto-delete.
Pin minute/hour/day-of-month/month to specific values:
  "remind me at 2:30pm today to check the deploy" -> cron: "30 14 <today_dom> <today_month> *", recurring: false
  "tomorrow morning, run the smoke test" -> cron: "57 8 <tomorrow_dom> <tomorrow_month> *", recurring: false

### Recurring jobs (recurring: true, the default)

For "every N minutes" / "every hour" / "weekdays at 9am" requests:
  "*/5 * * * *" (every 5 min), "0 * * * *" (hourly), "0 9 * * 1-5" (weekdays at 9am local)

### Avoid the :00 and :30 minute marks when the task allows it

Every user who asks for "9am" gets 0 9, and every user who asks for "hourly" gets 0 * — which means requests from across the planet land on the API at the same instant. When the user's request is approximate, pick a minute that is NOT 0 or 30:
  "every morning around 9" -> "57 8 * * *" or "3 9 * * *" (not "0 9 * * *")
  "hourly" -> "7 * * * *" (not "0 * * * *")
  "in an hour or so, remind me to..." -> pick whatever minute you land on, don't round

Only use minute 0 or 30 when the user names that exact time and clearly means it ("at 9:00 sharp", "at half past", coordinating with a meeting). When in doubt, nudge a few minutes early or late — the user will not notice, and the fleet will.

### Durability

By default (durable: false) the job lives only in this Claude session — nothing is written to disk, and the job is gone when Claude exits. Pass durable: true to write to .claude/scheduled_tasks.json so the job survives restarts. Only use durable: true when the user explicitly asks for the task to persist ("keep doing this every day", "set this up permanently"). Most "remind me in 5 minutes" / "check back in an hour" requests should stay session-only.

### Runtime behavior

Jobs only fire while the REPL is idle (not mid-query). Durable jobs persist to .claude/scheduled_tasks.json and survive session restarts — on next launch they resume automatically. One-shot durable tasks that were missed while the REPL was closed are surfaced for catch-up. Session-only jobs die with the process. The scheduler adds a small deterministic jitter on top of whatever you pick: recurring tasks fire up to 10% of their period late (max 15 min); one-shot tasks landing on :00 or :30 fire up to 90 s early. Picking an off-minute is still the bigger lever.

Recurring tasks auto-expire after 7 days — they fire one final time, then are deleted. This bounds session lifetime. Tell the user about the 7-day limit when scheduling recurring jobs.

Returns a job ID you can pass to CronDelete.

## CronDelete

Cancel a cron job previously scheduled with CronCreate. Removes it from .claude/scheduled_tasks.json (durable jobs) or the in-memory session store (session-only jobs).

## CronList

List all cron jobs scheduled via CronCreate, both durable (.claude/scheduled_tasks.json) and session-only.

## ScheduleWakeup

Schedule when to resume work in /loop dynamic mode — the user invoked /loop without an interval, asking you to self-pace iterations of a specific task.

Pass the same /loop prompt back via ` + "`" + `prompt` + "`" + ` each turn so the next firing repeats the task. For an autonomous /loop (no user prompt), pass the literal sentinel ` + "`" + `<<autonomous-loop-dynamic>>` + "`" + ` as ` + "`" + `prompt` + "`" + ` instead. Omit the call to end the loop.

### Picking delaySeconds

The prompt cache has a 5-minute TTL. Sleeping past 300 seconds means the next wake-up reads your full conversation context uncached — slower and more expensive. So the natural breakpoints:

- **Under 5 minutes (60s–270s)**: cache stays warm. Right for active work — checking a build, polling for state that's about to change, watching a process you just started.
- **5 minutes to 1 hour (300s–3600s)**: pay the cache miss. Right when there's no point checking sooner.

**Don't pick 300s.** It's the worst-of-both: you pay the cache miss without amortizing it. If you're tempted to "wait 5 minutes," either drop to 270s (stay in cache) or commit to 1200s+ (one cache miss buys a much longer wait).

For idle ticks with no specific signal to watch, default to **1200s–1800s** (20–30 min).

The runtime clamps to [60, 3600], so you don't need to clamp yourself.

### The reason field

One short sentence on what you chose and why. Be specific. "checking long bun build" beats "waiting."`, nil
}
