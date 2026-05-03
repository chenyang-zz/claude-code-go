package teammemsync

import (
	"context"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// IsTeamMemorySyncAvailable checks whether team memory sync can be performed.
// Requires IsTeamMemoryEnabled (auto memory + feature flag) and a non-empty
// repo slug. Callers should also validate OAuth tokens separately.
func IsTeamMemorySyncAvailable(repoSlug string) bool {
	if !IsTeamMemoryEnabled() {
		return false
	}
	return repoSlug != ""
}

// SyncTeamMemory performs a bidirectional sync: pull from server, then push
// local changes back. Server entries take precedence on conflict. Push uses
// delta upload with optimistic locking and 412 conflict resolution.
func SyncTeamMemory(
	ctx context.Context,
	state *SyncState,
	baseURL string,
	repoSlug string,
	accessToken string,
	projectRoot string,
) (success bool, filesPulled int, filesPushed int, errStr string) {
	// 1. Pull remote to local (skip ETag cache for full sync).
	ok, pulled, _, _, pullErr := PullTeamMemory(ctx, state, baseURL, repoSlug, accessToken, projectRoot, true)
	if !ok {
		return false, pulled, 0, pullErr
	}

	// 2. Push local to remote (with conflict resolution).
	pushResult := PushTeamMemory(ctx, state, baseURL, repoSlug, accessToken, projectRoot)
	if !pushResult.Success {
		return false, pulled, pushResult.FilesUploaded, pushResult.Error
	}

	logger.InfoCF("teammemsync", "sync completed", map[string]any{
		"files_pulled": pulled,
		"files_pushed": pushResult.FilesUploaded,
	})
	return true, pulled, pushResult.FilesUploaded, ""
}
