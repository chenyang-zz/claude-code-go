package teammemsync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// pushTimeout is the HTTP client timeout for push requests.
	pushTimeout = 30 * time.Second
)

// uploadTeamMemoryOnce performs a single PUT request to upload team memory
// entries. Supports If-Match for optimistic locking.
func uploadTeamMemoryOnce(
	ctx context.Context,
	baseURL string,
	repoSlug string,
	accessToken string,
	entries map[string]string,
	ifMatchChecksum string,
) *UploadResult {
	url := fmt.Sprintf("%s/api/claude_code/team_memory?repo=%s", baseURL, repoSlug)

	bodyBytes, err := json.Marshal(map[string]interface{}{
		"entries": entries,
	})
	if err != nil {
		return &UploadResult{
			Success:   false,
			Error:     fmt.Sprintf("failed to marshal upload body: %v", err),
			ErrorType: "unknown",
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return &UploadResult{Success: false, Error: err.Error(), ErrorType: "unknown"}
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", oauth.OAuthBetaHeader)
	req.Header.Set("Content-Type", "application/json")
	if ifMatchChecksum != "" {
		req.Header.Set("If-Match", fmt.Sprintf(`"%s"`, trimQuotes(ifMatchChecksum)))
	}

	client := &http.Client{Timeout: pushTimeout}
	resp, err := client.Do(req)
	if err != nil {
		logger.WarnCF("teammemsync", "upload request failed", map[string]any{
			"error": err.Error(),
		})
		return &UploadResult{
			Success:   false,
			Error:     err.Error(),
			ErrorType: "network",
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))

	// 412 Precondition Failed — ETag mismatch (conflict).
	if resp.StatusCode == http.StatusPreconditionFailed {
		logger.InfoCF("teammemsync", "upload conflict (412 Precondition Failed)", nil)
		return &UploadResult{Success: false, Conflict: true, Error: "ETag mismatch"}
	}

	// Parse structured 413 for too-many-entries.
	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		var tooMany TeamMemoryTooManyEntries
		if json.Unmarshal(body, &tooMany) == nil &&
			tooMany.Error.Details.ErrorCode == "team_memory_too_many_entries" {
			return &UploadResult{
				Success:              false,
				Error:                fmt.Sprintf("HTTP 413: too many entries (max %d, received %d)", tooMany.Error.Details.MaxEntries, tooMany.Error.Details.ReceivedEntries),
				ErrorType:            "unknown",
				HTTPStatus:           resp.StatusCode,
				ServerErrorCode:      tooMany.Error.Details.ErrorCode,
				ServerMaxEntries:     tooMany.Error.Details.MaxEntries,
				ServerReceivedEntries: tooMany.Error.Details.ReceivedEntries,
			}
		}
		return &UploadResult{
			Success:    false,
			Error:      fmt.Sprintf("HTTP 413: %s", string(body)),
			ErrorType:  "unknown",
			HTTPStatus: resp.StatusCode,
		}
	}

	// Non-200 response.
	if resp.StatusCode != http.StatusOK {
		return &UploadResult{
			Success:    false,
			Error:      fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
			ErrorType:  "unknown",
			HTTPStatus: resp.StatusCode,
		}
	}

	var uploadResp struct {
		Checksum     string `json:"checksum"`
		LastModified string `json:"lastModified"`
	}
	if err := json.Unmarshal(body, &uploadResp); err != nil {
		// Non-fatal: upload succeeded even if response parse fails.
		logger.DebugCF("teammemsync", "upload succeeded but response parse failed", map[string]any{
			"error": err.Error(),
		})
		return &UploadResult{Success: true}
	}

	logger.DebugCF("teammemsync", "upload succeeded", map[string]any{
		"entry_cnt":    len(entries),
		"lastModified": uploadResp.LastModified,
	})
	return &UploadResult{
		Success:      true,
		Checksum:     uploadResp.Checksum,
		LastModified: uploadResp.LastModified,
	}
}

// PushTeamMemory pushes local team memory files to the server with optimistic
// locking. Uses delta upload: only keys whose local content hash differs from
// serverChecksums are uploaded. On 412 conflict, probes GET ?view=hashes,
// recomputes the delta, and retries.
func PushTeamMemory(
	ctx context.Context,
	state *SyncState,
	baseURL string,
	repoSlug string,
	accessToken string,
	projectRoot string,
) *PushResult {
	// Read local entries once.
	entries, err := ReadLocalTeamMemory(projectRoot, state.ServerMaxEntries)
	if err != nil {
		return &PushResult{
			Success:     false,
			Error:       fmt.Sprintf("failed to read local team memory: %v", err),
			ErrorType:   "unknown",
		}
	}

	if len(entries) == 0 {
		return &PushResult{Success: true, FilesUploaded: 0}
	}

	// Hash each local entry once. Reused across conflict retries.
	localHashes := make(map[string]string, len(entries))
	for key, content := range entries {
		localHashes[key] = HashContent(content)
	}

	for conflictAttempt := 0; conflictAttempt <= MaxConflictRetries; conflictAttempt++ {
		// Compute delta: only keys whose local hash differs from what we believe
		// the server holds.
		delta := make(map[string]string)
		for key, localHash := range localHashes {
			if serverHash, ok := state.ServerChecksums[key]; !ok || serverHash != localHash {
				delta[key] = entries[key]
			}
		}

		if len(delta) == 0 {
			// Nothing to upload. Fast path after fresh pull with no local edits.
			return &PushResult{Success: true, FilesUploaded: 0}
		}

		// Split into PUT-sized batches.
		batches := BatchDeltaByBytes(delta)
		var filesUploaded int
		var lastResult *UploadResult

		for _, batch := range batches {
			result := uploadTeamMemoryOnce(
				ctx, baseURL, repoSlug, accessToken, batch, state.LastKnownChecksum,
			)
			lastResult = result
			if !result.Success {
				break
			}

			// Update serverChecksums for committed keys.
			for key := range batch {
				state.ServerChecksums[key] = localHashes[key]
			}
			filesUploaded += len(batch)

			// Update ETag chain for the next batch.
			if result.Checksum != "" {
				state.LastKnownChecksum = result.Checksum
			}
		}

		if lastResult == nil {
			return &PushResult{Success: true, FilesUploaded: 0}
		}

		if lastResult.Success {
			logger.InfoCF("teammemsync", "push completed", map[string]any{
				"files_uploaded": filesUploaded,
				"total_entries":  len(entries),
			})
			return &PushResult{
				Success:       true,
				FilesUploaded: filesUploaded,
				Checksum:      lastResult.Checksum,
			}
		}

		if !lastResult.Conflict {
			// Non-conflict failure — cache serverMaxEntries from 413 if present.
			if lastResult.ServerMaxEntries > 0 {
				max := lastResult.ServerMaxEntries
				state.ServerMaxEntries = &max
				logger.WarnCF("teammemsync", "learned server max_entries from 413", map[string]any{
					"max_entries": max,
				})
			}
			return &PushResult{
				Success:       false,
				FilesUploaded: filesUploaded,
				Error:         lastResult.Error,
				ErrorType:     lastResult.ErrorType,
				HTTPStatus:    lastResult.HTTPStatus,
			}
		}

		// 412 conflict — refresh serverChecksums and retry.
		if conflictAttempt >= MaxConflictRetries {
			logger.WarnCF("teammemsync", "giving up after max conflict retries", nil)
			return &PushResult{
				Success:     false,
				Conflict:    true,
				Error:       "Conflict resolution failed after retries",
				ErrorType:   "conflict",
			}
		}

		logger.InfoCF("teammemsync", "conflict (412), probing server hashes", map[string]any{
			"attempt": conflictAttempt + 1,
			"max":     MaxConflictRetries,
		})

		probe := fetchTeamMemoryHashes(ctx, baseURL, repoSlug, accessToken)
		if !probe.Success || probe.EntryChecksums == nil {
			return &PushResult{
				Success:     false,
				Conflict:    true,
				Error:       fmt.Sprintf("Conflict resolution hashes probe failed: %s", probe.Error),
				ErrorType:   "conflict",
			}
		}

		state.ServerChecksums = make(map[string]string)
		for k, v := range probe.EntryChecksums {
			state.ServerChecksums[k] = v
		}
		if probe.Checksum != "" {
			state.LastKnownChecksum = probe.Checksum
		}
	}

	return &PushResult{
		Success:   false,
		Error:     "Unexpected end of conflict resolution loop",
		ErrorType: "conflict",
	}
}

