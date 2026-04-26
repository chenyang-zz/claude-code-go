package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckAgentMemorySnapshot_NoSnapshot(t *testing.T) {
	dir := t.TempDir()
	paths := &Paths{CWD: dir}

	result, err := paths.CheckAgentMemorySnapshot("explore", ScopeProject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "none" {
		t.Errorf("action = %q, want none", result.Action)
	}
}

func TestCheckAgentMemorySnapshot_Initialize(t *testing.T) {
	dir := t.TempDir()
	paths := &Paths{CWD: dir}

	// Create snapshot with no local memory.
	createSnapshot(t, paths, "explore", time.Now().Format(time.RFC3339))

	result, err := paths.CheckAgentMemorySnapshot("explore", ScopeProject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "initialize" {
		t.Errorf("action = %q, want initialize", result.Action)
	}
	if result.SnapshotTimestamp == "" {
		t.Error("expected snapshot timestamp")
	}
}

func TestCheckAgentMemorySnapshot_PromptUpdate(t *testing.T) {
	dir := t.TempDir()
	paths := &Paths{CWD: dir}

	// Create snapshot with a newer timestamp.
	now := time.Now()
	createSnapshot(t, paths, "explore", now.Format(time.RFC3339))

	// Create local memory and an older synced marker.
	memDir := paths.GetAgentMemoryDir("explore", ScopeProject)
	_ = EnsureAgentMemoryDir(memDir)
	_ = os.WriteFile(filepath.Join(memDir, "test.md"), []byte("hello"), 0644)

	oldTime := now.Add(-time.Hour).Format(time.RFC3339)
	_ = paths.saveSyncedMeta("explore", ScopeProject, oldTime)

	result, err := paths.CheckAgentMemorySnapshot("explore", ScopeProject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "prompt-update" {
		t.Errorf("action = %q, want prompt-update", result.Action)
	}
}

func TestCheckAgentMemorySnapshot_AlreadySynced(t *testing.T) {
	dir := t.TempDir()
	paths := &Paths{CWD: dir}

	now := time.Now().Format(time.RFC3339)
	createSnapshot(t, paths, "explore", now)

	memDir := paths.GetAgentMemoryDir("explore", ScopeProject)
	_ = EnsureAgentMemoryDir(memDir)
	_ = os.WriteFile(filepath.Join(memDir, "test.md"), []byte("hello"), 0644)
	_ = paths.saveSyncedMeta("explore", ScopeProject, now)

	result, err := paths.CheckAgentMemorySnapshot("explore", ScopeProject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "none" {
		t.Errorf("action = %q, want none", result.Action)
	}
}

func TestInitializeFromSnapshot(t *testing.T) {
	dir := t.TempDir()
	paths := &Paths{CWD: dir}

	now := time.Now().Format(time.RFC3339)
	createSnapshot(t, paths, "explore", now)
	// Add an extra file to the snapshot.
	snapDir := paths.snapshotDir("explore")
	_ = os.WriteFile(filepath.Join(snapDir, "extra.md"), []byte("extra content"), 0644)

	err := paths.InitializeFromSnapshot("explore", ScopeProject, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	memDir := paths.GetAgentMemoryDir("explore", ScopeProject)
	if _, err := os.Stat(filepath.Join(memDir, "extra.md")); os.IsNotExist(err) {
		t.Error("expected extra.md to be copied to local memory")
	}

	// Synced marker should exist.
	if _, err := os.Stat(paths.syncedJSONPath("explore", ScopeProject)); os.IsNotExist(err) {
		t.Error("expected .snapshot-synced.json to exist")
	}
}

func TestReplaceFromSnapshot(t *testing.T) {
	dir := t.TempDir()
	paths := &Paths{CWD: dir}

	// Set up local memory with an old file.
	memDir := paths.GetAgentMemoryDir("explore", ScopeProject)
	_ = EnsureAgentMemoryDir(memDir)
	_ = os.WriteFile(filepath.Join(memDir, "old.md"), []byte("old"), 0644)

	// Set up snapshot with a new file.
	now := time.Now().Format(time.RFC3339)
	createSnapshot(t, paths, "explore", now)
	snapDir := paths.snapshotDir("explore")
	_ = os.WriteFile(filepath.Join(snapDir, "new.md"), []byte("new"), 0644)

	err := paths.ReplaceFromSnapshot("explore", ScopeProject, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Old file should be removed.
	if _, err := os.Stat(filepath.Join(memDir, "old.md")); !os.IsNotExist(err) {
		t.Error("expected old.md to be removed")
	}
	// New file should exist.
	if _, err := os.Stat(filepath.Join(memDir, "new.md")); os.IsNotExist(err) {
		t.Error("expected new.md to be copied")
	}
}

func TestMarkSnapshotSynced(t *testing.T) {
	dir := t.TempDir()
	paths := &Paths{CWD: dir}

	now := time.Now().Format(time.RFC3339)
	err := paths.MarkSnapshotSynced("explore", ScopeProject, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	meta, err := paths.readSyncedMeta("explore", ScopeProject)
	if err != nil {
		t.Fatalf("read synced meta: %v", err)
	}
	if meta.SyncedFrom != now {
		t.Errorf("syncedFrom = %q, want %q", meta.SyncedFrom, now)
	}
}

func TestCopySnapshotToLocal_SkipsSnapshotJSON(t *testing.T) {
	dir := t.TempDir()
	paths := &Paths{CWD: dir}

	now := time.Now().Format(time.RFC3339)
	createSnapshot(t, paths, "explore", now)
	// snapshot.json should not be copied.
	snapDir := paths.snapshotDir("explore")
	_ = os.WriteFile(filepath.Join(snapDir, "note.md"), []byte("note"), 0644)

	err := paths.copySnapshotToLocal("explore", ScopeProject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	memDir := paths.GetAgentMemoryDir("explore", ScopeProject)
	if _, err := os.Stat(filepath.Join(memDir, "note.md")); os.IsNotExist(err) {
		t.Error("expected note.md to be copied")
	}
	// snapshot.json should NOT be in local memory.
	if _, err := os.Stat(filepath.Join(memDir, snapshotJSON)); !os.IsNotExist(err) {
		t.Error("expected snapshot.json to NOT be copied")
	}
}

// createSnapshot creates a snapshot.json for the given agent with the given timestamp.
func createSnapshot(t *testing.T, paths *Paths, agentType, timestamp string) {
	t.Helper()
	snapDir := paths.snapshotDir(agentType)
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		t.Fatalf("mkdir snapshot dir: %v", err)
	}
	meta := SnapshotMeta{UpdatedAt: timestamp}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(paths.snapshotJSONPath(agentType), data, 0644); err != nil {
		t.Fatalf("write snapshot.json: %v", err)
	}
}
