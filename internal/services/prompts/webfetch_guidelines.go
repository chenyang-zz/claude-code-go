package prompts

import "context"

// WebFetchGuidelinesSection provides guidance on when and how to use the WebFetch tool.
type WebFetchGuidelinesSection struct{}

// Name returns the section identifier.
func (s WebFetchGuidelinesSection) Name() string { return "webfetch_guidelines" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s WebFetchGuidelinesSection) IsVolatile() bool { return false }

// Compute generates the WebFetch usage guidelines.
func (s WebFetchGuidelinesSection) Compute(ctx context.Context) (string, error) {
	return `# WebFetch Tool Guidelines

Use the WebFetch tool when you need to retrieve and analyze web content that is not available through other means.

## When to use WebFetch

- Fetch documentation, API references, or guides to answer user questions.
- Retrieve content from web pages to inform code changes or recommendations.
- Look up information from external sources when no local or built-in tool covers the need.

## When NOT to use WebFetch

- If an MCP server provides a web fetch tool, prefer the MCP tool instead (it typically has fewer restrictions).
- For GitHub URLs (issues, PRs, repositories), prefer using the Bash tool with the gh CLI (e.g., ` + "`gh pr view`, `gh issue view`, `gh api`" + `) rather than WebFetch.

## URL requirements

- The URL must be a fully-formed valid URL (including scheme and host).
- HTTP URLs are automatically upgraded to HTTPS (localhost is exempt).

## Permission behavior

- Pre-approved domains are allowed automatically without user confirmation.
- Non-pre-approved domains are matched against deny/ask/allow rules; the default is to ask the user for permission.
- If the user denies a WebFetch request, do not retry the same URL. Adjust your approach instead.

## Output format

- HTML content is converted to Markdown (headings, paragraphs, links, lists, code blocks, blockquotes, emphasis).
- Very large content may be summarized rather than returned in full.
- Output is truncated at 100KB.

## Caching

- WebFetch uses a self-cleaning cache with a 15-minute TTL. Re-fetching the same URL within this window returns cached results.

## Redirects

- Same-host and www-subdomain redirects are followed automatically.
- Cross-host redirects are not followed automatically. The tool will inform you of the redirect URL, and you must make a new WebFetch request with that URL.`, nil
}
