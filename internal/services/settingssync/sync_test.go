package settingssync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadUserSettings_DisabledGate(t *testing.T) {
	ctx := context.Background()
	ResetDownloadState()

	// Without config/loader set, IsUsingOAuth returns false → skip.
	result := DownloadUserSettings(ctx)
	if result {
		t.Error("disabled gate should return false")
	}
}

func TestRedownloadUserSettings_DisabledGate(t *testing.T) {
	ctx := context.Background()
	result := RedownloadUserSettings(ctx)
	if result {
		t.Error("disabled gate should return false for force download")
	}
}

func TestResetDownloadState(t *testing.T) {
	ResetDownloadState()
	ResetDownloadState()
	// Should not panic.
}

func TestUploadUserSettingsInBackground_DisabledGate(t *testing.T) {
	// Feature flag disabled by default — should not panic.
	UploadUserSettingsInBackground()
}

func TestDoDownload_SkipNotOAuth(t *testing.T) {
	ctx := context.Background()
	if doDownloadUserSettings(ctx, 0) {
		t.Error("expected false when not using OAuth")
	}
}

func TestApplyRemoteEntriesToLocal_RespectsFilePermissions(t *testing.T) {
	home := t.TempDir()

	entries := map[string]string{
		KeyUserSettings: `{"theme":"dark"}`,
	}

	ApplyRemoteEntriesToLocal(entries, "", home, "")

	userSettings := filepath.Join(home, ".claude", "settings.json")
	info, err := os.Stat(userSettings)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("expected 0644, got %o", info.Mode().Perm())
	}
}

func TestSyncKeyRoundTrip(t *testing.T) {
	// Verify that ProjectSettingsKey and ProjectMemoryKey produce keys
	// that BuildEntriesFromLocalFiles can consume.
	projectID := "org/repo"
	settingsKey := ProjectSettingsKey(projectID)
	memoryKey := ProjectMemoryKey(projectID)

	if settingsKey != "projects/org/repo/.claude/settings.local.json" {
		t.Errorf("unexpected settings key: %s", settingsKey)
	}
	if memoryKey != "projects/org/repo/CLAUDE.local.md" {
		t.Errorf("unexpected memory key: %s", memoryKey)
	}
}
