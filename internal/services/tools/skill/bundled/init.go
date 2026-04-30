package bundled

// InitBundledSkills registers all bundled (built-in) skills at startup.
// Call this once during bootstrap after the skill package is initialized.
//
// Bundled skills are compiled into the binary and available to all users.
// Some skills are feature-gated via environment variables as noted in
// individual register functions.
//
// Skills that exist in TS but are skipped in Go (no recoverable source
// or require unavailable infrastructure):
//   - dream (KAIROS feature flag): no TS source available
//   - hunter (REVIEW_ARTIFACT feature flag): no TS source available
//   - runSkillGenerator (RUN_SKILL_GENERATOR feature flag): no TS source available
//   - claudeInChrome: requires @ant/claude-for-chrome-mcp npm package and BASE_CHROME_PROMPT
func InitBundledSkills() {
	// Tier 1: Self-contained prompt skills
	registerLoremIpsumSkill()
	registerSimplifySkill()
	registerStuckSkill()
	registerLoopSkill()
	registerBatchSkill()
	registerDebugSkill()
	registerRememberSkill()

	// Tier 2: Skills with inlined external dependencies
	registerUpdateConfigSkill()
	registerKeybindingsSkill()
	registerVerifySkill()

	// Tier 3: Skills with simplified/cropped external dependencies
	registerSkillifySkill()
	registerClaudeApiSkill()
	registerScheduleRemoteAgentsSkill()
}
