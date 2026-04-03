package fs

import (
	"errors"
	stdfs "io/fs"
	"syscall"
	"testing"
)

// TestErrnoCode verifies common filesystem sentinel errors map to stable errno-like strings.
func TestErrnoCode(t *testing.T) {
	t.Parallel()

	if got := ErrnoCode(stdfs.ErrNotExist); got != "ENOENT" {
		t.Fatalf("ErrnoCode(fs.ErrNotExist) = %q, want %q", got, "ENOENT")
	}
	if got := ErrnoCode(stdfs.ErrPermission); got != "EACCES" {
		t.Fatalf("ErrnoCode(fs.ErrPermission) = %q, want %q", got, "EACCES")
	}
	if got := ErrnoCode(syscall.ELOOP); got != "ELOOP" {
		t.Fatalf("ErrnoCode(syscall.ELOOP) = %q, want %q", got, "ELOOP")
	}
	if got := ErrnoCode(errors.New("boom")); got != "" {
		t.Fatalf("ErrnoCode(custom error) = %q, want empty string", got)
	}
}

// TestIsInaccessible verifies missing-path and permission failures share one expected-path classification.
func TestIsInaccessible(t *testing.T) {
	t.Parallel()

	if !IsInaccessible(stdfs.ErrNotExist) {
		t.Fatal("IsInaccessible(fs.ErrNotExist) = false, want true")
	}
	if !IsInaccessible(stdfs.ErrPermission) {
		t.Fatal("IsInaccessible(fs.ErrPermission) = false, want true")
	}
	if !IsInaccessible(syscall.ENOTDIR) {
		t.Fatal("IsInaccessible(syscall.ENOTDIR) = false, want true")
	}
	if IsInaccessible(errors.New("boom")) {
		t.Fatal("IsInaccessible(custom error) = true, want false")
	}
}
