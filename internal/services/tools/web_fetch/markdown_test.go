package web_fetch

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTMLToMarkdown_Headings(t *testing.T) {
	html := "<h1>Title</h1><h2>Subtitle</h2><h3>Section</h3>"
	md := htmlToMarkdown(html)
	assert.Contains(t, md, "# Title")
	assert.Contains(t, md, "## Subtitle")
	assert.Contains(t, md, "### Section")
}

func TestHTMLToMarkdown_Paragraphs(t *testing.T) {
	html := "<p>First paragraph.</p><p>Second paragraph.</p>"
	md := htmlToMarkdown(html)
	assert.Contains(t, md, "First paragraph.")
	assert.Contains(t, md, "Second paragraph.")
}

func TestHTMLToMarkdown_Links(t *testing.T) {
	html := `<a href="https://example.com">Click here</a>`
	md := htmlToMarkdown(html)
	assert.Contains(t, md, "[Click here](https://example.com)")
}

func TestHTMLToMarkdown_Strong(t *testing.T) {
	html := "<strong>bold</strong> and <b>also bold</b>"
	md := htmlToMarkdown(html)
	assert.Contains(t, md, "**bold**")
	assert.Contains(t, md, "**also bold**")
}

func TestHTMLToMarkdown_Em(t *testing.T) {
	html := "<em>italic</em> and <i>also italic</i>"
	md := htmlToMarkdown(html)
	assert.Contains(t, md, "_italic_")
	assert.Contains(t, md, "_also italic_")
}

func TestHTMLToMarkdown_Code(t *testing.T) {
	html := "<code>inline</code>"
	md := htmlToMarkdown(html)
	assert.Contains(t, md, "`inline`")
}

func TestHTMLToMarkdown_Blockquote(t *testing.T) {
	html := "<blockquote>A quote</blockquote>"
	md := htmlToMarkdown(html)
	assert.Contains(t, md, "> A quote")
}

func TestHTMLToMarkdown_List(t *testing.T) {
	html := "<ul><li>Item 1</li><li>Item 2</li></ul>"
	md := htmlToMarkdown(html)
	assert.Contains(t, md, "- Item 1")
	assert.Contains(t, md, "- Item 2")
}

func TestHTMLToMarkdown_RemovesScript(t *testing.T) {
	html := "<p>Safe</p><script>alert('xss')</script>"
	md := htmlToMarkdown(html)
	assert.Contains(t, md, "Safe")
	assert.NotContains(t, md, "alert")
}

func TestHTMLToMarkdown_RemovesStyle(t *testing.T) {
	html := "<p>Safe</p><style>.red{color:red}</style>"
	md := htmlToMarkdown(html)
	assert.Contains(t, md, "Safe")
	assert.NotContains(t, md, ".red")
}

func TestTruncateMarkdown_UnderLimit(t *testing.T) {
	content := "short content"
	result := truncateMarkdown(content, 100)
	assert.Equal(t, content, result)
}

func TestTruncateMarkdown_OverLimit(t *testing.T) {
	content := strings.Repeat("a", 200)
	result := truncateMarkdown(content, 100)
	assert.True(t, strings.HasSuffix(result, "[Content truncated due to length...]"))
	assert.Equal(t, 100+len("\n\n[Content truncated due to length...]"), len(result))
}
