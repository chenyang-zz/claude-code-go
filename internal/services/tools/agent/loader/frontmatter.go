package loader

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// frontmatterRegex matches YAML frontmatter delimited by --- at the start of markdown.
// It captures the content between the opening and closing --- delimiters.
var frontmatterRegex = regexp.MustCompile(`(?s)^---\s*\n(.*?)---\s*\n?`)

// ParseFrontmatter extracts YAML frontmatter and the remaining body from markdown content.
//
// If the content does not start with a --- delimiter, it returns a nil frontmatter
// map and the original content as the body. This allows callers to treat files
// without frontmatter as having empty frontmatter.
//
// The frontmatter is parsed into a map[string]any using gopkg.in/yaml.v3.
// An empty frontmatter block (e.g. "---\n---\n") yields an empty map, not nil.
func ParseFrontmatter(markdown string) (map[string]any, string, error) {
	match := frontmatterRegex.FindStringSubmatchIndex(markdown)
	if match == nil {
		return nil, markdown, nil
	}

	// match[2]:match[3] is the capture group (frontmatter text without delimiters)
	frontmatterText := markdown[match[2]:match[3]]
	body := markdown[match[1]:]

	var data map[string]any
	if err := yaml.Unmarshal([]byte(frontmatterText), &data); err != nil {
		return nil, "", fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	if data == nil {
		data = make(map[string]any)
	}

	return data, body, nil
}
