package bundled

import (
	"os"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

// languageIndicators maps languages to file indicators for project detection.
var languageIndicators = map[string][]string{
	"python":     {".py", "requirements.txt", "pyproject.toml", "setup.py", "Pipfile"},
	"typescript": {".ts", ".tsx", "tsconfig.json", "package.json"},
	"java":       {".java", "pom.xml", "build.gradle"},
	"go":         {".go", "go.mod"},
	"ruby":       {".rb", "Gemfile"},
	"csharp":     {".cs", ".csproj"},
	"php":        {".php", "composer.json"},
}

// detectLanguage attempts to detect the primary language of the current project.
func detectLanguage() string {
	entries, err := os.ReadDir(".")
	if err != nil {
		return ""
	}

	for lang, indicators := range languageIndicators {
		if len(indicators) == 0 {
			continue
		}
		for _, indicator := range indicators {
			if strings.HasPrefix(indicator, ".") {
				for _, e := range entries {
					if strings.HasSuffix(e.Name(), indicator) {
						return lang
					}
				}
			} else {
				for _, e := range entries {
					if e.Name() == indicator {
						return lang
					}
				}
			}
		}
	}
	return ""
}

const claudeApiBasePrompt = `# Claude API / SDK Skill

Help the user build applications with the Claude API, Anthropic SDKs, or Agent SDK.

## Detecting the Right SDK

This skill TRIGGERS when:
- Code imports anthropic / @anthropic-ai/sdk / claude_agent_sdk
- User asks to use Claude API, Anthropic SDKs, or Agent SDK

This skill does NOT trigger for: OpenAI imports, general programming, ML/data-science tasks.

## Quick Reference by Task

**Single text classification/summarization/extraction/Q&A:**
→ Use Claude API Messages endpoint

**Chat UI or real-time response display:**
→ Use Claude API with streaming enabled

**Long-running conversations (may exceed context window):**
→ See Compaction section of the API docs

**Prompt caching / optimize caching:**
→ Use cache_control breakpoints on system messages and tool definitions

**Function calling / tool use / agents:**
→ Define tools in the API request and parse tool_use responses

**Batch processing (non-latency-sensitive):**
→ Use the Claude API Batches endpoint

**Agent with built-in tools:**
→ Use the Claude Agent SDK (Python & TypeScript)

**Error handling:**
→ Common error codes: 429 (rate limit), 529 (overload), 400 (invalid request), 401 (auth)

## Getting the Latest Docs

Use WebFetch to get the latest documentation:
- Python SDK: https://docs.anthropic.com/en/api/client-sdks/python
- TypeScript SDK: https://docs.anthropic.com/en/api/client-sdks/typescript
- Agent SDK: https://docs.anthropic.com/en/docs/agents-and-tools/agent-sdk
- API Reference: https://docs.anthropic.com/en/api

## Common Pitfalls
- Always handle rate limits (429) with exponential backoff
- Use prompt caching for repeated system messages and tool definitions
- Stream responses for better UX in chat applications
- Don't forget to set appropriate max_tokens for the expected response length`

func registerClaudeApiSkill() {
	isEnabled := func() bool {
		return os.Getenv("CLAUDE_CODE_BUILDING_APPS") != ""
	}

	skill.RegisterBundledSkill(skill.BundledSkillDefinition{
		Name:         "claude-api",
		Description:  "Build apps with the Claude API or Anthropic SDK. TRIGGER when: code imports anthropic/@anthropic-ai-sdk/claude_agent_sdk, or user asks to use Claude API, Anthropic SDKs, or Agent SDK. DO NOT TRIGGER when: code imports openai/other AI SDK, general programming, or ML/data-science tasks.",
		AllowedTools: []string{"Read", "Grep", "Glob", "WebFetch"},
		UserInvocable: true,
		IsEnabled:     isEnabled,
		GetPromptForCommand: func(args string) (string, error) {
			lang := detectLanguage()
			prompt := claudeApiBasePrompt

			if lang != "" {
				prompt += "\n\n## Detected Language: " + lang
			} else {
				prompt += "\n\nNo project language was auto-detected. Ask the user which language they are using."
			}

			if args != "" {
				prompt += "\n\n## User Request\n\n" + args
			}

			prompt += "\n\nUse WebFetch to retrieve the latest SDK/API documentation for the detected language."
			return prompt, nil
		},
	})
}
