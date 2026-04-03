package fs

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// LocalFS is the production FileSystem implementation backed by the local OS filesystem.
type LocalFS struct{}

// NewLocalFS creates a FileSystem implementation backed by os package operations.
func NewLocalFS() *LocalFS {
	return &LocalFS{}
}

// Stat returns file metadata while following symlinks.
func (LocalFS) Stat(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	logFSResult("stat", path, err)
	return info, err
}

// Lstat returns file metadata without following symlinks.
func (LocalFS) Lstat(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	logFSResult("lstat", path, err)
	return info, err
}

// ReadDir lists directory entries for the provided path.
func (LocalFS) ReadDir(path string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(path)
	logFSResult("readdir", path, err)
	return entries, err
}

// ReadFile loads the entire file into memory.
func (LocalFS) ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	logFSResult("readfile", path, err)
	return data, err
}

// OpenRead opens one filesystem entry for streaming reads.
func (LocalFS) OpenRead(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	logFSResult("openread", path, err)
	return file, err
}

// WriteFile writes or replaces a file with the provided permissions.
func (LocalFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	err := os.WriteFile(path, data, perm)
	logFSResult("writefile", path, err)
	return err
}

// MkdirAll creates the directory tree required by path.
func (LocalFS) MkdirAll(path string, perm os.FileMode) error {
	err := os.MkdirAll(path, perm)
	logFSResult("mkdirall", path, err)
	return err
}

// Rename moves or renames a filesystem entry.
func (LocalFS) Rename(oldPath, newPath string) error {
	err := os.Rename(oldPath, newPath)
	logger.DebugCF("fs", "filesystem operation finished", map[string]any{
		"operation": "rename",
		"old_path":  oldPath,
		"new_path":  newPath,
		"error":     errorString(err),
	})
	return err
}

// Remove deletes a single filesystem entry.
func (LocalFS) Remove(path string) error {
	err := os.Remove(path)
	logFSResult("remove", path, err)
	return err
}

// RemoveAll recursively deletes a filesystem subtree.
func (LocalFS) RemoveAll(path string) error {
	err := os.RemoveAll(path)
	logFSResult("removeall", path, err)
	return err
}

// Readlink returns the target of a symbolic link.
func (LocalFS) Readlink(path string) (string, error) {
	target, err := os.Readlink(path)
	logger.DebugCF("fs", "filesystem operation finished", map[string]any{
		"operation": "readlink",
		"path":      path,
		"target":    target,
		"error":     errorString(err),
	})
	return target, err
}

// EvalSymlinks resolves symlinks in path and returns the canonical result.
func (LocalFS) EvalSymlinks(path string) (string, error) {
	resolvedPath, err := filepathEvalSymlinks(path)
	logger.DebugCF("fs", "filesystem operation finished", map[string]any{
		"operation": "evalsymlinks",
		"path":      path,
		"resolved":  resolvedPath,
		"error":     errorString(err),
	})
	return resolvedPath, err
}

// logFSResult records the outcome of a filesystem call without forcing callers to repeat the pattern.
func logFSResult(operation, path string, err error) {
	logger.DebugCF("fs", "filesystem operation finished", map[string]any{
		"operation": operation,
		"path":      path,
		"error":     errorString(err),
	})
}

// errorString normalizes nil and non-nil errors for structured logging fields.
func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// filepathEvalSymlinks exists as a seam for tests that need the standard implementation today.
var filepathEvalSymlinks = filepath.EvalSymlinks
