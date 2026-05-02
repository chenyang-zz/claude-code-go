package file_read

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

// FileFreshnessState tracks the observed state of a file for change-detection.
type FileFreshnessState struct {
	// ObservedModTime records the modification time seen at the last successful read.
	ObservedModTime time.Time
	// HasChangedExternally is set to true when an fsnotify event indicates the file changed.
	HasChangedExternally bool
}

// FreshnessTracker manages per-file freshness state for a single FileReadTool instance.
type FreshnessTracker struct {
	mu     sync.RWMutex
	states map[string]FileFreshnessState
}

// NewFreshnessTracker creates a new tracker with an empty state map.
func NewFreshnessTracker() *FreshnessTracker {
	return &FreshnessTracker{
		states: make(map[string]FileFreshnessState),
	}
}

// RecordRead stores the modification time observed during a successful read.
func (ft *FreshnessTracker) RecordRead(filePath string, modTime time.Time) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	ft.states[filePath] = FileFreshnessState{
		ObservedModTime:      modTime,
		HasChangedExternally: false,
	}
}

// MarkChanged sets the external-change flag for a file without updating the mod time.
func (ft *FreshnessTracker) MarkChanged(filePath string) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	state, ok := ft.states[filePath]
	if !ok {
		state = FileFreshnessState{}
	}
	state.HasChangedExternally = true
	ft.states[filePath] = state
}

// GetState returns the current freshness state for a file and whether it exists.
func (ft *FreshnessTracker) GetState(filePath string) (FileFreshnessState, bool) {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	state, ok := ft.states[filePath]
	return state, ok
}

// HasChanged reports whether the file has changed since the last recorded read.
func (ft *FreshnessTracker) HasChanged(filePath string, currentModTime time.Time) bool {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	state, ok := ft.states[filePath]
	if !ok {
		return false
	}
	if state.HasChangedExternally {
		return true
	}
	if !state.ObservedModTime.IsZero() && !state.ObservedModTime.Equal(currentModTime) {
		return true
	}
	return false
}

// BuildReminder generates a human-readable freshness reminder when a file has changed.
func (ft *FreshnessTracker) BuildReminder(filePath string, currentModTime time.Time) string {
	ft.mu.RLock()
	state, ok := ft.states[filePath]
	ft.mu.RUnlock()
	if !ok {
		return ""
	}

	var reason string
	if state.HasChangedExternally {
		reason = "changed externally"
	} else if !state.ObservedModTime.IsZero() && !state.ObservedModTime.Equal(currentModTime) {
		diff := currentModTime.Sub(state.ObservedModTime)
		reason = formatTimeDelta(diff)
	} else {
		return ""
	}

	return fmt.Sprintf(
		"<system-reminder>File has %s since last read. The content below may differ from what you saw earlier.</system-reminder>\n",
		reason,
	)
}

// formatTimeDelta converts a duration into a human-readable relative time string.
func formatTimeDelta(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "changed just now"
	case d < time.Hour:
		minutes := int(d.Minutes())
		if minutes == 1 {
			return "changed 1 minute ago"
		}
		return fmt.Sprintf("changed %d minutes ago", minutes)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "changed 1 hour ago"
		}
		return fmt.Sprintf("changed %d hours ago", hours)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "changed 1 day ago"
		}
		return fmt.Sprintf("changed %d days ago", days)
	}
}

// thinSpace is the Unicode narrow no-break space (U+202F) used by some macOS versions.
const thinSpace = ' '

// screenshotFilenamePattern matches macOS screenshot filenames with AM/PM timestamps.
// The space before AM/PM may be a regular space or a thin space.
var screenshotFilenamePattern = regexp.MustCompile(`^(.+)([ ` + string(thinSpace) + `])(AM|PM)(\.png)$`)

// getAlternateScreenshotPath returns an alternate path with the other space character
// for macOS screenshot filenames. If the path does not match the screenshot pattern,
// it returns an empty string.
func getAlternateScreenshotPath(filePath string) string {
	filename := filepath.Base(filePath)
	match := screenshotFilenamePattern.FindStringSubmatch(filename)
	if match == nil {
		return ""
	}

	currentSpace := match[2]
	alternateSpace := " "
	if currentSpace == " " {
		alternateSpace = string(thinSpace)
	}

	dir := filepath.Dir(filePath)
	newFilename := match[1] + alternateSpace + match[3] + match[4]
	return filepath.Join(dir, newFilename)
}

// isScreenshotPath reports whether a path looks like a macOS screenshot file.
func isScreenshotPath(filePath string) bool {
	filename := filepath.Base(filePath)
	return screenshotFilenamePattern.MatchString(filename)
}

// resolveScreenshotPath attempts to resolve a macOS screenshot path by trying
// the alternate space character if the original does not exist.
// It returns the resolved path and a bool indicating whether resolution succeeded.
func resolveScreenshotPath(fs platformfs.FileSystem, filePath string) (string, bool) {
	if !isScreenshotPath(filePath) {
		return filePath, true
	}

	// Check if the original path exists
	if _, err := fs.Stat(filePath); err == nil {
		return filePath, true
	}

	// Try the alternate path
	altPath := getAlternateScreenshotPath(filePath)
	if altPath == "" {
		return filePath, false
	}

	if _, err := fs.Stat(altPath); err == nil {
		return altPath, true
	}

	return filePath, false
}
