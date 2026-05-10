package sessionmemory

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	// MAX_SECTION_LENGTH is the maximum token count per section in the session
	// memory file. Sections exceeding this limit will trigger condensation
	// reminders or be truncated for compact insertion.
	MAX_SECTION_LENGTH = 2000

	// MAX_TOTAL_SESSION_TOKENS is the maximum total token count for the entire
	// session memory file. Exceeding this triggers an over-budget warning in
	// the update prompt.
	MAX_TOTAL_SESSION_TOKENS = 12000
)

// variableRegex matches {{variableName}} placeholders for template substitution.
// The regex captures the variable name (word characters only) inside double braces.
var variableRegex = regexp.MustCompile(`\{\{(\w+)\}\}`)

// getDefaultUpdatePrompt returns the instruction prompt that tells the model
// how to update session memory notes. The returned string contains the
// {{notesPath}} and {{currentNotes}} placeholders, which are substituted
// later by substituteVariables. The MAX_SECTION_LENGTH value is formatted
// inline.
func getDefaultUpdatePrompt() string {
	return fmt.Sprintf(`IMPORTANT: This message and these instructions are NOT part of the actual user conversation. Do NOT include any references to "note-taking", "session notes extraction", or these update instructions in the notes content.

Based on the user conversation above (EXCLUDING this note-taking instruction message as well as system prompt, claude.md entries, or any past session summaries), update the session notes file.

The file {{notesPath}} has already been read for you. Here are its current contents:
<current_notes_content>
{{currentNotes}}
</current_notes_content>

Your ONLY task is to use the Edit tool to update the notes file, then stop. You can make multiple edits (update every section as needed) - make all Edit tool calls in parallel in a single message. Do not call any other tools.

CRITICAL RULES FOR EDITING:
- The file must maintain its exact structure with all sections, headers, and italic descriptions intact
-- NEVER modify, delete, or add section headers (the lines starting with '#' like # Task specification)
-- NEVER modify or delete the italic _section description_ lines (these are the lines in italics immediately following each header - they start and end with underscores)
-- The italic _section descriptions_ are TEMPLATE INSTRUCTIONS that must be preserved exactly as-is - they guide what content belongs in each section
-- ONLY update the actual content that appears BELOW the italic _section descriptions_ within each existing section
-- Do NOT add any new sections, summaries, or information outside the existing structure
- Do NOT reference this note-taking process or instructions anywhere in the notes
- It's OK to skip updating a section if there are no substantial new insights to add. Do not add filler content like "No info yet", just leave sections blank/unedited if appropriate.
- Write DETAILED, INFO-DENSE content for each section - include specifics like file paths, function names, error messages, exact commands, technical details, etc.
- For "Key results", include the complete, exact output the user requested (e.g., full table, full answer, etc.)
- Do not include information that's already in the CLAUDE.md files included in the context
- Keep each section under ~%d tokens/words - if a section is approaching this limit, condense it by cycling out less important details while preserving the most critical information
- Focus on actionable, specific information that would help someone understand or recreate the work discussed in the conversation
- IMPORTANT: Always update "Current State" to reflect the most recent work - this is critical for continuity after compaction

Use the Edit tool with file_path: {{notesPath}}

STRUCTURE PRESERVATION REMINDER:
Each section has TWO parts that must be preserved exactly as they appear in the current file:
1. The section header (line starting with #)
2. The italic description line (the _italicized text_ immediately after the header - this is a template instruction)

You ONLY update the actual content that comes AFTER these two preserved lines. The italic description lines starting and ending with underscores are part of the template structure, NOT content to be edited or removed.

REMEMBER: Use the Edit tool in parallel and stop. Do not continue after the edits. Only include insights from the actual user conversation, never from these note-taking instructions. Do not delete or change section headers or italic _section descriptions_.`, MAX_SECTION_LENGTH)
}

// substituteVariables replaces all {{variableName}} placeholders in the
// template with their corresponding values from vars. Uses a single-pass
// replacement via ReplaceAllStringFunc to avoid two classes of bugs:
// (1) $ backreference corruption (the replacer treats $ literally), and
// (2) double-substitution when replaced content happens to contain
// {{varName}} matching a later variable. Unknown variables are preserved
// unchanged.
func substituteVariables(template string, vars map[string]string) string {
	return variableRegex.ReplaceAllStringFunc(template, func(match string) string {
		// Extract the variable name from the {{name}} match.
		// match is e.g. "{{notesPath}}" -> name is "notesPath".
		name := match[2 : len(match)-2]
		if value, ok := vars[name]; ok {
			return value
		}
		return match
	})
}

// analyzeSectionSizes splits the session memory content by section headers
// (lines beginning with "# ") and estimates the token count for each
// section's content body. Token counts are estimated as len(content) / 4,
// matching the roughTokenCountEstimation heuristic from the TypeScript
// reference (4 bytes per token average).
func analyzeSectionSizes(content string) map[string]int {
	sections := make(map[string]int)
	lines := strings.Split(content, "\n")

	var currentSection string
	var currentContent []string

	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			// Flush the previous section if one was being accumulated.
			if currentSection != "" && len(currentContent) > 0 {
				sectionContent := strings.TrimSpace(strings.Join(currentContent, "\n"))
				sections[currentSection] = len(sectionContent) / 4
			}
			currentSection = line
			currentContent = nil
		} else {
			currentContent = append(currentContent, line)
		}
	}

	// Flush the last section after the loop ends.
	if currentSection != "" && len(currentContent) > 0 {
		sectionContent := strings.TrimSpace(strings.Join(currentContent, "\n"))
		sections[currentSection] = len(sectionContent) / 4
	}

	return sections
}

