package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverSkillDirsForPaths_NoFiles(t *testing.T) {
	defer ClearDynamicSkills()

	result := DiscoverSkillDirsForPaths(nil, "/project")
	if len(result) != 0 {
		t.Fatalf("expected 0 discovered dirs for no files, got %d", len(result))
	}
}

func TestDiscoverSkillDirsForPaths_FileWithoutSkills(t *testing.T) {
	defer ClearDynamicSkills()

	// Create a temp directory structure: /tmp/.../project/sub/file.txt
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	subDir := filepath.Join(projectDir, "sub")
	os.MkdirAll(subDir, 0o755)
	os.WriteFile(filepath.Join(subDir, "file.txt"), []byte("test"), 0o644)

	result := DiscoverSkillDirsForPaths([]string{filepath.Join(subDir, "file.txt")}, projectDir)

	if len(result) != 0 {
		t.Fatalf("expected 0 discovered dirs when no .claude/skills/ exists, got %d", len(result))
	}
	// The sub directory should be recorded as checked
	if _, checked := dynamicSkillDirs[filepath.Join(subDir, ".claude", "skills")]; !checked {
		t.Fatal("expected sub dir skills path to be recorded as checked")
	}
}

func TestDiscoverSkillDirsForPaths_WithSkillsDir(t *testing.T) {
	defer ClearDynamicSkills()

	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	subDir := filepath.Join(projectDir, "sub")
	skillsDir := filepath.Join(subDir, ".claude", "skills")
	os.MkdirAll(skillsDir, 0o755)

	result := DiscoverSkillDirsForPaths([]string{filepath.Join(subDir, "file.txt")}, projectDir)

	if len(result) != 1 {
		t.Fatalf("expected 1 discovered dir, got %d", len(result))
	}
	if result[0] != skillsDir {
		t.Fatalf("expected %s, got %s", skillsDir, result[0])
	}
}

func TestDiscoverSkillDirsForPaths_AlreadyChecked(t *testing.T) {
	defer ClearDynamicSkills()

	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	subDir := filepath.Join(projectDir, "sub")
	skillsDir := filepath.Join(subDir, ".claude", "skills")
	os.MkdirAll(skillsDir, 0o755)

	// First call records the check
	DiscoverSkillDirsForPaths([]string{filepath.Join(subDir, "file.txt")}, projectDir)
	// Second call should not rediscover
	result := DiscoverSkillDirsForPaths([]string{filepath.Join(subDir, "file.txt")}, projectDir)

	if len(result) != 0 {
		t.Fatalf("expected 0 rediscovered dirs, got %d", len(result))
	}
}

func TestDiscoverSkillDirsForPaths_NotEmpty_NotBelowCwd(t *testing.T) {
	defer ClearDynamicSkills()

	result := DiscoverSkillDirsForPaths([]string{"/outside/file.txt"}, "/project")

	if len(result) != 0 {
		t.Fatalf("expected 0 discovered dirs for path outside cwd, got %d", len(result))
	}
}

func TestAddSkillDirectories_Empty(t *testing.T) {
	defer ClearDynamicSkills()

	skills, err := AddSkillDirectories(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills for empty dirs, got %d", len(skills))
	}
}

func TestAddSkillDirectories_LoadsSkills(t *testing.T) {
	defer ClearDynamicSkills()

	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, ".claude", "skills")
	skillDir := filepath.Join(skillsDir, "mytest")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Test Skill\n\nThis is a test skill."), 0o644)

	skills, err := AddSkillDirectories([]string{skillsDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) < 1 {
		t.Fatalf("expected at least 1 loaded skill, got %d", len(skills))
	}

	// Check dynamicSkills was populated
	dynSkills := GetDynamicSkills()
	if len(dynSkills) < 1 {
		t.Fatal("expected dynamic skills to be populated")
	}
}

func TestAddSkillDirectories_DuplicateRemoval(t *testing.T) {
	defer ClearDynamicSkills()

	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, ".claude", "skills")
	skillDir := filepath.Join(skillsDir, "mytest")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test\n\nContent."), 0o644)

	// Load same directory twice
	AddSkillDirectories([]string{skillsDir})
	skills, err := AddSkillDirectories([]string{skillsDir})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = skills // second load still returns skills but they overwrite in the map
	dynSkills := GetDynamicSkills()
	if len(dynSkills) != 1 {
		t.Fatalf("expected 1 unique dynamic skill after duplicate load, got %d", len(dynSkills))
	}
}

func TestOnDynamicSkillsLoaded(t *testing.T) {
	defer ClearDynamicSkills()

	called := false
	unsub := OnDynamicSkillsLoaded(func() {
		called = true
	})
	defer unsub()

	emitSkillsLoaded()

	if !called {
		t.Fatal("expected callback to be called")
	}
}

func TestOnDynamicSkillsLoaded_Unsubscribe(t *testing.T) {
	defer ClearDynamicSkills()

	count := 0
	cb := func() { count++ }
	unsub := OnDynamicSkillsLoaded(cb)

	emitSkillsLoaded()
	if count != 1 {
		t.Fatalf("expected 1 callback invocation, got %d", count)
	}

	unsub()
	emitSkillsLoaded()
	if count != 1 {
		t.Fatalf("expected count to remain 1 after unsubscribe, got %d", count)
	}
}

func TestClearDynamicSkills(t *testing.T) {
	// Register a callback then clear; callback list should be emptied
	called := false
	OnDynamicSkillsLoaded(func() { called = true })
	ClearDynamicSkills()

	emitSkillsLoaded()
	if called {
		t.Fatal("expected callbacks to be cleared")
	}
}

func TestDiscoverSkillDirsForPaths_DeepestFirst(t *testing.T) {
	defer ClearDynamicSkills()

	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	deepDir := filepath.Join(projectDir, "a", "b")
	shallowDir := filepath.Join(projectDir, "a")

	os.MkdirAll(filepath.Join(deepDir, ".claude", "skills"), 0o755)
	os.MkdirAll(filepath.Join(shallowDir, ".claude", "skills"), 0o755)

	result := DiscoverSkillDirsForPaths([]string{filepath.Join(deepDir, "file.txt")}, projectDir)

	if len(result) != 2 {
		t.Fatalf("expected 2 discovered dirs, got %d", len(result))
	}
	// Deepest first
	if result[0] != filepath.Join(deepDir, ".claude", "skills") {
		t.Fatalf("expected deepest dir first, got %s", result[0])
	}
	if result[1] != filepath.Join(shallowDir, ".claude", "skills") {
		t.Fatalf("expected shallow dir last, got %s", result[1])
	}
}
