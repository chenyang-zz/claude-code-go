package bundled

import (
	"os"
	"path/filepath"

	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

const minAgents = 5
const maxAgents = 30

const workerInstructions = `After you finish implementing the change:
1. **Simplify** — Invoke the Skill tool with ` + "`skill: \"simplify\"`" + ` to review and clean up your changes.
2. **Run unit tests** — Run the project's test suite (check for package.json scripts, Makefile targets, or common commands like ` + "`npm test`" + `, ` + "`bun test`" + `, ` + "`pytest`" + `, ` + "`go test`" + `). If tests fail, fix them.
3. **Test end-to-end** — Follow the e2e test recipe from the coordinator's prompt. If the recipe says to skip e2e for this unit, skip it.
4. **Commit and push** — Commit all changes with a clear message, push the branch, and create a PR with ` + "`gh pr create`" + `. Use a descriptive title.
5. **Report** — End with a single line: ` + "`PR: <url>`" + ` so the coordinator can track it.`

const batchNotAGitRepo = "This is not a git repository. The /batch command requires a git repo because it spawns agents in isolated git worktrees and creates PRs from each. Initialize a repo first, or run this from inside an existing one."

const batchMissingInstruction = `Provide an instruction describing the batch change you want to make.

Examples:
  /batch migrate from react to vue
  /batch replace all uses of lodash with native equivalents
  /batch add type annotations to all untyped function parameters`

func buildBatchPrompt(instruction string) string {
	return `# Batch: Parallel Work Orchestration

You are orchestrating a large, parallelizable change across this codebase.

## User Instruction

` + instruction + `

## Phase 1: Research and Plan (Plan Mode)

Call the EnterPlanMode tool now to enter plan mode, then:

1. **Understand the scope.** Launch one or more subagents to deeply research what this instruction touches.
2. **Decompose into independent units.** Break the work into ` + itoa(minAgents) + "–" + itoa(maxAgents) + ` self-contained units.
3. **Determine the e2e test recipe.** Figure out how a worker can verify its change actually works end-to-end.
4. **Write the plan.** Include summary, numbered list of work units, e2e test recipe, and worker instructions.

5. Call ExitPlanMode to present the plan for approval.

## Phase 2: Spawn Workers (After Plan Approval)

Once the plan is approved, spawn one background agent per work unit using the Agent tool. All agents must use isolation: "worktree" and run_in_background: true.

For each agent, the prompt must be fully self-contained. Include:
- The overall goal
- This unit's specific task
- Any codebase conventions discovered
- The e2e test recipe
- The worker instructions below, copied verbatim:

` + "```" + `
` + workerInstructions + `
` + "```" + `

Use subagent_type: "general-purpose" unless a more specific agent type fits.

## Phase 3: Track Progress

After launching all workers, render an initial status table:

| # | Unit | Status | PR |
|---|------|--------|----|

As background-agent completion notifications arrive, parse the PR: <url> line from each agent's result and re-render the table. When all agents have reported, render the final table and a one-line summary.`
}

// isInGitRepo walks up from the current working directory to find a .git
// directory or file (supporting git worktrees), returning true if found.
func isInGitRepo() bool {
	dir, err := os.Getwd()
	if err != nil {
		return false
	}
	for {
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func registerBatchSkill() {
	skill.RegisterBundledSkill(skill.BundledSkillDefinition{
		Name:                "batch",
		Description:         "Research and plan a large-scale change, then execute it in parallel across 5-30 isolated worktree agents that each open a PR.",
		WhenToUse:           "Use when the user wants to make a sweeping, mechanical change across many files (migrations, refactors, bulk renames) that can be decomposed into independent parallel units.",
		ArgumentHint:        "<instruction>",
		UserInvocable:        true,
		DisableModelInvocation: true,
		GetPromptForCommand: func(args string) (string, error) {
			instruction := args
			if instruction == "" {
				return batchMissingInstruction, nil
			}
		if !isInGitRepo() {
			return batchNotAGitRepo, nil
		}
			return buildBatchPrompt(instruction), nil
		},
	})
}
