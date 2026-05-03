package settingssync

import (
	"context"
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// pickDiff returns entries in local that differ from (or are absent in) remote.
func pickDiff(local, remote map[string]string) map[string]string {
	diff := make(map[string]string)
	for k, v := range local {
		if remote[k] != v {
			diff[k] = v
		}
	}
	return diff
}

// UploadUserSettingsInBackground performs an incremental settings sync upload
// as a fire-and-forget goroutine. It fetches the current remote entries,
// compares them with local files, and uploads only the changed entries.
//
// The function silently skips (fail-open) when:
//   - the feature flag is disabled
//   - the user is not using first-party OAuth
//   - the session is not interactive
//   - the fetch or upload fails
func UploadUserSettingsInBackground() {
	if !IsSettingsSyncPushEnabled() {
		logger.DebugCF("settingssync", "upload skipped: feature flag disabled", nil)
		return
	}
	if !IsUsingOAuth() {
		logger.DebugCF("settingssync", "upload skipped: not using OAuth", nil)
		return
	}

	c := getConfig()
	if c == nil {
		return
	}

	accessToken := AccessToken()
	baseURL := OAuthBaseURL()
	configHome := c.HomeDir
	cwd := c.ProjectPath

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.WarnCF("settingssync", "upload panicked", map[string]any{"panic": r})
			}
		}()

		logger.DebugCF("settingssync", "upload starting", nil)

		// 1. Fetch current remote entries.
		result, err := FetchUserSettings(context.Background(), baseURL, accessToken, defaultMaxRetries)
		if err != nil || !result.Success {
			logger.WarnCF("settingssync", "upload fetch failed", map[string]any{
				"success": result != nil && result.Success,
			})
			return
		}

		// 2. Build local entries.
		projectID := DeriveProjectID(cwd)
		localEntries := BuildEntriesFromLocalFiles(cwd, configHome, projectID)

		// 3. Diff.
		remoteEntries := map[string]string{}
		if result.Data != nil {
			remoteEntries = result.Data.Content.Entries
		}
		changedEntries := pickDiff(localEntries, remoteEntries)

		if len(changedEntries) == 0 {
			logger.DebugCF("settingssync", "upload skipped: no changes", nil)
			return
		}

		// 4. Upload changed entries.
		uploadResult, err := UploadUserSettings(context.Background(), baseURL, accessToken, changedEntries)
		if err != nil || !uploadResult.Success {
			logger.WarnCF("settingssync", "upload failed", map[string]any{
				"entry_count": len(changedEntries),
			})
			return
		}

		logger.DebugCF("settingssync", "upload success", map[string]any{
			"entry_count": len(changedEntries),
		})
	}()
}

// downloadPromise caches the first download call so that multiple callers
// (e.g. fire-and-forget + await in plugin install) share one request.
var (
	downloadOnce sync.Once
	downloadRes  bool
)

// DownloadUserSettings triggers a one-shot remote settings download that
// applies the fetched entries locally. Multiple concurrent callers share the
// same in-flight request and see the same result.
// Returns true if remote settings were successfully applied.
func DownloadUserSettings(ctx context.Context) bool {
	downloadOnce.Do(func() {
		downloadRes = doDownloadUserSettings(ctx, defaultMaxRetries)
	})
	return downloadRes
}

// RedownloadUserSettings forces a fresh download with no retries (0 retries),
// bypassing any cached result. Called by /reload-plugins so that mid-session
// settings changes pushed from the user's other device take effect
// before the plugin-cache sweep.
// Returns true if remote settings were successfully applied.
func RedownloadUserSettings(ctx context.Context) bool {
	return doDownloadUserSettings(ctx, 0)
}

// doDownloadUserSettings performs the actual download→apply pipeline.
func doDownloadUserSettings(ctx context.Context, maxRetries int) bool {
	if !IsSettingsSyncPullEnabled() {
		logger.DebugCF("settingssync", "download skipped: feature flag disabled", nil)
		return false
	}
	if !IsUsingOAuth() {
		logger.DebugCF("settingssync", "download skipped: not using OAuth", nil)
		return false
	}

	c := getConfig()
	if c == nil {
		return false
	}

	accessToken := AccessToken()
	baseURL := OAuthBaseURL()
	configHome := c.HomeDir
	cwd := c.ProjectPath

	logger.DebugCF("settingssync", "download starting", nil)

	result, err := FetchUserSettings(ctx, baseURL, accessToken, maxRetries)
	if err != nil || !result.Success {
		logger.WarnCF("settingssync", "download fetch failed", map[string]any{
			"success": result != nil && result.Success,
		})
		return false
	}

	if result.IsEmpty {
		logger.DebugCF("settingssync", "download skipped: remote empty", nil)
		return false
	}

	entries := result.Data.Content.Entries
	projectID := DeriveProjectID(cwd)

	entryCount := len(entries)
	logger.DebugCF("settingssync", "download applying", map[string]any{
		"entry_count": entryCount,
	})

	ApplyRemoteEntriesToLocal(entries, cwd, configHome, projectID)

	logger.DebugCF("settingssync", "download success", map[string]any{
		"entry_count": entryCount,
	})
	return true
}

// ResetDownloadState clears the cached download result so the next call to
// DownloadUserSettings starts a fresh fetch. Useful for testing.
func ResetDownloadState() {
	downloadOnce = sync.Once{}
	downloadRes = false
}

