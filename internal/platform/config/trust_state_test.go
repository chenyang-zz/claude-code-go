package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTrustState_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	state, err := LoadTrustState(tmpDir)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if len(state.Projects) != 0 {
		t.Fatalf("expected empty projects, got %d", len(state.Projects))
	}
}

func TestLoadTrustState_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, TrustStateDir, TrustStateFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadTrustState(tmpDir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSaveAndLoadTrustState(t *testing.T) {
	tmpDir := t.TempDir()
	state := NewTrustState()
	AcceptTrust(state, "/home/user/project")

	if err := SaveTrustState(tmpDir, state); err != nil {
		t.Fatalf("save trust state: %v", err)
	}

	loaded, err := LoadTrustState(tmpDir)
	if err != nil {
		t.Fatalf("load trust state: %v", err)
	}

	if !IsTrustAccepted(loaded, "/home/user/project", "/home/user") {
		t.Fatal("expected /home/user/project to be trusted")
	}
}

func TestIsTrustAccepted_HomeDir(t *testing.T) {
	state := NewTrustState()
	if !IsTrustAccepted(state, "/home/user", "/home/user") {
		t.Fatal("expected home dir to be trusted")
	}
}

func TestIsTrustAccepted_EmptyDir(t *testing.T) {
	state := NewTrustState()
	if !IsTrustAccepted(state, "", "/home/user") {
		t.Fatal("expected empty dir to be trusted")
	}
}

func TestIsTrustAccepted_ParentInheritance(t *testing.T) {
	state := NewTrustState()
	AcceptTrust(state, "/home/user/workspace")

	if !IsTrustAccepted(state, "/home/user/workspace/project", "/home/user") {
		t.Fatal("expected child of trusted parent to be trusted")
	}
}

func TestIsTrustAccepted_NotTrusted(t *testing.T) {
	state := NewTrustState()
	if IsTrustAccepted(state, "/some/other/path", "/home/user") {
		t.Fatal("expected untrusted path to return false")
	}
}

func TestIsTrustAccepted_NilState(t *testing.T) {
	if IsTrustAccepted(nil, "/home/user/project", "/home/user") {
		t.Fatal("expected nil state to return false")
	}
}

func TestAcceptTrust_NormalizesPath(t *testing.T) {
	state := NewTrustState()
	AcceptTrust(state, "/home/user/project/")

	normalized := normalizeTrustPath("/home/user/project/")
	if _, ok := state.Projects[normalized]; !ok {
		t.Fatalf("expected normalized key %q in projects", normalized)
	}
}
