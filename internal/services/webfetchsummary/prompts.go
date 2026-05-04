// Package webfetchsummary provides Haiku-based content summarisation for
// pages fetched by WebFetchTool.
//
// All prompt literals in this file are taken verbatim from
// src/tools/WebFetchTool/prompt.ts and utils.ts to preserve model behaviour.
package webfetchsummary

import "fmt"

// querySource is the identifier passed to the haiku layer for log
// correlation. Mirrors the TS querySource string ("web_fetch_apply")
// verbatim.
const querySource = "web_fetch_apply"

// makeSecondaryModelPrompt builds the prompt sent to the secondary
// (Haiku) model. Mirrors the TS makeSecondaryModelPrompt function in
// src/tools/WebFetchTool/prompt.ts.
//
// The guidelines differ based on whether the domain is preapproved:
//   - preapproved: allow generous quoting and code examples.
//   - non-preapproved: enforce strict 125-char quote limit, copyright
//     disclaimers, and no song lyrics.
func makeSecondaryModelPrompt(markdownContent string, prompt string, isPreapprovedDomain bool) string {
	var guidelines string
	if isPreapprovedDomain {
		guidelines = `Provide a concise response based on the content above. Include relevant details, code examples, and documentation excerpts as needed.`
	} else {
		guidelines = `Provide a concise response based only on the content above. In your response:
 - Enforce a strict 125-character maximum for quotes from any source document. Open Source Software is ok as long as we respect the license.
 - Use quotation marks for exact language from articles; any language outside of the quotation should never be word-for-word the same.
 - You are not a lawyer and never comment on the legality of your own prompts and responses.
 - Never produce or reproduce exact song lyrics.`
	}

	return fmt.Sprintf(`
Web page content:
---
%s
---

%s

%s
`, markdownContent, prompt, guidelines)
}
