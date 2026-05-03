package teammemsync

import (
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// classifyHTTPErr maps common HTTP client errors to a category string.
func classifyHTTPErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") {
		return "timeout"
	}
	return "network"
}

// exponentialBackoff computes a retry delay with exponential increase and jitter.
// baseDelay of 500ms * 2^attempt, capped at maxDelay of 30s.
func exponentialBackoff(attempt int) time.Duration {
	const baseDelay = 500 * time.Millisecond
	const maxDelay = 30 * time.Second
	const jitterFraction = 0.25

	exp := float64(uint(1) << attempt)
	delay := baseDelay.Seconds() * exp
	jitter := delay * jitterFraction * (2*rand.Float64() - 1)
	delay += jitter
	if delay < baseDelay.Seconds() {
		delay = baseDelay.Seconds()
	}
	if delay > maxDelay.Seconds() {
		delay = maxDelay.Seconds()
	}
	return time.Duration(math.Ceil(delay * float64(time.Second)))
}

// trimQuotes removes surrounding double quotes from a string.
func trimQuotes(s string) string {
	return strings.Trim(s, `"`)
}

// walkTeamDir recursively reads all files from the team memory directory.
// Keys are relative paths from teamDir.
func walkTeamDir(dir, teamDir string, entries map[string]string) error {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range dirEntries {
		fullPath := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if err := walkTeamDir(fullPath, teamDir, entries); err != nil {
				return err
			}
			continue
		}

		if !entry.Type().IsRegular() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.Size() > MaxFileSizeBytes {
			continue
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		relPath, err := filepath.Rel(teamDir, fullPath)
		if err != nil {
			continue
		}
		// Normalize to forward slashes for cross-platform consistency.
		relPath = filepath.ToSlash(relPath)
		entries[relPath] = string(content)
	}

	return nil
}
