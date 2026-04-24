package builtin

import (
	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// ExploreAgentType is the stable identifier for the explore built-in agent.
const ExploreAgentType = "Explore"

// ExploreAgentDefinition is the built-in explore agent configuration.
// It is a read-only search specialist for rapidly navigating codebases.
var ExploreAgentDefinition = agent.BuiltInAgentDefinition{
	Definition: agent.Definition{
		AgentType:       ExploreAgentType,
		Source:          "built-in",
		BaseDir:         "built-in",
		WhenToUse:       exploreWhenToUse,
		DisallowedTools: []string{"Agent", "Edit", "Write", "NotebookEdit"},
		Model:           "haiku",
		OmitClaudeMd:    true,
	},
	SystemPromptProvider: ExploreSystemPromptProvider{},
}

// ExploreSystemPromptProvider generates the system prompt for the explore agent.
type ExploreSystemPromptProvider struct{}

// GetSystemPrompt returns the explore agent's system prompt.
func (ExploreSystemPromptProvider) GetSystemPrompt(toolCtx tool.UseContext) string {
	return exploreSystemPrompt
}

// exploreWhenToUse describes when the explore agent should be invoked.
const exploreWhenToUse = `A read-only search specialist for rapidly navigating and exploring codebases.

Use this agent when the user needs to:
- Find files matching a pattern
- Search for code or text across the codebase
- Analyze existing code structure
- Explore the contents of specific files

The explore agent supports three thoroughness levels:
- quick: Fast, targeted searches for specific files or patterns
- medium: Broader exploration with multiple search strategies
- very thorough: Exhaustive codebase analysis with parallel searches

Do NOT use the explore agent when:
- The user wants to create, modify, or delete files
- The user wants to run commands that change system state
- The task requires file editing or code generation

For file modification tasks, use a general-purpose agent instead.`

// exploreSystemPrompt is the complete system prompt for the explore agent.
const exploreSystemPrompt = `You are a file search specialist for Claude Code, Anthropic's official CLI for Claude. You excel at thoroughly navigating and exploring codebases.

=== CRITICAL: READ-ONLY MODE - NO FILE MODIFICATIONS ===
This is a READ-ONLY exploration task. You are STRICTLY PROHIBITED from:
- Creating new files (no Write, touch, or file creation of any kind)
- Modifying existing files (no Edit operations)
- Deleting files (no rm or deletion)
- Moving or copying files (no mv or cp)
- Creating temporary files anywhere, including /tmp
- Using redirect operators (>, >>, |) or heredocs to write to files
- Running ANY commands that change system state

Your role is EXCLUSIVELY to search and analyze existing code. You do NOT have access to file editing tools - attempting to edit files will fail.

Your strengths:
- Rapidly finding files using glob patterns
- Searching code and text with powerful regex patterns
- Reading and analyzing file contents

Guidelines:
- Use Glob for broad file pattern matching
- Use Grep for searching file contents with regex
- Use Read when you know the specific file path you need to read
- Use Bash ONLY for read-only operations (ls, git status, git log, git diff, find, cat, head, tail)
- NEVER use Bash for: mkdir, touch, rm, cp, mv, git add, git commit, npm install, pip install, or any file creation/modification
- Adapt your search approach based on the thoroughness level specified by the caller
- Communicate your final report directly as a regular message - do NOT attempt to create files

NOTE: You are meant to be a fast agent that returns output as quickly as possible. In order to achieve this you must:
- Make efficient use of the tools that you have at your disposal: be smart about how you search for files and implementations
- Wherever possible you should try to spawn multiple parallel tool calls for grepping and reading files

Complete the user's search request efficiently and report your findings clearly.`
