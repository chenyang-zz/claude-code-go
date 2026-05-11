package vcr

import (
	"os"
	"regexp"
	"strings"
)

// Environment-specific patterns for dehydration.
// These replace machine-local paths with portable placeholders so fixture
// hashes are deterministic across environments.
var (
	numFilesRe     = regexp.MustCompile(`num_files="\d+"`)
	durationMsRe   = regexp.MustCompile(`duration_ms="\d+"`)
	costUSDRe      = regexp.MustCompile(`cost_usd="[0-9.]+"`)
	availableCmdsRe = regexp.MustCompile(`Available commands:.+`)
	uuidRe          = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	timestampRe     = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z?`)
	nonAlphaNumRe   = regexp.MustCompile(`[^a-zA-Z0-9]`)
)

// claudeConfigHome returns the Claude config home directory path, typically ~/.claude.
func claudeConfigHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/.claude"
	}
	return home + "/.claude"
}

// cwd returns the current working directory.
func cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "/tmp"
	}
	return dir
}

// cwdSlug returns the cwd as a slug (non-alphanumeric chars replaced with "-").
func cwdSlug() string {
	return nonAlphaNumRe.ReplaceAllString(cwd(), "-")
}

// DehydrateValue replaces environment-specific paths and values with
// portable placeholders so fixture hashes are deterministic across machines.
func DehydrateValue(s string) string {
	configHome := claudeConfigHome()
	cwdPath := cwd()

	s1 := numFilesRe.ReplaceAllString(s, `num_files="[NUM]"`)
	s1 = durationMsRe.ReplaceAllString(s1, `duration_ms="[DURATION]"`)
	s1 = costUSDRe.ReplaceAllString(s1, `cost_usd="[COST]"`)
	s1 = strings.ReplaceAll(s1, configHome, "[CONFIG_HOME]")
	s1 = strings.ReplaceAll(s1, cwdPath, "[CWD]")
	s1 = availableCmdsRe.ReplaceAllString(s1, "Available commands: [COMMANDS]")

	if strings.Contains(s1, "Files modified by user:") {
		s1 = "Files modified by user: [FILES]"
	}

	return s1
}

// HydrateValue restores portable placeholders back to the real
// environment-specific values for test replay.
func HydrateValue(s string) string {
	s1 := strings.ReplaceAll(s, "[NUM]", "1")
	s1 = strings.ReplaceAll(s1, "[DURATION]", "100")
	s1 = strings.ReplaceAll(s1, "[CONFIG_HOME]", claudeConfigHome())
	s1 = strings.ReplaceAll(s1, "[CWD]", cwd())
	return s1
}

// DehydrateTokenCountInput performs extra normalization for token-count
// fixtures: replaces CWD slugs, UUIDs, and timestamps with placeholders.
func DehydrateTokenCountInput(input string) string {
	s := DehydrateValue(input)
	s = strings.ReplaceAll(s, cwdSlug(), "[CWD_SLUG]")
	s = uuidRe.ReplaceAllString(s, "[UUID]")
	s = timestampRe.ReplaceAllString(s, "[TIMESTAMP]")
	return s
}
