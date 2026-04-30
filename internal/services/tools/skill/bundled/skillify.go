package bundled

import "github.com/sheepzhao/claude-code-go/internal/services/tools/skill"

const skillifyPrompt = `# Skillify

You are capturing this session's repeatable process as a reusable skill.

## Your Task

### Step 1: Analyze the Session
Before asking any questions, analyze the session to identify:
- What repeatable process was performed
- What the inputs/parameters were
- The distinct steps (in order)
- The success artifacts/criteria for each step
- Where the user corrected or steered you
- What tools and permissions were needed
- What agents were used

### Step 2: Interview the User

Use AskUserQuestion for ALL questions. Never ask questions via plain text.

**Round 1: High level confirmation**
- Suggest a name and description for the skill based on your analysis
- Suggest high-level goal(s) and specific success criteria

**Round 2: More details**
- Present the high-level steps as a numbered list
- Suggest arguments if needed
- Ask if the skill should run inline or forked
- Ask where to save: project (.claude/skills/<name>/SKILL.md) or personal (~/.claude/skills/<name>/SKILL.md)

**Round 3: Breaking down each step**
For each major step, ask:
- What does this step produce that later steps need?
- What proves that this step succeeded?
- Should the user confirm before proceeding?
- Are any steps independent and could run in parallel?

**Round 4: Final questions**
- Confirm when this skill should be invoked, suggest trigger phrases
- Ask for any other gotchas or things to watch out for

Stop interviewing once you have enough information. Don't over-ask for simple processes!

### Step 3: Write the SKILL.md

Create the skill directory and file at the chosen location.

Use this format:
` + "```markdown" + `
---
name: skill-name
description: one-line description
allowed-tools:
  - tool patterns observed
when_to_use: detailed description including trigger phrases
argument-hint: "hint showing argument placeholders"
arguments:
  - arg names
---
# Skill Title

## Goal
Clearly stated goal with success criteria.

## Steps

### 1. Step Name
What to do. Be specific and actionable.

**Success criteria**: How to know this step is done.
` + "```" + `

**Per-step annotations:**
- **Success criteria** is REQUIRED on every step
- **Execution**: Direct (default), Task agent, Teammate, or [human]
- **Artifacts**: Data this step produces for later steps
- **Human checkpoint**: When to pause and ask before proceeding

### Step 4: Confirm and Save

Before writing, output the complete SKILL.md content for review. After writing, tell the user:
- Where the skill was saved
- How to invoke it: /skill-name [arguments]
- That they can edit the SKILL.md directly to refine it`

func registerSkillifySkill() {
	skill.RegisterBundledSkill(skill.BundledSkillDefinition{
		Name:                "skillify",
		Description:         "Capture this session's repeatable process into a skill. Call at end of the process you want to capture with an optional description.",
		AllowedTools:        []string{"Read", "Write", "Edit", "Glob", "Grep", "AskUserQuestion", "Bash(mkdir:*)"},
		UserInvocable:        true,
		DisableModelInvocation: true,
		ArgumentHint:        "[description of the process you want to capture]",
		GetPromptForCommand: func(args string) (string, error) {
			prompt := skillifyPrompt
			if args != "" {
				prompt += "\n\nThe user described this process as: \"" + args + "\""
			}
			return prompt, nil
		},
	})
}
