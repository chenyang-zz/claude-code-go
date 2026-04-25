package web_fetch

import (
	"fmt"
	"regexp"
	"strings"
)

// htmlToMarkdown converts a small subset of HTML into readable Markdown.
// It is intentionally lightweight and does not aim for full Turndown parity.
func htmlToMarkdown(html string) string {
	md := html

	// Remove script and style blocks entirely.
	md = removeTagBlock(md, "script")
	md = removeTagBlock(md, "style")

	// Headings
	md = replaceSimpleTag(md, "h1", "# ")
	md = replaceSimpleTag(md, "h2", "## ")
	md = replaceSimpleTag(md, "h3", "### ")
	md = replaceSimpleTag(md, "h4", "#### ")
	md = replaceSimpleTag(md, "h5", "##### ")
	md = replaceSimpleTag(md, "h6", "###### ")

	// Block elements
	md = replaceSimpleTag(md, "blockquote", "> ")
	md = replaceSimpleTag(md, "p", "")

	// Inline formatting
	md = replaceInlineTag(md, "strong", "**")
	md = replaceInlineTag(md, "b", "**")
	md = replaceInlineTag(md, "em", "_")
	md = replaceInlineTag(md, "i", "_")
	md = replaceInlineTag(md, "code", "`")

	// Links
	md = convertLinks(md)

	// Lists
	md = convertLists(md)

	// Pre / code blocks
	md = convertPreBlocks(md)

	// Clean up remaining tags roughly.
	md = stripRemainingTags(md)

	// Collapse excessive blank lines.
	md = collapseBlankLines(md)

	return strings.TrimSpace(md)
}

// truncateMarkdown caps markdown content to the maximum allowed length.
func truncateMarkdown(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}
	return content[:maxLength] + "\n\n[Content truncated due to length...]"
}

func removeTagBlock(html, tag string) string {
	re := regexp.MustCompile(fmt.Sprintf(`(?is)<\s*%s\b[^>]*>.*?</\s*%s\s*>`, tag, tag))
	return re.ReplaceAllString(html, "")
}

func replaceSimpleTag(html, tag, prefix string) string {
	openRe := regexp.MustCompile(fmt.Sprintf(`(?i)<\s*%s\b[^>]*>`, tag))
	closeRe := regexp.MustCompile(fmt.Sprintf(`(?i)</\s*%s\s*>`, tag))

	result := openRe.ReplaceAllString(html, prefix)
	result = closeRe.ReplaceAllString(result, "\n\n")
	return result
}

func replaceInlineTag(html, tag, marker string) string {
	openRe := regexp.MustCompile(fmt.Sprintf(`(?i)<\s*%s\b[^>]*>`, tag))
	closeRe := regexp.MustCompile(fmt.Sprintf(`(?i)</\s*%s\s*>`, tag))

	result := openRe.ReplaceAllString(html, marker)
	result = closeRe.ReplaceAllString(result, marker)
	return result
}

func convertLinks(html string) string {
	// <a href="...">text</a> → [text](href)
	re := regexp.MustCompile(`(?i)<\s*a\s+[^>]*href\s*=\s*["']([^"']+)["'][^>]*>(.*?)</\s*a\s*>`)
	return re.ReplaceAllStringFunc(html, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		href := strings.TrimSpace(submatches[1])
		text := stripTags(submatches[2])
		text = strings.TrimSpace(text)
		if text == "" {
			text = href
		}
		return fmt.Sprintf("[%s](%s)", text, href)
	})
}

func convertLists(html string) string {
	result := html

	// Unordered lists
	ulOpen := regexp.MustCompile(`(?i)<\s*ul\b[^>]*>`)
	ulClose := regexp.MustCompile(`(?i)</\s*ul\s*>`)
	result = ulOpen.ReplaceAllString(result, "\n")
	result = ulClose.ReplaceAllString(result, "\n")

	// Ordered lists
	olOpen := regexp.MustCompile(`(?i)<\s*ol\b[^>]*>`)
	olClose := regexp.MustCompile(`(?i)</\s*ol\s*>`)
	result = olOpen.ReplaceAllString(result, "\n")
	result = olClose.ReplaceAllString(result, "\n")

	// List items
	liOpen := regexp.MustCompile(`(?i)<\s*li\b[^>]*>`)
	liClose := regexp.MustCompile(`(?i)</\s*li\s*>`)
	result = liOpen.ReplaceAllString(result, "\n- ")
	result = liClose.ReplaceAllString(result, "\n")

	return result
}

func convertPreBlocks(html string) string {
	result := html

	// <pre><code>...</code></pre>
	preCodeRe := regexp.MustCompile(`(?is)<\s*pre\b[^>]*>\s*<\s*code\b[^>]*>(.*?)</\s*code\s*>\s*</\s*pre\s*>`)
	result = preCodeRe.ReplaceAllString(result, "\n\n```\n$1\n```\n\n")

	// <pre>...</pre> without code
	preRe := regexp.MustCompile(`(?is)<\s*pre\b[^>]*>(.*?)</\s*pre\s*>`)
	result = preRe.ReplaceAllString(result, "\n\n```\n$1\n```\n\n")

	// <code>...</code> inline (if not already inside pre)
	codeRe := regexp.MustCompile(`(?i)<\s*code\b[^>]*>(.*?)</\s*code\s*>`)
	result = codeRe.ReplaceAllString(result, "`$1`")

	return result
}

func stripTags(html string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(html, "")
}

func stripRemainingTags(html string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(html, "")
}

func collapseBlankLines(text string) string {
	re := regexp.MustCompile(`\n{3,}`)
	return re.ReplaceAllString(text, "\n\n")
}
