package teleport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sheepzhao/claude-code-go/internal/platform/shell"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// defaultBundleMaxBytes is the default maximum bundle size (100MB).
	defaultBundleMaxBytes = 100 * 1024 * 1024
	// filesAPIUploadTimeout is the timeout for Files API upload requests.
	filesAPIUploadTimeout    = 120 * time.Second
	fileUploadMaxBytes       = 500 * 1024 * 1024 // 500MB
	filesAPIBetaHeader       = "files-api-2025-04-14,oauth-2025-04-20"
	gitCommandTimeout        = 120 * time.Second
)

// GitBundleConfig holds configuration for creating and uploading a git bundle.
type GitBundleConfig struct {
	// BaseURL is the Files API base URL (e.g. https://api.anthropic.com).
	BaseURL string
	// AccessToken is the OAuth access token for authentication.
	AccessToken string
	// GitRoot is the absolute path to the git repository root.
	GitRoot string
}

// gitResult holds the outcome of a git command execution.
type gitResult struct {
	Stdout string
	Stderr string
	Code   int
}

// runGit executes a git command in the given working directory using the
// shell executor. Returns stdout, stderr, and exit code. When the command
// itself could not be launched (e.g. git not found), the exit code is -1.
func runGit(ctx context.Context, cwd, command string) gitResult {
	executor := shell.NewExecutor()
	result, err := executor.Execute(ctx, shell.Request{
		Command:    command,
		WorkingDir: cwd,
		Timeout:    gitCommandTimeout,
	})
	if err != nil {
		logger.DebugCF("teleport", "git command execution error", map[string]any{
			"command": command,
			"error":   err.Error(),
		})
		return gitResult{Code: -1, Stderr: err.Error()}
	}
	return gitResult{
		Stdout: result.Stdout,
		Stderr: result.Stderr,
		Code:   result.ExitCode,
	}
}

// bundleCreateResult is the internal result of a bundle creation attempt.
type bundleCreateResult struct {
	ok         bool
	size       int64
	scope      BundleScope
	err        string
	failReason BundleFailReason
}

// bundleWithFallback attempts --all, then HEAD, then squashed-root bundle
// creation. It mirrors _bundleWithFallback() in
// src/utils/teleport/gitBundle.ts.
func bundleWithFallback(ctx context.Context, gitRoot, bundlePath string, maxBytes int64, hasStash bool) bundleCreateResult {
	mkBundle := func(base string) gitResult {
		cmd := fmt.Sprintf("git bundle create %s %s", bundlePath, base)
		if hasStash {
			cmd = fmt.Sprintf("git bundle create %s %s refs/seed/stash", bundlePath, base)
		}
		return runGit(ctx, gitRoot, cmd)
	}

	// --all
	allResult := mkBundle("--all")
	if allResult.Code != 0 {
		return bundleCreateResult{
			ok:         false,
			err:        fmt.Sprintf("git bundle create --all failed (%d): %s", allResult.Code, truncateStr(allResult.Stderr, 200)),
			failReason: BundleFailGitError,
		}
	}

	allSize := fileSize(bundlePath)
	if allSize >= 0 && allSize <= maxBytes {
		return bundleCreateResult{ok: true, size: allSize, scope: BundleScopeAll}
	}

	logger.DebugCF("teleport", "--all bundle exceeds limit, retrying HEAD-only", map[string]any{
		"size_mb": float64(allSize) / 1024 / 1024,
		"max_mb":  float64(maxBytes) / 1024 / 1024,
	})

	// HEAD
	headResult := mkBundle("HEAD")
	if headResult.Code != 0 {
		return bundleCreateResult{
			ok:         false,
			err:        fmt.Sprintf("git bundle create HEAD failed (%d): %s", headResult.Code, truncateStr(headResult.Stderr, 200)),
			failReason: BundleFailGitError,
		}
	}

	headSize := fileSize(bundlePath)
	if headSize >= 0 && headSize <= maxBytes {
		return bundleCreateResult{ok: true, size: headSize, scope: BundleScopeHead}
	}

	logger.DebugCF("teleport", "HEAD bundle exceeds limit, retrying squashed-root", map[string]any{
		"size_mb": float64(headSize) / 1024 / 1024,
		"max_mb":  float64(maxBytes) / 1024 / 1024,
	})

	// Squashed root via git commit-tree.
	treeRef := "HEAD^{tree}"
	if hasStash {
		treeRef = "refs/seed/stash^{tree}"
	}

	commitTreeResult := runGit(ctx, gitRoot, fmt.Sprintf("git commit-tree %s -m seed", treeRef))
	if commitTreeResult.Code != 0 {
		return bundleCreateResult{
			ok:         false,
			err:        fmt.Sprintf("git commit-tree failed (%d): %s", commitTreeResult.Code, truncateStr(commitTreeResult.Stderr, 200)),
			failReason: BundleFailGitError,
		}
	}

	squashedSha := strings.TrimSpace(commitTreeResult.Stdout)
	if squashedSha == "" {
		return bundleCreateResult{
			ok:         false,
			err:        "git commit-tree returned empty SHA",
			failReason: BundleFailGitError,
		}
	}

	// Mark the squash commit so bundle creation can reference it.
	runGit(ctx, gitRoot, fmt.Sprintf("git update-ref refs/seed/root %s", squashedSha))

	squashResult := runGit(ctx, gitRoot, fmt.Sprintf("git bundle create %s refs/seed/root", bundlePath))
	if squashResult.Code != 0 {
		return bundleCreateResult{
			ok:         false,
			err:        fmt.Sprintf("git bundle create refs/seed/root failed (%d): %s", squashResult.Code, truncateStr(squashResult.Stderr, 200)),
			failReason: BundleFailGitError,
		}
	}

	squashSize := fileSize(bundlePath)
	if squashSize >= 0 && squashSize <= maxBytes {
		return bundleCreateResult{ok: true, size: squashSize, scope: BundleScopeSquashed}
	}

	return bundleCreateResult{
		ok:         false,
		err:        "Repo is too large to bundle. Please setup GitHub on https://claude.ai/code",
		failReason: BundleFailTooLarge,
	}
}

