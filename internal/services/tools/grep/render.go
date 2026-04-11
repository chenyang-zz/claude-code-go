package grep

import (
	"fmt"
	"strings"
)

// effectiveContext picks the symmetric content-context flag, preferring the long-form input.
func effectiveContext(context *int, contextAlias *int) *int {
	if context != nil {
		return context
	}
	return contextAlias
}

// normalizeShowLineNumbers keeps content mode aligned with the source default of showing line numbers.
func normalizeShowLineNumbers(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}

// splitGlobPatterns expands a comma-or-space-separated glob string into ripgrep --glob arguments.
func splitGlobPatterns(glob string) []string {
	trimmed := strings.TrimSpace(glob)
	if trimmed == "" {
		return nil
	}

	rawPatterns := strings.Fields(trimmed)
	patterns := make([]string, 0, len(rawPatterns))
	for _, rawPattern := range rawPatterns {
		if strings.Contains(rawPattern, "{") && strings.Contains(rawPattern, "}") {
			patterns = append(patterns, rawPattern)
			continue
		}
		for _, part := range strings.Split(rawPattern, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				patterns = append(patterns, part)
			}
		}
	}
	return patterns
}

// applyHeadLimit paginates rows or matches and reports whether truncation metadata should be surfaced.
func applyHeadLimit[T any](items []T, limit *int, offset int) ([]T, *int, *int) {
	safeOffset := clampOffset(offset, len(items))
	if limit != nil && *limit == 0 {
		return items[safeOffset:], nil, offsetPointer(safeOffset)
	}

	effectiveLimit := defaultHeadLimit
	if limit != nil {
		effectiveLimit = clampNonNegative(*limit)
	}

	end := safeOffset + effectiveLimit
	if end > len(items) {
		end = len(items)
	}

	var appliedLimit *int
	if len(items)-safeOffset > effectiveLimit {
		appliedLimit = intPointer(effectiveLimit)
	}

	return items[safeOffset:end], appliedLimit, offsetPointer(safeOffset)
}

// clampNonNegative normalizes user-provided paging values to non-negative integers.
func clampNonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

// clampOffset keeps pagination offsets within the result bounds.
func clampOffset(offset int, itemCount int) int {
	normalized := clampNonNegative(offset)
	if normalized > itemCount {
		return itemCount
	}
	return normalized
}

// intPointer allocates one stable integer pointer for optional output fields.
func intPointer(value int) *int {
	return &value
}

// offsetPointer omits zero offsets from the structured pagination metadata.
func offsetPointer(offset int) *int {
	if offset <= 0 {
		return nil
	}
	return intPointer(offset)
}

// derefInt turns optional integers into log-friendly values.
func derefInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

// normalizeOutputMode keeps unknown modes aligned with the default files-with-matches path.
func normalizeOutputMode(outputMode string) string {
	switch strings.TrimSpace(outputMode) {
	case outputModeContent:
		return outputModeContent
	case outputModeCount:
		return outputModeCount
	default:
		return outputModeFilesWithMatches
	}
}

// formatLimitInfo renders the human-facing pagination suffix shown in tool output.
func formatLimitInfo(appliedLimit *int, appliedOffset *int) string {
	parts := make([]string, 0, 2)
	if appliedLimit != nil {
		parts = append(parts, fmt.Sprintf("limit: %d", *appliedLimit))
	}
	if appliedOffset != nil && *appliedOffset > 0 {
		parts = append(parts, fmt.Sprintf("offset: %d", *appliedOffset))
	}
	return strings.Join(parts, ", ")
}

// renderOutput formats the caller-facing result body for the current migration pass.
func renderOutput(output Output) string {
	switch output.Mode {
	case outputModeContent:
		if strings.TrimSpace(output.Content) == "" {
			return "No matches found"
		}
		if output.PaginationSummary == "" {
			return output.Content
		}
		return output.Content + "\n\n[" + output.PaginationSummary + "]"
	case outputModeCount:
		if strings.TrimSpace(output.Content) == "" {
			return "No matches found"
		}
		limitInfo := formatLimitInfo(output.AppliedLimit, output.AppliedOffset)
		summary := fmt.Sprintf(
			"\n\nFound %d total %s across %d %s%s.",
			output.NumMatches,
			pluralize(output.NumMatches, "occurrence", "occurrences"),
			output.NumFiles,
			pluralize(output.NumFiles, "file", "files"),
			formatPaginationSuffix(limitInfo),
		)
		if output.PaginationSummary != "" {
			summary += "\n" + output.PaginationSummary + "."
		}
		return output.Content + summary
	default:
		if output.NumFiles == 0 {
			return "No files found"
		}
		if output.PaginationSummary == "" {
			return strings.Join(output.Filenames, "\n")
		}
		return fmt.Sprintf(
			"Found %d %s\n[%s]\n%s",
			output.NumFiles,
			pluralize(output.NumFiles, "file", "files"),
			output.PaginationSummary,
			strings.Join(output.Filenames, "\n"),
		)
	}
}

// formatPaginationSuffix appends pagination details to summary text only when needed.
func formatPaginationSuffix(limitInfo string) string {
	if limitInfo == "" {
		return ""
	}
	return " with pagination = " + limitInfo
}

// buildPaginationSummary converts limit/offset metadata into a caller-facing "showing X-Y of Z" sentence.
func buildPaginationSummary(total int, returned int, appliedLimit *int, appliedOffset *int, noun string) string {
	if total == 0 || returned == 0 {
		return ""
	}
	if appliedLimit == nil && appliedOffset == nil {
		return ""
	}

	start := 1
	if appliedOffset != nil && *appliedOffset > 0 {
		start += *appliedOffset
	}
	end := start + returned - 1
	limitInfo := formatLimitInfo(appliedLimit, appliedOffset)
	if limitInfo == "" {
		return fmt.Sprintf("Showing %s %d-%d of %d", noun, start, end, total)
	}
	return fmt.Sprintf("Showing %s %d-%d of %d with pagination = %s", noun, start, end, total, limitInfo)
}

// inputPathOrWorkingDir keeps handled validation errors aligned with the user-provided path.
func inputPathOrWorkingDir(inputPath string, expandedPath string) string {
	if strings.TrimSpace(inputPath) == "" {
		return expandedPath
	}
	return inputPath
}

// pluralize chooses a singular or plural label for small caller-facing summaries.
func pluralize(count int, singular string, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
