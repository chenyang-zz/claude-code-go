package builtin

import (
	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// GeneralPurposeAgentType is the stable identifier for the general-purpose built-in agent.
const GeneralPurposeAgentType = "general-purpose"

// GeneralPurposeAgentDefinition is the built-in general-purpose agent configuration.
var GeneralPurposeAgentDefinition = agent.BuiltInAgentDefinition{
	Definition: agent.Definition{
		AgentType: GeneralPurposeAgentType,
		Source:    "built-in",
		BaseDir:   "built-in",
		WhenToUse: generalPurposeWhenToUse,
		Tools:     []string{"*"},
		Model:     "",
	},
	SystemPromptProvider: GeneralPurposeSystemPromptProvider{},
}

// GeneralPurposeSystemPromptProvider generates the system prompt for the general-purpose agent.
type GeneralPurposeSystemPromptProvider struct{}

// GetSystemPrompt returns the general-purpose agent's system prompt.
func (GeneralPurposeSystemPromptProvider) GetSystemPrompt(toolCtx tool.UseContext) string {
	return generalPurposeSystemPrompt
}

// generalPurposeWhenToUse describes when the general-purpose agent should be invoked.
const generalPurposeWhenToUse = `General-purpose agent for researching complex questions, searching for code, and executing multi-step tasks. When you are searching for a keyword or file and are not confident that you will find the right match in the first few tries use this agent to perform the search for you.`

// generalPurposeSystemPrompt is the complete system prompt for the general-purpose agent.
const generalPurposeSystemPrompt = `You are an agent for Claude Code, Anthropic's official CLI for Claude. Given the user's message, you should use the tools available to complete the task. Complete the task fully—don't gold-plate, but don't leave it half-done. When you complete the task, respond with a concise report covering what was done and any key findings — the caller will relay this to the user, so it only needs the essentials.

Your strengths:
- Searching for code, configurations, and patterns across large codebases
- Analyzing multiple files to understand system architecture
- Investigating complex questions that require exploring many files
- Performing multi-step research tasks

Guidelines:
- For file searches: search broadly when you don't know where something lives. Use Read when you know the specific file path.
- For analysis: Start broad and narrow down. Use multiple search strategies if the first doesn't yield results.
- Be thorough: Check multiple locations, consider different naming conventions, look for related files.
- NEVER create files unless they're absolutely necessary for achieving your goal. ALWAYS prefer editing an existing file to creating a new one.
- NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested.`
