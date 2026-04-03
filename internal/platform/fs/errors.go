package fs

import (
	"errors"
	"io/fs"
	"syscall"
)

// ErrnoCode extracts a stable errno-like code from a filesystem error when available.
func ErrnoCode(err error) string {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		switch {
		case errors.Is(pathErr.Err, fs.ErrNotExist):
			return "ENOENT"
		case errors.Is(pathErr.Err, fs.ErrPermission):
			return "EACCES"
		}
	}

	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ENOENT:
			return "ENOENT"
		case syscall.EACCES:
			return "EACCES"
		case syscall.EPERM:
			return "EPERM"
		case syscall.ENOTDIR:
			return "ENOTDIR"
		case syscall.ELOOP:
			return "ELOOP"
		}
	}

	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "ENOENT"
	case errors.Is(err, fs.ErrPermission):
		return "EACCES"
	default:
		return ""
	}
}

// IsNotExist reports whether err represents a missing path.
func IsNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}

// IsPermission reports whether err represents a permission failure.
func IsPermission(err error) bool {
	return errors.Is(err, fs.ErrPermission)
}

// IsInaccessible reports whether err should be treated as an expected inaccessible-path failure.
func IsInaccessible(err error) bool {
	switch ErrnoCode(err) {
	case "ENOENT", "EACCES", "EPERM", "ENOTDIR", "ELOOP":
		return true
	default:
		return false
	}
}
