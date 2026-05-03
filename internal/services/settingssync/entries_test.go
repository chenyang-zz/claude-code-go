package settingssync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTryReadFile_NonExistent(t *testing.T) {
	b := TryReadFile("/tmp/settingssync_nonexistent_test_file")
	if b != nil {
		t.Error("non-existent file should return nil")
	}
}

func TestTryReadFile_Empty(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(p, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	b := TryReadFile(p)
	if b != nil {
		t.Error("empty file should return nil")
	}
}

func TestTryReadFile_WhitespaceOnly(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "whitespace.txt")
	if err := os.WriteFile(p, []byte("   \n\t\n  "), 0o644); err != nil {
		t.Fatal(err)
	}
	b := TryReadFile(p)
	if b != nil {
		t.Error("whitespace-only file should return nil")
	}
}

func TestTryReadFile_Valid(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	content := `{"theme":"dark"}`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	b := TryReadFile(p)
	if b == nil {
		t.Fatal("valid file should be readable")
	}
	if string(b) != content {
		t.Errorf("content mismatch: got %q, want %q", string(b), content)
	}
}

func TestTryReadFile_TooLarge(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "large.json")
	large := make([]byte, maxFileSizeBytes+1)
	if err := os.WriteFile(p, large, 0o644); err != nil {
		t.Fatal(err)
	}
	b := TryReadFile(p)
	if b != nil {
		t.Error("file exceeding size limit should return nil")
	}
}

func TestTryReadFile_EmptyPath(t *testing.T) {
	if b := TryReadFile(""); b != nil {
		t.Error("empty path should return nil")
	}
}

func TestBuildEntriesFromLocalFiles_AllPresent(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()

	// Create ~/.claude/settings.json
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"theme":"dark"}`), 0o644)

	// Create ~/.claude/CLAUDE.md
	os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("User memory content"), 0o644)

	// Create <cwd>/.claude/settings.local.json
	projectClaudeDir := filepath.Join(cwd, ".claude")
	os.MkdirAll(projectClaudeDir, 0o755)
	os.WriteFile(filepath.Join(projectClaudeDir, "settings.local.json"), []byte(`{"local":true}`), 0o644)

	// Create <cwd>/CLAUDE.local.md
	os.WriteFile(filepath.Join(cwd, "CLAUDE.local.md"), []byte("Project memory"), 0o644)

	entries := BuildEntriesFromLocalFiles(cwd, home, "org/repo")

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d: %v", len(entries), entries)
	}
	if entries[KeyUserSettings] != `{"theme":"dark"}` {
		t.Errorf("user settings: got %q", entries[KeyUserSettings])
	}
	if entries[KeyUserMemory] != "User memory content" {
		t.Errorf("user memory: got %q", entries[KeyUserMemory])
	}
	if entries[ProjectSettingsKey("org/repo")] != `{"local":true}` {
		t.Errorf("project settings: got %q", entries[ProjectSettingsKey("org/repo")])
	}
	if entries[ProjectMemoryKey("org/repo")] != "Project memory" {
		t.Errorf("project memory: got %q", entries[ProjectMemoryKey("org/repo")])
	}
}

func TestBuildEntriesFromLocalFiles_NoProjectID(t *testing.T) {
	home := t.TempDir()

	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"theme":"dark"}`), 0o644)
	os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("User memory"), 0o644)

	entries := BuildEntriesFromLocalFiles("", home, "")

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (user only), got %d", len(entries))
	}
	if entries[KeyUserSettings] != `{"theme":"dark"}` {
		t.Error("user settings missing")
	}
	if entries[KeyUserMemory] != "User memory" {
		t.Error("user memory missing")
	}
}

func TestBuildEntriesFromLocalFiles_PartialFiles(t *testing.T) {
	home := t.TempDir()

	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"key":"val"}`), 0o644)
	// No CLAUDE.md — should be skipped

	entries := BuildEntriesFromLocalFiles("", home, "")

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (settings only), got %d", len(entries))
	}
	if entries[KeyUserSettings] != `{"key":"val"}` {
		t.Error("user settings mismatch")
	}
}

func TestSanitizeRemoteURL_GitHubSSH(t *testing.T) {
	got := sanitizeRemoteURL("git@github.com:org/repo.git")
	if got != "org/repo" {
		t.Errorf("got %q, want %q", got, "org/repo")
	}
}

func TestSanitizeRemoteURL_GitHubHTTPS(t *testing.T) {
	got := sanitizeRemoteURL("https://github.com/org/repo.git")
	if got != "org/repo" {
		t.Errorf("got %q, want %q", got, "org/repo")
	}
}

func TestSanitizeRemoteURL_Unknown(t *testing.T) {
	raw := "https://gitlab.com/other/stuff.git"
	got := sanitizeRemoteURL(raw)
	if got != raw {
		t.Errorf("unknown format should return raw: got %q", got)
	}
}

func TestResolveHomePath_WithTilde(t *testing.T) {
	got := resolveHomePath("~/.claude/settings.json", "/home/user")
	want := "/home/user/.claude/settings.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveHomePath_NoTilde(t *testing.T) {
	got := resolveHomePath("/absolute/path", "/home/user")
	if got != "/absolute/path" {
		t.Errorf("got %q, want unchanged", got)
	}
}

func TestTryReadFile_ExactlyMaxSize(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "exact.json")
	content := make([]byte, maxFileSizeBytes)
	for i := range content {
		content[i] = 'a'
	}
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	b := TryReadFile(p)
	if b == nil {
		t.Error("file at max size should be readable")
	}
	if len(b) != maxFileSizeBytes {
		t.Errorf("size mismatch: got %d, want %d", len(b), maxFileSizeBytes)
	}
}
