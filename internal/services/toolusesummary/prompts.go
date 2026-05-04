// Package toolusesummary generates ~30-character single-line summaries of
// completed tool batches. The Go port mirrors
// src/services/toolUseSummary/toolUseSummaryGenerator.ts and is wired via the
// haiku.Querier interface so it remains testable without a real Anthropic
// client.
//
// All prompt literals in this file are kept verbatim against the TS source
// to preserve model behaviour. Editing them changes the produced summary
// style across the SDK and should be considered carefully.
package toolusesummary

// SystemPrompt is the system message used for tool use summary generation.
// Verbatim copy of TOOL_USE_SUMMARY_SYSTEM_PROMPT from the TS source.
const SystemPrompt = `Write a short summary label describing what these tool calls accomplished. It appears as a single-line row in a mobile app and truncates around 30 characters, so think git-commit-subject, not sentence.

Keep the verb in past tense and the most distinctive noun. Drop articles, connectors, and long location context first.

Examples:
- Searched in auth/
- Fixed NPE in UserService
- Created signup endpoint
- Read config.json
- Ran failing tests`

// userPromptTemplate is the format string for the user prompt sent to Haiku.
// Substitution order: contextPrefix, toolSummaries.
//
// TS reference: `${contextPrefix}Tools completed:\n\n${toolSummaries}\n\nLabel:`
const userPromptTemplate = "%sTools completed:\n\n%s\n\nLabel:"

// contextPrefixTemplate optionally prefixes the user prompt with the
// assistant's last message, capped at 200 characters by callers.
//
// TS reference: `User's intent (from assistant's last message): ${lastAssistantText.slice(0, 200)}\n\n`
const contextPrefixTemplate = "User's intent (from assistant's last message): %s\n\n"

// querySource is the identifier passed to the haiku layer for log
// correlation. Mirrors the TS querySource string verbatim.
const querySource = "tool_use_summary_generation"

// lastAssistantTextLimit caps the assistant context prefix at 200 characters
// before insertion into the prompt template, mirroring TS slice(0, 200).
const lastAssistantTextLimit = 200

// truncationLimit caps each tool input/output JSON encoding at 300 characters
// (including the trailing "..." sentinel) before insertion into the prompt.
const truncationLimit = 300
