package teammemsync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// pullTimeout is the HTTP client timeout for pull requests.
	pullTimeout = 30 * time.Second
)

// AccessToken returns the current OAuth access token or empty string.
// Passed via the SubscriptionLoader interface.
type TokenProvider interface {
	AccessToken() string
	IsUsingOAuth() bool
}

// fetchTeamMemoryOnce performs a single GET request to fetch team memory data
// from the backend. Handles ETag-based conditional requests (If-None-Match).
func fetchTeamMemoryOnce(
	ctx context.Context,
	baseURL string,
	repoSlug string,
	accessToken string,
	etag string,
) *FetchResult {
	url := fmt.Sprintf("%s/api/claude_code/team_memory?repo=%s", baseURL, repoSlug)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return &FetchResult{Success: false, Error: err.Error(), ErrorType: "unknown"}
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", oauth.OAuthBetaHeader)
	if etag != "" {
		req.Header.Set("If-None-Match", fmt.Sprintf(`"%s"`, strings.Trim(etag, `"`)))
	}

	client := &http.Client{Timeout: pullTimeout}
	resp, err := client.Do(req)
	if err != nil {
		kind := classifyHTTPErr(err)
		if kind == "auth" {
			return &FetchResult{
				Success:   false,
				Error:     "Not authorized for team memory sync",
				SkipRetry: true,
				ErrorType: "auth",
			}
		}
		return &FetchResult{
			Success:   false,
			Error:     "Cannot connect to server",
			ErrorType: "network",
		}
	}
	defer resp.Body.Close()

	// 304 Not Modified.
	if resp.StatusCode == http.StatusNotModified {
		logger.DebugCF("teammemsync", "fetch not modified (304)", nil)
		return &FetchResult{Success: true, NotModified: true, Checksum: etag}
	}

	// 404 No data exists.
	if resp.StatusCode == http.StatusNotFound {
		logger.DebugCF("teammemsync", "fetch returned 404 - no remote data", nil)
		return &FetchResult{Success: true, IsEmpty: true}
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MB limit

	// Auth failure.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &FetchResult{
			Success:   false,
			Error:     fmt.Sprintf("Not authorized for team memory sync (HTTP %d)", resp.StatusCode),
			SkipRetry: true,
			ErrorType: "auth",
			HTTPStatus: resp.StatusCode,
		}
	}

	// Unexpected status.
	if resp.StatusCode != http.StatusOK {
		return &FetchResult{
			Success:   false,
			Error:     fmt.Sprintf("Unexpected HTTP status %d", resp.StatusCode),
			ErrorType: "unknown",
			HTTPStatus: resp.StatusCode,
		}
	}

	var data TeamMemoryData
	if err := json.Unmarshal(body, &data); err != nil {
		logger.WarnCF("teammemsync", "invalid response format", map[string]any{
			"error": err.Error(),
		})
		return &FetchResult{
			Success:   false,
			Error:     "Invalid team memory response format",
			SkipRetry: true,
			ErrorType: "parse",
		}
	}

	// Extract checksum from response data or ETag header.
	checksum := data.Checksum
	if checksum == "" {
		checksum = strings.Trim(resp.Header.Get("ETag"), `"`)
	}

	logger.DebugCF("teammemsync", "fetch succeeded", map[string]any{
		"checksum":    checksum,
		"entry_count": len(data.Content.Entries),
	})
	return &FetchResult{
		Success:     true,
		Data:        &data,
		IsEmpty:     false,
		Checksum:    checksum,
	}
}

