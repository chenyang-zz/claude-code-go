package shared

import "strings"

// Hunk describes one minimal structured diff segment shared by write-oriented tools.
type Hunk struct {
	// OldStart stores the 1-based starting line in the original content.
	OldStart int `json:"oldStart"`
	// OldLines stores how many original lines participate in this hunk.
	OldLines int `json:"oldLines"`
	// NewStart stores the 1-based starting line in the updated content.
	NewStart int `json:"newStart"`
	// NewLines stores how many updated lines participate in this hunk.
	NewLines int `json:"newLines"`
	// Lines stores diff display lines prefixed with " ", "-" or "+".
	Lines []string `json:"lines"`
}

// BuildStructuredPatch computes a minimal line-based patch for one file mutation.
func BuildStructuredPatch(oldContent string, newContent string) []Hunk {
	if oldContent == newContent {
		return nil
	}

	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	prefix := 0
	for prefix < len(oldLines) && prefix < len(newLines) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}

	oldSuffix := len(oldLines)
	newSuffix := len(newLines)
	for oldSuffix > prefix && newSuffix > prefix && oldLines[oldSuffix-1] == newLines[newSuffix-1] {
		oldSuffix--
		newSuffix--
	}

	oldStart := prefix + 1
	newStart := prefix + 1
	oldChanged := oldLines[prefix:oldSuffix]
	newChanged := newLines[prefix:newSuffix]
	lines := make([]string, 0, len(oldChanged)+len(newChanged))

	for _, line := range oldChanged {
		lines = append(lines, "-"+line)
	}
	for _, line := range newChanged {
		lines = append(lines, "+"+line)
	}

	return []Hunk{
		{
			OldStart: oldStart,
			OldLines: len(oldChanged),
			NewStart: newStart,
			NewLines: len(newChanged),
			Lines:    lines,
		},
	}
}

// splitLines converts file content into logical lines while dropping line terminators.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}

	lines := strings.Split(content, "\n")
	if strings.HasSuffix(content, "\n") {
		lines = lines[:len(lines)-1]
	}

	return lines
}
