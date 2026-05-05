package agentsummary

import "strings"

// BuildSummaryPrompt constructs a 3-5 word present-tense summary prompt.
// Mirrors TS buildSummaryPrompt in agentSummary.ts.
// If previousSummary is non-empty, the prompt encourages the model to say
// something new rather than repeating the last summary.
func BuildSummaryPrompt(previousSummary string) string {
	var b strings.Builder

	b.WriteString(`Describe your most recent action in 3-5 words using present tense (-ing). Name the file or function, not the branch. Do not use tools.
`)

	if previousSummary != "" {
		b.WriteString("\n")
		b.WriteString("Previous: \"")
		b.WriteString(previousSummary)
		b.WriteString("\" — say something NEW.\n")
	}

	b.WriteString("\nGood: \"Reading runAgent.ts\"\n")
	b.WriteString("Good: \"Fixing null check in validate.ts\"\n")
	b.WriteString("Good: \"Running auth module tests\"\n")
	b.WriteString("Good: \"Adding retry logic to fetchUser\"\n")
	b.WriteString("\n")
	b.WriteString("Bad (past tense): \"Analyzed the branch diff\"\n")
	b.WriteString("Bad (too vague): \"Investigating the issue\"\n")
	b.WriteString("Bad (too long): \"Reviewing full branch diff and AgentTool.tsx integration\"\n")
	b.WriteString("Bad (branch name): \"Analyzed adam/background-summary branch diff\"")

	return b.String()
}
