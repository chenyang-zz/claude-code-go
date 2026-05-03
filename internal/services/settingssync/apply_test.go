package settingssync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileForSync_Success(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "subdir", "file.txt")
	if !WriteFileForSync(p, "hello world") {
		t.Fatal("WriteFileForSync should succeed")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("content: got %q, want %q", string(data), "hello world")
	}
}

func TestWriteFileForSync_Overwrite(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file.txt")
	os.WriteFile(p, []byte("old"), 0o644)
	if !WriteFileForSync(p, "new content") {
		t.Fatal("WriteFileForSync overwrite should succeed")
	}
	data, _ := os.ReadFile(p)
	if string(data) != "new content" {
		t.Errorf("content: got %q, want %q", string(data), "new content")
	}
}

func TestApplyRemoteEntriesToLocal_AllFourKeys(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()

	entries := map[string]string{
		KeyUserSettings:                  `{"theme":"dark"}`,
		KeyUserMemory:                    "User memory from remote",
		ProjectSettingsKey("org/repo"):   `{"local":true}`,
		ProjectMemoryKey("org/repo"):     "Project memory from remote",
	}

	ApplyRemoteEntriesToLocal(entries, cwd, home, "org/repo")

	// Verify user settings written
	userSettings := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(userSettings)
	if err != nil {
		t.Fatalf("user settings not written: %v", err)
	}
	if string(data) != `{"theme":"dark"}` {
		t.Errorf("user settings: got %q", string(data))
	}

	// Verify user memory written
	userMem := filepath.Join(home, "CLAUDE.md")
	data, err = os.ReadFile(userMem)
	if err != nil {
		t.Fatalf("user memory not written: %v", err)
	}
	if string(data) != "User memory from remote" {
		t.Errorf("user memory: got %q", string(data))
	}

	// Verify project settings written
	projSettings := filepath.Join(cwd, ".claude", "settings.local.json")
	data, err = os.ReadFile(projSettings)
	if err != nil {
		t.Fatalf("project settings not written: %v", err)
	}
	if string(data) != `{"local":true}` {
		t.Errorf("project settings: got %q", string(data))
	}

	// Verify project memory written
	projMem := filepath.Join(cwd, "CLAUDE.local.md")
	data, err = os.ReadFile(projMem)
	if err != nil {
		t.Fatalf("project memory not written: %v", err)
	}
	if string(data) != "Project memory from remote" {
		t.Errorf("project memory: got %q", string(data))
	}
}

func TestApplyRemoteEntriesToLocal_NoProjectID(t *testing.T) {
	home := t.TempDir()

	entries := map[string]string{
		KeyUserSettings: `{"theme":"dark"}`,
		KeyUserMemory:   "Memory",
	}

	ApplyRemoteEntriesToLocal(entries, "", home, "")

	// Verify user settings written
	userSettings := filepath.Join(home, ".claude", "settings.json")
	if _, err := os.ReadFile(userSettings); err != nil {
		t.Errorf("user settings not written: %v", err)
	}

	// Verify user memory written
	userMem := filepath.Join(home, "CLAUDE.md")
	if _, err := os.ReadFile(userMem); err != nil {
		t.Errorf("user memory not written: %v", err)
	}
}

func TestApplyRemoteEntriesToLocal_ExceedsSizeLimit(t *testing.T) {
	home := t.TempDir()

	large := make([]byte, maxFileSizeBytes+1)
	for i := range large {
		large[i] = 'x'
	}

	entries := map[string]string{
		KeyUserSettings: string(large),
	}

	ApplyRemoteEntriesToLocal(entries, "", home, "")

	// Verify NOT written (exceeds size limit)
	userSettings := filepath.Join(home, ".claude", "settings.json")
	if _, err := os.ReadFile(userSettings); err == nil {
		t.Error("exceeds-size-limit entry should not be written")
	}
}

func TestApplyRemoteEntriesToLocal_PartialEntries(t *testing.T) {
	home := t.TempDir()

	entries := map[string]string{
		KeyUserSettings: `{"theme":"dark"}`,
	}

	ApplyRemoteEntriesToLocal(entries, "", home, "")

	// Settings should be written
	userSettings := filepath.Join(home, ".claude", "settings.json")
	if _, err := os.ReadFile(userSettings); err != nil {
		t.Errorf("user settings not written: %v", err)
	}

	// Memory should NOT be written (not in entries)
	userMem := filepath.Join(home, "CLAUDE.md")
	if _, err := os.ReadFile(userMem); err == nil {
		t.Error("memory should not be written when absent from entries")
	}
}
