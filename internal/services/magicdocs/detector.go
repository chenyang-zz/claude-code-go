package magicdocs

import (
	"regexp"
	"strings"
)

// MagicDocInfo holds the parsed result of a Magic Doc header detection.
type MagicDocInfo struct {
	Title        string // The title from "# MAGIC DOC: <title>"
	Instructions string // Optional italicized instructions line after the header
}

var (
	magicDocHeaderPattern = regexp.MustCompile(`(?m)^#\s*MAGIC\s+DOC:\s*(.+)$`)
	italicsPattern        = regexp.MustCompile(`^[_*](.+?)[_*]\s*$`)
	nextLinePattern       = regexp.MustCompile(`^\s*\n(?:\s*\n)?(.+?)(?:\n|$)`)
)

// DetectMagicDocHeader checks if content has a Magic Doc header.
// Only matches headers on the first line. Returns nil if no header is found.
func DetectMagicDocHeader(content string) *MagicDocInfo {
	loc := magicDocHeaderPattern.FindStringSubmatchIndex(content)
	if loc == nil {
		return nil
	}

	// Extract the title from capture group 1.
	title := string(magicDocHeaderPattern.ExpandString(nil, "$1", content, loc))
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}

	info := &MagicDocInfo{Title: title}

	// Check for italicized instructions on the line immediately after the header.
	headerEnd := loc[1]
	afterHeader := content[headerEnd:]

	// Skip leading whitespace/newlines after the header, then read next non-blank line.
	nextLineMatch := nextLinePattern.FindStringSubmatch(afterHeader)
	if nextLineMatch != nil {
		italicsMatch := italicsPattern.FindStringSubmatch(nextLineMatch[1])
		if italicsMatch != nil {
			info.Instructions = strings.TrimSpace(italicsMatch[1])
		}
	}

	return info
}
