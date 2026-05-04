// Package sessiontitle generates concise sentence-case session titles via
// the Haiku helper. The Go port mirrors src/utils/sessionTitle.ts and is
// wired via the haiku.Querier interface.
//
// All prompt literals in this file are kept verbatim against the TS source
// to preserve model behaviour.
package sessiontitle

// systemPrompt is the system message used for session title generation.
// Verbatim copy of SESSION_TITLE_PROMPT from the TS source.
const systemPrompt = `Generate a concise, sentence-case title (3-7 words) that captures the main topic or goal of this coding session. The title should be clear enough that the user recognizes the session in a list. Use sentence case: capitalize only the first word and proper nouns.

Return JSON with a single "title" field.

Good examples:
{"title": "Fix login button on mobile"}
{"title": "Add OAuth authentication"}
{"title": "Debug failing CI tests"}
{"title": "Refactor API client error handling"}

Bad (too vague): {"title": "Code changes"}
Bad (too long): {"title": "Investigate and fix the issue where the login button does not respond on mobile devices"}
Bad (wrong case): {"title": "Fix Login Button On Mobile"}`

// querySource is the identifier passed to the haiku layer for log
// correlation. Mirrors the TS querySource string verbatim.
const querySource = "generate_session_title"
