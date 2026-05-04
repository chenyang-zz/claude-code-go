package internallogging

import (
	"os"
	"path/filepath"
	"testing"

)

func TestIsInternalLoggingEnabled_Disabled(t *testing.T) {
	os.Unsetenv("CLAUDE_FEATURE_INTERNAL_LOGGING")
	if IsInternalLoggingEnabled() {
		t.Error("expected IsInternalLoggingEnabled to return false when flag is unset")
	}
}

func TestIsInternalLoggingEnabled_Enabled(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_INTERNAL_LOGGING", "1")
	defer os.Unsetenv("CLAUDE_FEATURE_INTERNAL_LOGGING")
	if !IsInternalLoggingEnabled() {
		t.Error("expected IsInternalLoggingEnabled to return true when flag is set to 1")
	}
}

func TestGetKubernetesNamespace_Disabled(t *testing.T) {
	os.Unsetenv("CLAUDE_FEATURE_INTERNAL_LOGGING")
	if got := GetKubernetesNamespace(); got != "" {
		t.Errorf("expected empty string when disabled, got %q", got)
	}
}

func TestReadNamespaceFile_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "namespace")
	if err := os.WriteFile(path, []byte("production\n"), 0644); err != nil {
		t.Fatalf("failed to write namespace file: %v", err)
	}

	oldPath := namespaceFilePath
	namespaceFilePath = path
	defer func() { namespaceFilePath = oldPath }()

	got := readNamespaceFile()
	if got != "production" {
		t.Errorf("expected 'production', got %q", got)
	}
}

func TestReadNamespaceFile_Missing(t *testing.T) {
	oldPath := namespaceFilePath
	namespaceFilePath = "/nonexistent/namespace"
	defer func() { namespaceFilePath = oldPath }()

	got := readNamespaceFile()
	if got != namespaceNotFound {
		t.Errorf("expected %q, got %q", namespaceNotFound, got)
	}
}

func TestReadNamespaceFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "namespace")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty namespace file: %v", err)
	}

	oldPath := namespaceFilePath
	namespaceFilePath = path
	defer func() { namespaceFilePath = oldPath }()

	got := readNamespaceFile()
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestGetContainerID_Disabled(t *testing.T) {
	os.Unsetenv("CLAUDE_FEATURE_INTERNAL_LOGGING")
	if got := GetContainerID(); got != "" {
		t.Errorf("expected empty string when disabled, got %q", got)
	}
}

func TestReadMountinfoFile_Docker(t *testing.T) {
	content := `262 260 0:54 / / rw,relatime master:1 - overlay /docker/containers/abc123def456abc123def456abc123def456abc123def456abc123def456abcd/rootfs rw,lowerdir=/var/lib/docker/overlay2/l/XYZ,upperdir=/var/lib/docker/overlay2/ABC/diff,workdir=/var/lib/docker/overlay2/ABC/work`

	dir := t.TempDir()
	path := filepath.Join(dir, "mountinfo")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write mountinfo: %v", err)
	}

	oldPath := mountinfoFilePath
	mountinfoFilePath = path
	defer func() { mountinfoFilePath = oldPath }()

	got := readMountinfoFile()
	want := "abc123def456abc123def456abc123def456abc123def456abc123def456abcd"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestReadMountinfoFile_Containerd(t *testing.T) {
	content := `262 260 0:54 / / rw,relatime - overlay /sandboxes/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef/rootfs rw`

	dir := t.TempDir()
	path := filepath.Join(dir, "mountinfo")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write mountinfo: %v", err)
	}

	oldPath := mountinfoFilePath
	mountinfoFilePath = path
	defer func() { mountinfoFilePath = oldPath }()

	got := readMountinfoFile()
	want := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestReadMountinfoFile_CRI_O(t *testing.T) {
	content := `262 260 0:54 / / rw,relatime - overlay /sandboxes/deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef/rootfs rw`

	dir := t.TempDir()
	path := filepath.Join(dir, "mountinfo")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write mountinfo: %v", err)
	}

	oldPath := mountinfoFilePath
	mountinfoFilePath = path
	defer func() { mountinfoFilePath = oldPath }()

	got := readMountinfoFile()
	want := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestReadMountinfoFile_Missing(t *testing.T) {
	oldPath := mountinfoFilePath
	mountinfoFilePath = "/nonexistent/mountinfo"
	defer func() { mountinfoFilePath = oldPath }()

	got := readMountinfoFile()
	if got != containerIDNotFound {
		t.Errorf("expected %q, got %q", containerIDNotFound, got)
	}
}

func TestReadMountinfoFile_NoMatch(t *testing.T) {
	content := `262 260 0:54 / / rw,relatime - overlay /some/random/path/rootfs rw`

	dir := t.TempDir()
	path := filepath.Join(dir, "mountinfo")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write mountinfo: %v", err)
	}

	oldPath := mountinfoFilePath
	mountinfoFilePath = path
	defer func() { mountinfoFilePath = oldPath }()

	got := readMountinfoFile()
	if got != containerIDNotFoundInMountinfo {
		t.Errorf("expected %q, got %q", containerIDNotFoundInMountinfo, got)
	}
}

func TestReadMountinfoFile_InvalidHexLength(t *testing.T) {
	// 63 chars instead of 64 — should not match.
	content := `262 260 0:54 / / rw,relatime - overlay /docker/containers/abc123def456abc123def456abc123def456abc123def456abc123def456abc/rootfs rw`

	dir := t.TempDir()
	path := filepath.Join(dir, "mountinfo")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write mountinfo: %v", err)
	}

	oldPath := mountinfoFilePath
	mountinfoFilePath = path
	defer func() { mountinfoFilePath = oldPath }()

	got := readMountinfoFile()
	if got != containerIDNotFoundInMountinfo {
		t.Errorf("expected %q, got %q", containerIDNotFoundInMountinfo, got)
	}
}

func TestReadMountinfoFile_NonHexChars(t *testing.T) {
	// Contains non-hex characters — should not match.
	content := `262 260 0:54 / / rw,relatime - overlay /docker/containers/zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz/rootfs rw`

	dir := t.TempDir()
	path := filepath.Join(dir, "mountinfo")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write mountinfo: %v", err)
	}

	oldPath := mountinfoFilePath
	mountinfoFilePath = path
	defer func() { mountinfoFilePath = oldPath }()

	got := readMountinfoFile()
	if got != containerIDNotFoundInMountinfo {
		t.Errorf("expected %q, got %q", containerIDNotFoundInMountinfo, got)
	}
}

func TestInit_DoesNotPanic(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_INTERNAL_LOGGING", "1")
	defer os.Unsetenv("CLAUDE_FEATURE_INTERNAL_LOGGING")

	// Init should not panic regardless of flag state.
	Init(InitOptions{})
}