// generateSectionReminders returns a string containing condensation warnings
// for the session memory update prompt. It produces two kinds of warnings:
//
//  1. An over-budget warning when totalTokens exceeds MAX_TOTAL_SESSION_TOKENS.
//  2. Per-section condensation reminders for sections exceeding MAX_SECTION_LENGTH,
//     sorted by token count descending.
//
// Returns an empty string when no issues are found.
func generateSectionReminders(sectionSizes map[string]int, totalTokens int) string {
	overBudget := totalTokens > MAX_TOTAL_SESSION_TOKENS

	// Collect and sort oversized sections by token count descending.
	type sectionEntry struct {
		name   string
		tokens int
	}
	var oversized []sectionEntry
	for name, tokens := range sectionSizes {
		if tokens > MAX_SECTION_LENGTH {
			oversized = append(oversized, sectionEntry{name, tokens})
		}
	}
	sort.Slice(oversized, func(i, j int) bool {
		return oversized[i].tokens > oversized[j].tokens
	})

	if len(oversized) == 0 && !overBudget {
		return ""
	}

	var parts []string

	if overBudget {
		parts = append(parts, fmt.Sprintf(
			"\n\nCRITICAL: The session memory file is currently ~%d tokens, which exceeds the maximum of %d tokens. You MUST condense the file to fit within this budget. Aggressively shorten oversized sections by removing less important details, merging related items, and summarizing older entries. Prioritize keeping \"Current State\" and \"Errors & Corrections\" accurate and detailed.",
			totalTokens, MAX_TOTAL_SESSION_TOKENS,
		))
	}

	if len(oversized) > 0 {
		var sectionLines []string
		for _, s := range oversized {
			sectionLines = append(sectionLines, fmt.Sprintf(
				`- "%s" is ~%d tokens (limit: %d)`,
				s.name, s.tokens, MAX_SECTION_LENGTH,
			))
		}

		if overBudget {
			parts = append(parts, "\n\nOversized sections to condense:\n"+strings.Join(sectionLines, "\n"))
		} else {
			parts = append(parts, "\n\nIMPORTANT: The following sections exceed the per-section limit and MUST be condensed:\n"+strings.Join(sectionLines, "\n"))
		}
	}

	return strings.Join(parts, "")
}

// BuildSessionMemoryUpdatePrompt constructs the full update prompt for the
// session memory extraction flow. It loads the default update prompt,
// analyzes section sizes in the current notes, generates condensation
// reminders for oversized sections, and substitutes {{notesPath}} and
// {{currentNotes}} variables into the prompt template.
func BuildSessionMemoryUpdatePrompt(currentNotes, notesPath string) string {
	promptTemplate := getDefaultUpdatePrompt()

	sectionSizes := analyzeSectionSizes(currentNotes)
	totalTokens := len(currentNotes) / 4
	sectionReminders := generateSectionReminders(sectionSizes, totalTokens)

	variables := map[string]string{
		"currentNotes": currentNotes,
		"notesPath":    notesPath,
	}

	basePrompt := substituteVariables(promptTemplate, variables)

	return basePrompt + sectionReminders
}

// TruncateSessionMemoryForCompact truncates session memory sections that
// exceed the per-section token limit, for use when inserting session memory
// into compact messages. Each section's content is capped at
// MAX_SECTION_LENGTH * 4 characters (the inverse of the 4-bytes-per-token
// rough estimate). Truncation occurs at line boundaries to avoid splitting
// mid-line. Returns the truncated content and whether any truncation occurred.
func TruncateSessionMemoryForCompact(content string) (truncatedContent string, wasTruncated bool) {
	lines := strings.Split(content, "\n")
	maxCharsPerSection := MAX_SECTION_LENGTH * 4

	var outputLines []string
	var currentSectionLines []string
	var currentSectionHeader string
	var truncated bool

	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			resultLines, resultTruncated := flushSessionSection(currentSectionHeader, currentSectionLines, maxCharsPerSection)
			outputLines = append(outputLines, resultLines...)
			truncated = truncated || resultTruncated

			currentSectionHeader = line
			currentSectionLines = nil
		} else {
			currentSectionLines = append(currentSectionLines, line)
		}
	}

	// Flush the last section after the loop ends.
	resultLines, resultTruncated := flushSessionSection(currentSectionHeader, currentSectionLines, maxCharsPerSection)
	outputLines = append(outputLines, resultLines...)
	truncated = truncated || resultTruncated

	return strings.Join(outputLines, "\n"), truncated
}

// flushSessionSection processes a single section, returning its lines and
// whether truncation was applied. If the section's content exceeds maxChars,
// it is truncated at the nearest line boundary with a truncation notice
// appended. If sectionHeader is empty (no section was active), the raw lines
// are returned without a header.
func flushSessionSection(sectionHeader string, sectionLines []string, maxChars int) ([]string, bool) {
	if sectionHeader == "" {
		return sectionLines, false
	}

	sectionContent := strings.Join(sectionLines, "\n")
	if len(sectionContent) <= maxChars {
		result := make([]string, 0, 1+len(sectionLines))
		result = append(result, sectionHeader)
		result = append(result, sectionLines...)
		return result, false
	}

	// Truncate at a line boundary near the character limit.
	charCount := 0
	keptLines := []string{sectionHeader}
	for _, line := range sectionLines {
		if charCount+len(line)+1 > maxChars {
			break
		}
		keptLines = append(keptLines, line)
		charCount += len(line) + 1
	}
	keptLines = append(keptLines, "\n[... section truncated for length ...]")
	return keptLines, true
}
