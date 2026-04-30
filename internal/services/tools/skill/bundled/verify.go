package bundled

import "github.com/sheepzhao/claude-code-go/internal/services/tools/skill"

func registerVerifySkill() {
	skill.RegisterBundledSkill(skill.BundledSkillDefinition{
		Name:         "verify",
		Description:  "Verify a code change does what it should by running the app and testing end-to-end.",
		UserInvocable: true,
		GetPromptForCommand: func(args string) (string, error) {
			prompt := `# Verify Skill

## Goal
Verify a code change does what it should by running the app and testing end-to-end.

## Steps

### 1. Understand the Change
Identify what files were changed and what the change is supposed to do. Run ` + "`git diff`" + ` (or ` + "`git diff HEAD`" + `) to see the full diff.

### 2. Run Unit Tests
Run the project's test suite first. Find the right test command for the project (check package.json scripts, Makefile targets, go test, etc.)

### 3. Build the Project
Build the project to ensure there are no compilation errors.

### 4. Test End-to-End
Start the application and verify the change works as expected. For:
- **CLI tools**: Run the app with relevant arguments
- **Web apps**: Start the dev server and test endpoints/routes
- **Libraries**: Write a quick test harness or use existing examples

### 5. Check for Side Effects
- Run the existing test suite again after e2e testing
- Check for any warnings or errors in logs
- Verify related functionality still works

## Success Criteria
- All unit tests pass
- The build succeeds
- The changed behavior works as expected end-to-end
- No regressions in related functionality

## Output
Report what you tested, how you tested it, and whether everything passed. If something failed, explain what went wrong and suggest fixes.
`
			if args != "" {
				prompt += "\n## User Request\n\n" + args
			}
			return prompt, nil
		},
	})
}
