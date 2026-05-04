// Package rename generates kebab-case session name suggestions via the
// Haiku helper. The Go port mirrors src/commands/rename/generateSessionName.ts
// and is wired via the haiku.Querier interface.
//
// All prompt literals in this file are kept verbatim against the TS source
// to preserve model behaviour.
package rename

// systemPrompt is the system message used for rename suggestion generation.
// Verbatim copy of the prompt from generateSessionName.ts.
const systemPrompt = `Generate a short kebab-case name (2-4 words) that captures the main topic of this conversation. Use lowercase words separated by hyphens. Examples: "fix-login-bug", "add-auth-feature", "refactor-api-client", "debug-test-failures". Return JSON with a "name" field.`

// querySource is the identifier passed to the haiku layer for log
// correlation. Mirrors the TS querySource string verbatim.
const querySource = "rename_generate_name"