// uploadBundleResponse is the JSON response from the Files API upload endpoint.
type uploadBundleResponse struct {
	ID string `json:"id"`
}

// uploadBundleFile uploads a bundle file to the Files API at /v1/files using
// multipart form data. It mirrors the upload logic in
// src/services/api/filesApi.ts uploadFile().
func uploadBundleFile(ctx context.Context, baseURL, accessToken, filePath, filename string) (fileID string, size int64, err error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("teleport: read bundle file for upload: %w", err)
	}

	fileSize := int64(len(content))
	if fileSize > fileUploadMaxBytes {
		return "", 0, fmt.Errorf("teleport: bundle file exceeds maximum upload size (%d > %d)", fileSize, fileUploadMaxBytes)
	}

	url := fmt.Sprintf("%s/v1/files", strings.TrimRight(baseURL, "/"))

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// File part with explicit filename.
	filePart, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return "", 0, fmt.Errorf("teleport: create form file part: %w", err)
	}
	if _, err := filePart.Write(content); err != nil {
		return "", 0, fmt.Errorf("teleport: write file content to form: %w", err)
	}

	// Purpose part.
	if err := writer.WriteField("purpose", "user_data"); err != nil {
		return "", 0, fmt.Errorf("teleport: write purpose field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", 0, fmt.Errorf("teleport: close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return "", 0, fmt.Errorf("teleport: build upload request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("anthropic-beta", filesAPIBetaHeader)

	client := &http.Client{Timeout: filesAPIUploadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("teleport: upload bundle file: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("teleport: read upload response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, fmt.Errorf("teleport: upload failed (status %d %s)",
			resp.StatusCode, strings.TrimSpace(http.StatusText(resp.StatusCode)))
	}

	var parsed uploadBundleResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return "", 0, fmt.Errorf("teleport: decode upload response: %w", err)
	}
	if parsed.ID == "" {
		return "", 0, fmt.Errorf("teleport: upload response missing file id")
	}

	logger.DebugCF("teleport", "bundle upload succeeded", map[string]any{
		"file_id": parsed.ID,
		"size":    fileSize,
	})

	return parsed.ID, fileSize, nil
}

// generateBundleTempPath creates a temporary file path for a git bundle.
// It mirrors generateTempFilePath() in src/utils/tempfile.ts with the
// ccr-seed prefix and .bundle extension.
func generateBundleTempPath() (string, error) {
	uid, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("teleport: generate temp path uuid: %w", err)
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("ccr-seed-%s.bundle", uid.String())), nil
}

// CreateAndUploadGitBundle creates a git bundle with --all / HEAD / squashed-root
// fallback and uploads it to the Files API. It mirrors createAndUploadGitBundle()
// in src/utils/teleport/gitBundle.ts.
//
// The config includes:
//   - BaseURL: Files API base URL
//   - AccessToken: OAuth access token
//   - GitRoot: absolute path to the git repository root
func CreateAndUploadGitBundle(ctx context.Context, config GitBundleConfig) BundleUploadResult {
	gitRoot := config.GitRoot
	if gitRoot == "" {
		return BundleUploadResult{
			Success:    false,
			Error:      "Git root is required",
			FailReason: BundleFailGitError,
		}
	}

	// Verify git root exists on disk.
	if _, err := os.Stat(gitRoot); err != nil {
		return BundleUploadResult{
			Success:    false,
			Error:      fmt.Sprintf("Not in a git repository: %s", gitRoot),
			FailReason: BundleFailGitError,
		}
	}

	// Sweep stale seed refs from a crashed prior run before --all bundles them.
	// Runs before the empty-repo check so it is never skipped by an early return.
	for _, ref := range []string{"refs/seed/stash", "refs/seed/root"} {
		runGit(ctx, gitRoot, fmt.Sprintf("git update-ref -d %s", ref))
	}

	// Check for any commits (any ref, not just HEAD).
	r := runGit(ctx, gitRoot, "git for-each-ref --count=1 refs/")
	refOut, refCode := r.Stdout, r.Code
	if refCode == 0 && strings.TrimSpace(refOut) == "" {
		logger.DebugCF("teleport", "empty repository, cannot bundle", nil)
		return BundleUploadResult{
			Success:    false,
			Error:      "Repository has no commits yet",
			FailReason: BundleFailEmptyRepo,
		}
	}

	// Capture WIP via stash create.
	stashResult := runGit(ctx, gitRoot, "git stash create")
	wipStashSHA := ""
	hasWip := false
	if stashResult.Code == 0 {
		wipStashSHA = strings.TrimSpace(stashResult.Stdout)
		hasWip = wipStashSHA != ""
	}
	if stashResult.Code != 0 {
		logger.DebugCF("teleport", "git stash create failed, proceeding without WIP", map[string]any{
			"code": stashResult.Code,
		})
	} else if hasWip {
		logger.DebugCF("teleport", "captured WIP stash", map[string]any{
			"sha": wipStashSHA,
		})
		runGit(ctx, gitRoot, fmt.Sprintf("git update-ref refs/seed/stash %s", wipStashSHA))
	}

	// Generate a temp path for the bundle file (do NOT create the file — git
	// bundle create will create/overwrite it).
	bundlePath, err := generateBundleTempPath()
	if err != nil {
		return BundleUploadResult{
			Success:    false,
			Error:      fmt.Sprintf("Failed to generate temp path: %s", err.Error()),
			FailReason: BundleFailGitError,
		}
	}

	// Cleanup: remove temp bundle file and seed refs in all exit paths.
	defer func() {
		if err := os.Remove(bundlePath); err != nil && !os.IsNotExist(err) {
			logger.DebugCF("teleport", "could not delete temp bundle", map[string]any{
				"path":  bundlePath,
				"error": err.Error(),
			})
		}
		for _, ref := range []string{"refs/seed/stash", "refs/seed/root"} {
			runGit(ctx, gitRoot, fmt.Sprintf("git update-ref -d %s", ref))
		}
	}()

	// Bundle with --all / HEAD / squashed-root fallback.
	bundle := bundleWithFallback(ctx, gitRoot, bundlePath, defaultBundleMaxBytes, hasWip)
	if !bundle.ok {
		logger.DebugCF("teleport", "bundle creation failed", map[string]any{
			"error":       bundle.err,
			"fail_reason": string(bundle.failReason),
		})
		return BundleUploadResult{
			Success:    false,
			Error:      bundle.err,
			FailReason: bundle.failReason,
		}
	}

	// Upload to Files API.
	fileID, uploadedSize, err := uploadBundleFile(ctx, config.BaseURL, config.AccessToken, bundlePath, "_source_seed.bundle")
	if err != nil {
		logger.DebugCF("teleport", "bundle upload failed", map[string]any{
			"error": err.Error(),
		})
		return BundleUploadResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	logger.DebugCF("teleport", "bundle created and uploaded successfully", map[string]any{
		"file_id": fileID,
		"size":    uploadedSize,
		"scope":   string(bundle.scope),
		"has_wip": hasWip,
	})

	return BundleUploadResult{
		Success:         true,
		FileID:          fileID,
		BundleSizeBytes: uploadedSize,
		Scope:           bundle.scope,
		HasWip:          hasWip,
	}
}

// fileSize returns the size of the file at path, or -1 if it cannot be stated.
func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return info.Size()
}

// truncateStr truncates s to maxLen characters.
func truncateStr(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}
