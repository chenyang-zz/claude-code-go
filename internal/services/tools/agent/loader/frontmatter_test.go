package loader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := "# Hello\n\nThis is markdown without frontmatter."
	fm, body, err := ParseFrontmatter(content)
	require.NoError(t, err)
	assert.Nil(t, fm)
	assert.Equal(t, content, body)
}

func TestParseFrontmatter_EmptyFrontmatter(t *testing.T) {
	content := "---\n---\n# Hello\n"
	fm, body, err := ParseFrontmatter(content)
	require.NoError(t, err)
	assert.NotNil(t, fm)
	assert.Empty(t, fm)
	assert.Equal(t, "# Hello\n", body)
}

func TestParseFrontmatter_BasicFields(t *testing.T) {
	content := "---\nname: explore\ndescription: A search agent\n---\n# System Prompt\nSearch only."
	fm, body, err := ParseFrontmatter(content)
	require.NoError(t, err)
	assert.Equal(t, "explore", fm["name"])
	assert.Equal(t, "A search agent", fm["description"])
	assert.Equal(t, "# System Prompt\nSearch only.", body)
}

func TestParseFrontmatter_WithTrailingNewline(t *testing.T) {
	content := "---\nname: test\n---\nBody here\n"
	fm, body, err := ParseFrontmatter(content)
	require.NoError(t, err)
	assert.Equal(t, "test", fm["name"])
	assert.Equal(t, "Body here\n", body)
}

func TestParseFrontmatter_NoTrailingNewline(t *testing.T) {
	content := "---\nname: test\n---\nBody here"
	fm, body, err := ParseFrontmatter(content)
	require.NoError(t, err)
	assert.Equal(t, "test", fm["name"])
	assert.Equal(t, "Body here", body)
}

func TestParseFrontmatter_YAMLList(t *testing.T) {
	content := "---\ntools:\n  - Read\n  - Grep\n---\nBody"
	fm, body, err := ParseFrontmatter(content)
	require.NoError(t, err)
	assert.Equal(t, []any{"Read", "Grep"}, fm["tools"])
	assert.Equal(t, "Body", body)
}

func TestParseFrontmatter_InvalidYAML(t *testing.T) {
	content := "---\nname: {invalid\n---\nBody"
	_, _, err := ParseFrontmatter(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML frontmatter")
}

func TestParseFrontmatter_MultilineBody(t *testing.T) {
	content := "---\nname: agent\n---\nLine 1\nLine 2\nLine 3\n"
	fm, body, err := ParseFrontmatter(content)
	require.NoError(t, err)
	assert.Equal(t, "agent", fm["name"])
	assert.Equal(t, "Line 1\nLine 2\nLine 3\n", body)
}
