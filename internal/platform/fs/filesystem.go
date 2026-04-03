package fs

import "os"

// FileSystem defines the minimal filesystem contract shared by the first-batch tools.
type FileSystem interface {
	// Stat returns file metadata while following symlinks.
	Stat(path string) (os.FileInfo, error)
	// Lstat returns file metadata without following symlinks.
	Lstat(path string) (os.FileInfo, error)
	// ReadDir lists directory entries for the provided path.
	ReadDir(path string) ([]os.DirEntry, error)
	// ReadFile loads the entire file into memory.
	ReadFile(path string) ([]byte, error)
	// WriteFile writes or replaces a file with the provided permissions.
	WriteFile(path string, data []byte, perm os.FileMode) error
	// MkdirAll creates the directory tree required by path.
	MkdirAll(path string, perm os.FileMode) error
	// Rename moves or renames a filesystem entry.
	Rename(oldPath, newPath string) error
	// Remove deletes a single filesystem entry.
	Remove(path string) error
	// RemoveAll recursively deletes a filesystem subtree.
	RemoveAll(path string) error
	// Readlink returns the target of a symbolic link.
	Readlink(path string) (string, error)
	// EvalSymlinks resolves symlinks in path and returns the canonical result.
	EvalSymlinks(path string) (string, error)
}
