package mcpb

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// errPathTraversal is returned when a ZIP entry attempts to escape the
// extraction root directory via relative path components.
var errPathTraversal = fmt.Errorf("path traversal detected in MCPB archive")

// ExtractMcpb extracts a ZIP-encoded MCPB file from data into extractPath.
// File mode bits from the ZIP central directory are preserved for executable
// files. onProgress is called with status messages (may be nil).
func ExtractMcpb(data []byte, extractPath string, onProgress ProgressCallback) error {
	if onProgress != nil {
		onProgress("Extracting files...")
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("failed to open MCPB archive: %w", err)
	}

	// Create extraction directory.
	if err := os.MkdirAll(extractPath, 0o755); err != nil {
		return fmt.Errorf("failed to create extraction directory %s: %w", extractPath, err)
	}

	var filesWritten int
	for _, f := range reader.File {
		// Skip directory entries.
		if f.FileInfo().IsDir() {
			continue
		}

		fullPath := filepath.Join(extractPath, f.Name)

		// Reject entries that escape the extraction root (zip slip).
		cleaned := filepath.Clean(fullPath)
		cleanRoot := filepath.Clean(extractPath)
		if !strings.HasPrefix(cleaned, cleanRoot+string(os.PathSeparator)) &&
			cleaned != cleanRoot {
			logger.WarnCF("plugin.mcpb", "rejecting path traversal in MCPB archive", map[string]any{
				"entry": f.Name,
				"path":  cleaned,
			})
			return errPathTraversal
		}

		// Ensure parent directory exists.
		if dir := filepath.Dir(cleaned); dir != cleanRoot {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}

		// Write file content.
		if err := writeZipFile(f, cleaned); err != nil {
			return fmt.Errorf("failed to extract %s: %w", f.Name, err)
		}

		// Preserve executable permission.
		if mode := f.Mode(); mode&0o111 != 0 {
			if err := os.Chmod(cleaned, mode&0o777); err != nil {
				// Swallow permission errors (NFS root_squash, FUSE mounts).
				logger.DebugCF("plugin.mcpb", "chmod failed (non-fatal)", map[string]any{
					"file":  f.Name,
					"error": err.Error(),
				})
			}
		}

		filesWritten++
		if onProgress != nil && filesWritten%10 == 0 {
			onProgress(fmt.Sprintf("Extracted %d files", filesWritten))
		}
	}

	logger.DebugCF("plugin.mcpb", "extracted MCPB", map[string]any{
		"path":  extractPath,
		"files": filesWritten,
	})

	if onProgress != nil {
		onProgress(fmt.Sprintf("Extraction complete (%d files)", filesWritten))
	}

	return nil
}

// writeZipFile writes a single file from a ZIP archive to disk.
func writeZipFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Determine write mode based on file extension.
	mode := os.FileMode(0o644)
	if isTextFile(f.Name) {
		mode = 0o644
	}

	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return err
	}

	return nil
}

// isTextFile returns true for file extensions that typically contain text.
func isTextFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".json", ".js", ".ts", ".txt", ".md", ".yml", ".yaml", ".toml", ".xml", ".html", ".css":
		return true
	default:
		return false
	}
}
