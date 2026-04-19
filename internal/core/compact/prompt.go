package compact

import (
	"regexp"
	"strings"
)

// noToolsPreamble is injected at the start of compact prompts to prevent
// the summarizer model from calling tools. Matches TS NO_TOOLS_PREAMBLE.
const noToolsPreamble = `CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.

- Do NOT use Read, Bash, Grep, Glob, Edit, Write, or ANY other tool.
- You already have all the context you need in the conversation above.
- Tool calls will be REJECTED and will waste your only turn — you will fail the task.
- Your entire response must be plain text: an <analysis> block followed by a <summary> block.

`

// noToolsTrailer is appended to compact prompts as a final reminder.
// Matches TS NO_TOOLS_TRAILER.
const noToolsTrailer = `

REMINDER: Do NOT call any tools. Respond with plain text only — an <analysis> block followed by a <summary> block. Tool calls will be rejected and you will fail the task.`

// baseCompactPrompt is the main 9-section compact summary template.
// Matches TS BASE_COMPACT_PROMPT.
const baseCompactPrompt = `Your task is to create a detailed summary of the conversation so far, paying close attention to the user's explicit requests and your previous actions.
This summary should be thorough in capturing technical details, code patterns, and architectural decisions that would be essential for continuing development work without losing context.

Before providing your final summary, wrap your analysis in <analysis> tags to organize your thoughts and ensure you've covered all necessary points. In your analysis process:

1. Chronologically analyze each message and section of the conversation. For each section thoroughly identify:
   - The user's explicit requests and intents
   - Your approach to addressing the user's requests
   - Key decisions, technical concepts and code patterns
   - Specific details like:
     - file names
     - full code snippets
     - function signatures
     - file edits
   - Errors that you ran into and how you fixed them
   - Pay special attention to specific user feedback that you received, especially if the user told you to do something differently.
2. Double-check for technical accuracy and completeness, addressing each required element thoroughly.

Your summary should include the following sections:

1. Primary Request and Intent: Capture all of the user's explicit requests and intents in detail
2. Key Technical Concepts: List all important technical concepts, technologies, and frameworks discussed.
3. Files and Code Sections: Enumerate specific files and code sections examined, modified, or created. Pay special attention to the most recent messages and include full code snippets where applicable and include a summary of why this file read or edit is important.
4. Errors and fixes: List all errors that you ran into, and how you fixed them. Pay special attention to specific user feedback that you received, especially if the user told you to do something differently.
5. Problem Solving: Document problems solved and any ongoing troubleshooting efforts.
6. All user messages: List ALL user messages that are not tool results. These are critical for understanding the users' feedback and changing intent.
7. Pending Tasks: Outline any pending tasks that you have explicitly been asked to work on.
8. Current Work: Describe in detail precisely what was being worked on immediately before this summary request, paying special attention to the most recent messages from both user and assistant. Include file names and code snippets where applicable.
9. Optional Next Step: List the next step that you will take that is related to the most recent work you were doing. IMPORTANT: ensure that this step is DIRECTLY in line with the user's most recent explicit requests, and the task you were working on immediately before this summary request. If your last task was concluded, then only list next steps if they are explicitly in line with the users request. Do not start on tangential requests or really old requests that were already completed without confirming with the user first.
                       If there is a next step, include direct quotes from the most recent conversation showing exactly what task you were working on and where you left off. This should be verbatim to ensure there's no drift in task interpretation.

Here's an example of how your output should be structured:

<example>
<analysis>
[Your thought process, ensuring all points are covered thoroughly and accurately]
</analysis>

<summary>
1. Primary Request and Intent:
   [Detailed description]

2. Key Technical Concepts:
   - [Concept 1]
   - [Concept 2]
   - [...]

3. Files and Code Sections:
   - [File Name 1]
      - [Summary of why this file is important]
      - [Summary of the changes made to this file, if any]
      - [Important Code Snippet]
   - [File Name 2]
      - [Important Code Snippet]
   - [...]

4. Errors and fixes:
    - [Detailed description of error 1]:
      - [How you fixed the error]
      - [User feedback on the error if any]
    - [...]

5. Problem Solving:
   [Description of solved problems and ongoing troubleshooting]

6. All user messages:
    - [Detailed non tool use user message]
    - [...]

7. Pending Tasks:
   - [Task 1]
   - [Task 2]
   - [...]

8. Current Work:
   [Precise description of current work]

9. Optional Next Step:
   [Optional Next step to take]

</summary>
</example>

Please provide your summary based on the conversation so far, following this structure and ensuring precision and thoroughness in your response.

There may be additional summarization instructions provided in the included context. If you remember that, remember to follow them when creating this summary. Examples of instructions include:
<example>
## Compact Instructions
When summarizing the conversation focus on typescript code changes and also remember the mistakes you made and how you fixed them.
</example>

<example>
# Summary instructions
When you are using compact - please focus on test output and code changes. Include file reads verbatim.
</example>

`

// GetCompactPrompt returns the full compact summary prompt, optionally
// augmented with custom instructions. Aligns with TS getCompactPrompt.
func GetCompactPrompt(customInstructions string) string {
	prompt := noToolsPreamble + baseCompactPrompt
	if strings.TrimSpace(customInstructions) != "" {
		prompt += "\n\nAdditional Instructions:\n" + customInstructions
	}
	prompt += noToolsTrailer
	return prompt
}

// analysisRegex matches the <analysis>...</analysis> block in summary output.
var analysisRegex = regexp.MustCompile(`(?s)<analysis>.*?</analysis>`)

// summaryTagRegex matches the <summary>...</summary> block in summary output.
var summaryTagRegex = regexp.MustCompile(`(?s)<summary>(.*?)</summary>`)

// multiNewlineRegex matches three or more consecutive newlines.
var multiNewlineRegex = regexp.MustCompile(`\n\n+`)

// FormatCompactSummary processes the raw LLM summary output by stripping
// the analysis scratchpad and converting <summary> tags to readable headers.
// Aligns with TS formatCompactSummary.
func FormatCompactSummary(summary string) string {
	formatted := summary

	// Strip analysis section — it's a drafting scratchpad.
	formatted = analysisRegex.ReplaceAllString(formatted, "")

	// Extract and format summary section.
	match := summaryTagRegex.FindStringSubmatch(formatted)
	if len(match) >= 2 {
		content := strings.TrimSpace(match[1])
		formatted = summaryTagRegex.ReplaceAllString(formatted, "Summary:\n"+content)
	}

	// Clean up extra whitespace.
	formatted = multiNewlineRegex.ReplaceAllString(formatted, "\n\n")

	return strings.TrimSpace(formatted)
}
