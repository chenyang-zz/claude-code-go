package bundled

import "github.com/sheepzhao/claude-code-go/internal/services/tools/skill"

const simplifyPrompt = `# Simplify: Code Review and Cleanup

Review all changed files for reuse, quality, and efficiency. Fix any issues found.

## Phase 1: Identify Changes

Run ` + "`git diff`" + ` (or ` + "`git diff HEAD`" + ` if there are staged changes) to see what changed. If there are no git changes, review the most recently modified files that the user mentioned or that you edited earlier in this conversation.

## Phase 2: Launch Three Review Agents in Parallel

Use the Agent tool to launch all three agents concurrently in a single message. Pass each agent the full diff so it has the complete context.

### Agent 1: Code Reuse Review

For each change:
1. Search for existing utilities and helpers that could replace newly written code. Look for similar patterns elsewhere in the codebase.
2. Flag any new function that duplicates existing functionality. Suggest the existing function to use instead.
3. Flag any inline logic that could use an existing utility.

### Agent 2: Code Quality Review

Review the same changes for hacky patterns:
1. Redundant state: state that duplicates existing state, cached values that could be derived
2. Parameter sprawl: adding new parameters instead of generalizing existing ones
3. Copy-paste with slight variation: near-duplicate code blocks
4. Leaky abstractions: exposing internal details that should be encapsulated
5. Stringly-typed code: using raw strings where constants/enums exist
6. Unnecessary comments: comments explaining WHAT (code already shows this), narrating the change, or referencing the task
7. Unnecessary JSX nesting where inner component props already provide the behavior

### Agent 3: Efficiency Review

Review the same changes for efficiency:
1. Unnecessary work: redundant computations, repeated file reads, duplicate network calls, N+1 patterns
2. Missed concurrency: independent operations run sequentially
3. Hot-path bloat: new blocking work on startup or per-request hot paths
4. Recurring no-op updates: state updates inside polling loops without change detection
5. Unnecessary existence checks: pre-checking file existence before operating (TOCTOU anti-pattern)
6. Memory: unbounded data structures, missing cleanup, event listener leaks
7. Overly broad operations: reading entire files when only portions are needed

## Phase 3: Fix Issues

Wait for all three agents to complete. Aggregate their findings and fix each issue directly. If a finding is a false positive, note it and move on.

When done, briefly summarize what was fixed (or confirm the code was already clean).`

func registerSimplifySkill() {
	skill.RegisterBundledSkill(skill.BundledSkillDefinition{
		Name:         "simplify",
		Description:  "Review changed code for reuse, quality, and efficiency, then fix any issues found.",
		UserInvocable: true,
		GetPromptForCommand: func(args string) (string, error) {
			prompt := simplifyPrompt
			if args != "" {
				prompt += "\n\n## Additional Focus\n\n" + args
			}
			return prompt, nil
		},
	})
}
