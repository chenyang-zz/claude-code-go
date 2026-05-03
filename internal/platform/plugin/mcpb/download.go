package mcpb

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// downloadTimeout is the maximum time for an MCPB file download.
	downloadTimeout = 120 * time.Second
	// maxDownloadRetries is the number of retry attempts for failed downloads.
	maxDownloadRetries = 3
	// baseRetryDelayMs is the initial backoff delay in milliseconds.
	baseRetryDelayMs = 500
)

// DownloadMcpb downloads an MCPB file from url and saves it to destPath.
// It returns the file data and its SHA-256 content hash. onProgress is called
// with status messages during the download (may be nil).
func DownloadMcpb(url, destPath string, onProgress ProgressCallback) ([]byte, string, error) {
	logger.DebugCF("plugin.mcpb", "downloading MCPB", map[string]any{
		"url": url,
	})

	if onProgress != nil {
		onProgress("Downloading " + url + "...")
	}

	data, err := downloadWithRetry(url, onProgress)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download MCPB file from %s: %w", url, err)
	}

	// Generate content hash before writing to disk.
	hash := contentHash(data)

	// Save to disk.
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return nil, "", fmt.Errorf("failed to save downloaded MCPB to %s: %w", destPath, err)
	}

	logger.DebugCF("plugin.mcpb", "downloaded MCPB", map[string]any{
		"url":   url,
		"bytes": len(data),
		"hash":  hash,
		"dest":  destPath,
	})

	if onProgress != nil {
		onProgress("Download complete")
	}

	return data, hash, nil
}

// downloadWithRetry performs an HTTP GET with exponential backoff retry.
func downloadWithRetry(url string, onProgress ProgressCallback) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= maxDownloadRetries; attempt++ {
		data, err := doDownload(url)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if attempt < maxDownloadRetries {
			delay := time.Duration(baseRetryDelayMs*(1<<(attempt-1))) * time.Millisecond
			logger.DebugCF("plugin.mcpb", "download retry", map[string]any{
				"url":     url,
				"attempt": attempt,
				"delay":   delay.String(),
				"error":   err.Error(),
			})
			time.Sleep(delay)
		}
	}
	return nil, fmt.Errorf("download failed after %d attempts: %w", maxDownloadRetries, lastErr)
}

// doDownload performs a single HTTP GET request for an MCPB file.
func doDownload(url string) ([]byte, error) {
	client := &http.Client{
		Timeout: downloadTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// contentHash returns the first 16 hex characters of the SHA-256 hash of data.
func contentHash(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}
