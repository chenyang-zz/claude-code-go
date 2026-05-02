package file_read

import (
	"testing"
	"time"

	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

func TestFreshnessTrackerRecordAndGet(t *testing.T) {
	ft := NewFreshnessTracker()
	modTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	ft.RecordRead("/tmp/test.go", modTime)
	state, ok := ft.GetState("/tmp/test.go")
	if !ok {
		t.Fatal("expected state to exist")
	}
	if !state.ObservedModTime.Equal(modTime) {
		t.Fatalf("expected modTime %v, got %v", modTime, state.ObservedModTime)
	}
	if state.HasChangedExternally {
		t.Fatal("expected HasChangedExternally to be false")
	}
}

func TestFreshnessTrackerMarkChanged(t *testing.T) {
	ft := NewFreshnessTracker()
	modTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	ft.RecordRead("/tmp/test.go", modTime)
	ft.MarkChanged("/tmp/test.go")

	state, ok := ft.GetState("/tmp/test.go")
	if !ok {
		t.Fatal("expected state to exist")
	}
	if !state.HasChangedExternally {
		t.Fatal("expected HasChangedExternally to be true")
	}
}

func TestFreshnessTrackerHasChanged(t *testing.T) {
	ft := NewFreshnessTracker()
	modTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// Unknown file should not report changed.
	if ft.HasChanged("/tmp/unknown.go", modTime) {
		t.Fatal("unknown file should not report changed")
	}

	// File recorded, same mtime should not report changed.
	ft.RecordRead("/tmp/test.go", modTime)
	if ft.HasChanged("/tmp/test.go", modTime) {
		t.Fatal("same mtime should not report changed")
	}

	// Different mtime should report changed.
	newModTime := modTime.Add(time.Hour)
	if !ft.HasChanged("/tmp/test.go", newModTime) {
		t.Fatal("different mtime should report changed")
	}

	// External change flag should report changed.
	ft.RecordRead("/tmp/test.go", modTime)
	ft.MarkChanged("/tmp/test.go")
	if !ft.HasChanged("/tmp/test.go", modTime) {
		t.Fatal("external change flag should report changed even with same mtime")
	}
}

func TestFreshnessTrackerBuildReminder(t *testing.T) {
	ft := NewFreshnessTracker()
	modTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// Unknown file should return empty.
	if ft.BuildReminder("/tmp/unknown.go", modTime) != "" {
		t.Fatal("unknown file should return empty reminder")
	}

	// Same mtime should return empty.
	ft.RecordRead("/tmp/test.go", modTime)
	if ft.BuildReminder("/tmp/test.go", modTime) != "" {
		t.Fatal("same mtime should return empty reminder")
	}

	// External change should return reminder.
	ft.MarkChanged("/tmp/test.go")
	reminder := ft.BuildReminder("/tmp/test.go", modTime)
	if reminder == "" {
		t.Fatal("external change should return non-empty reminder")
	}
	if !contains(reminder, "changed externally") {
		t.Fatalf("reminder should mention external change, got: %s", reminder)
	}

	// Different mtime should return reminder with time delta.
	ft.RecordRead("/tmp/test.go", modTime)
	newModTime := modTime.Add(2 * time.Hour)
	reminder = ft.BuildReminder("/tmp/test.go", newModTime)
	if reminder == "" {
		t.Fatal("different mtime should return non-empty reminder")
	}
	if !contains(reminder, "changed") {
		t.Fatalf("reminder should mention change, got: %s", reminder)
	}
}

func TestFormatTimeDelta(t *testing.T) {
	tests := []struct {
		delta    time.Duration
		expected string
	}{
		{time.Second, "changed just now"},
		{30 * time.Second, "changed just now"},
		{time.Minute, "changed 1 minute ago"},
		{5 * time.Minute, "changed 5 minutes ago"},
		{time.Hour, "changed 1 hour ago"},
		{3 * time.Hour, "changed 3 hours ago"},
		{24 * time.Hour, "changed 1 day ago"},
		{3 * 24 * time.Hour, "changed 3 days ago"},
	}

	for _, tc := range tests {
		result := formatTimeDelta(tc.delta)
		if result != tc.expected {
			t.Fatalf("formatTimeDelta(%v) = %q, want %q", tc.delta, result, tc.expected)
		}
	}
}

func TestGetAlternateScreenshotPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/Users/foo/Desktop/Screen Shot 2024-01-01 at 10.00.00 AM.png", "/Users/foo/Desktop/Screen Shot 2024-01-01 at 10.00.00 AM.png"},
		{"/Users/foo/Desktop/Screen Shot 2024-01-01 at 10.00.00 AM.png", "/Users/foo/Desktop/Screen Shot 2024-01-01 at 10.00.00 AM.png"},
		{"/Users/foo/Desktop/regular file.png", ""},
		{"/Users/foo/Desktop/Screen Shot 2024-01-01 at 10.00.00 AM.jpg", ""},
		{"/Users/foo/Desktop/no-am-pm.png", ""},
	}

	for _, tc := range tests {
		result := getAlternateScreenshotPath(tc.input)
		if result != tc.expected {
			t.Fatalf("getAlternateScreenshotPath(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestIsScreenshotPath(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"/Users/foo/Desktop/Screen Shot 2024-01-01 at 10.00.00 AM.png", true},
		{"/Users/foo/Desktop/Screen Shot 2024-01-01 at 10.00.00 AM.png", true},
		{"/Users/foo/Desktop/regular file.png", false},
		{"/Users/foo/Desktop/Screen Shot 2024-01-01 at 10.00.00 AM.jpg", false},
	}

	for _, tc := range tests {
		result := isScreenshotPath(tc.input)
		if result != tc.expected {
			t.Fatalf("isScreenshotPath(%q) = %v, want %v", tc.input, result, tc.expected)
		}
	}
}

func TestResolveScreenshotPath(t *testing.T) {
	fs := platformfs.NewLocalFS()

	// Non-screenshot path should return as-is.
	path, ok := resolveScreenshotPath(fs, "/tmp/regular.txt")
	if path != "/tmp/regular.txt" || !ok {
		t.Fatalf("non-screenshot should return as-is, got %q, ok=%v", path, ok)
	}
}

func TestBuildFreshnessPrefix(t *testing.T) {
	tracker := NewFreshnessTracker()
	modTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// No reminders should return empty.
	if buildFreshnessPrefix(tracker, "/tmp/test.go", modTime) != "" {
		t.Fatal("no reminders should return empty")
	}

	// Memory file reminder (mock by using a known memory path if possible).
	// For this test, we just verify the function does not panic.
	_ = buildFreshnessPrefix(tracker, "/tmp/test.go", modTime)

	// External change reminder.
	tracker.RecordRead("/tmp/test.go", modTime)
	tracker.MarkChanged("/tmp/test.go")
	prefix := buildFreshnessPrefix(tracker, "/tmp/test.go", modTime)
	if prefix == "" {
		t.Fatal("should have freshness prefix after external change")
	}
	if !contains(prefix, "changed externally") {
		t.Fatalf("prefix should mention external change, got: %s", prefix)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && substr != "" && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