// fetchTeamMemoryHashes performs a GET ?view=hashes request to retrieve
// per-key checksums without entry bodies. Used for cheap serverChecksums
// refresh during 412 conflict resolution.
func fetchTeamMemoryHashes(
	ctx context.Context,
	baseURL string,
	repoSlug string,
	accessToken string,
) *HashesResult {
	url := fmt.Sprintf("%s/api/claude_code/team_memory?repo=%s&view=hashes", baseURL, repoSlug)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return &HashesResult{Success: false, Error: err.Error(), ErrorType: "unknown"}
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", oauth.OAuthBetaHeader)

	client := &http.Client{Timeout: pullTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return &HashesResult{Success: false, Error: err.Error(), ErrorType: "network"}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &HashesResult{Success: true, EntryChecksums: make(map[string]string)}
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var result struct {
		Checksum       string            `json:"checksum"`
		Version        int               `json:"version"`
		EntryChecksums map[string]string `json:"entryChecksums"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return &HashesResult{
			Success:   false,
			Error:     "Server did not return entryChecksums (?view=hashes unsupported)",
			ErrorType: "parse",
		}
	}

	checksum := result.Checksum
	if checksum == "" {
		checksum = strings.Trim(resp.Header.Get("ETag"), `"`)
	}

	return &HashesResult{
		Success:        true,
		Version:        result.Version,
		Checksum:       checksum,
		EntryChecksums: result.EntryChecksums,
	}
}

// PullTeamMemory fetches team memory from the server and writes to the local
// directory. Returns the number of files actually written to disk.
func PullTeamMemory(
	ctx context.Context,
	state *SyncState,
	baseURL string,
	repoSlug string,
	accessToken string,
	projectRoot string,
	skipEtagCache bool,
) (success bool, filesWritten int, entryCount int, notModified bool, errStr string) {
	if !skipEtagCache && state.LastKnownChecksum != "" {
		// Use conditional request.
	}

	etag := state.LastKnownChecksum
	if skipEtagCache {
		etag = ""
	}

	// Fetch with retries.
	var lastResult *FetchResult
	for attempt := 0; attempt <= DefaultMaxRetries; attempt++ {
		result := fetchTeamMemoryOnce(ctx, baseURL, repoSlug, accessToken, etag)
		lastResult = result

		if result.Success || result.SkipRetry {
			break
		}
		if attempt >= DefaultMaxRetries {
			break
		}
		delay := exponentialBackoff(attempt)
		logger.DebugCF("teammemsync", "pull retry", map[string]any{
			"attempt":  attempt + 1,
			"max":      DefaultMaxRetries,
			"delay_ms": delay.Milliseconds(),
		})
		select {
		case <-ctx.Done():
			return false, 0, 0, false, ctx.Err().Error()
		case <-time.After(delay):
		}
	}

	result := lastResult
	if !result.Success {
		return false, 0, 0, false, result.Error
	}
	if result.NotModified {
		return true, 0, 0, true, ""
	}
	if result.IsEmpty || result.Data == nil {
		state.ServerChecksums = make(map[string]string)
		return true, 0, 0, false, ""
	}

	// Update state.
	if result.Checksum != "" {
		state.LastKnownChecksum = result.Checksum
	}

	// Refresh serverChecksums from the server response.
	state.ServerChecksums = make(map[string]string)
	if result.Data.Content.EntryChecksums != nil {
		for k, v := range result.Data.Content.EntryChecksums {
			state.ServerChecksums[k] = v
		}
	} else {
		logger.DebugCF("teammemsync", "server response missing entryChecksums (pre-#283027 deploy) — next push will be full, not delta", nil)
	}

	entries := result.Data.Content.Entries
	written, err := writeRemoteEntriesToLocal(entries, projectRoot)
	if err != nil {
		return false, 0, 0, false, err.Error()
	}

	logger.InfoCF("teammemsync", "pull completed", map[string]any{
		"files_written": written,
		"entry_count":   len(entries),
	})
	return true, written, len(entries), false, ""
}

// writeRemoteEntriesToLocal writes server-provided team memory entries to the
// local filesystem. Validates every path key against the team memory directory
// boundary. Skips entries whose on-disk content already matches.
func writeRemoteEntriesToLocal(entries map[string]string, projectRoot string) (int, error) {
	var filesWritten int

	for relPath, content := range entries {
		validatedPath, err := ValidateTeamMemKey(relPath, projectRoot)
		if err != nil {
			if _, ok := err.(*PathTraversalError); ok {
				logger.WarnCF("teammemsync", "skipping entry with traversal key", map[string]any{
					"key":   relPath,
					"error": err.Error(),
				})
				continue
			}
			return filesWritten, err
		}

		if len(content) > MaxFileSizeBytes {
			logger.InfoCF("teammemsync", "skipping oversized remote entry", map[string]any{
				"key":  relPath,
				"size": len(content),
			})
			continue
		}

		// Skip if on-disk content already matches.
		if existing, readErr := os.ReadFile(validatedPath); readErr == nil {
			if string(existing) == content {
				continue
			}
		}

		// Ensure parent directory exists and write the file.
		parentDir := filepath.Dir(validatedPath)
		if err := os.MkdirAll(parentDir, 0o755); err != nil {
			logger.WarnCF("teammemsync", "failed to create directory", map[string]any{
				"path":  parentDir,
				"error": err.Error(),
			})
			continue
		}

		if err := os.WriteFile(validatedPath, []byte(content), 0o644); err != nil {
			logger.WarnCF("teammemsync", "failed to write entry", map[string]any{
				"key":   relPath,
				"error": err.Error(),
			})
			continue
		}
		filesWritten++
	}

	return filesWritten, nil
}
