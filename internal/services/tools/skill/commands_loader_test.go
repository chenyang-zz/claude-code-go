package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSkillsFromCommandsDir_NoCommandsDir(t *testing.T) {
	tmpDir := t.TempDir()

	skills, loadErrors, err := LoadSkillsFromCommandsDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills for non-existent commands dir, got %d", len(skills))
	}
	if len(loadErrors) != 0 {
		t.Fatalf("expected 0 load errors, got %d", len(loadErrors))
	}
}

func TestLoadSkillsFromCommandsDir_SingleMdFile(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, ".claude", "commands")
	os.MkdirAll(commandsDir, 0o755)
	os.WriteFile(filepath.Join(commandsDir, "build.md"), []byte("# Build command\n\nThis is a build command skill."), 0o644)

	skills, _, err := LoadSkillsFromCommandsDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill from single .md file, got %d", len(skills))
	}

	s := skills[0]
	if s.name != "build" {
		t.Fatalf("expected command name 'build', got '%s'", s.name)
	}
	if s.loadedFrom != "commands_DEPRECATED" {
		t.Fatalf("expected loadedFrom 'commands_DEPRECATED', got '%s'", s.loadedFrom)
	}
	if s.baseDir != "" {
		t.Fatalf("expected empty baseDir for single .md file, got '%s'", s.baseDir)
	}
}

func TestLoadSkillsFromCommandsDir_SkillMdFormat(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, ".claude", "commands")
	skillDir := filepath.Join(commandsDir, "deploy")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: Deploy skill\n---\n\nDeploy the app."), 0o644)

	skills, _, err := LoadSkillsFromCommandsDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill from SKILL.md format, got %d", len(skills))
	}

	s := skills[0]
	if s.name != "deploy" {
		t.Fatalf("expected command name 'deploy', got '%s'", s.name)
	}
	if s.description != "Deploy skill" {
		t.Fatalf("expected frontmatter description, got '%s'", s.description)
	}
	if s.baseDir != skillDir {
		t.Fatalf("expected baseDir %s, got %s", skillDir, s.baseDir)
	}
}

func TestLoadSkillsFromCommandsDir_SkillMdTakeover(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, ".claude", "commands")
	skillDir := filepath.Join(commandsDir, "mytool")
	os.MkdirAll(skillDir, 0o755)

	// SKILL.md takes over the namespace
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill version"), 0o644)
	// Non-SKILL.md in same dir should be dropped
	os.WriteFile(filepath.Join(skillDir, "other.md"), []byte("# Other file"), 0o644)

	skills, _, err := LoadSkillsFromCommandsDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected only 1 skill (SKILL.md takeover), got %d", len(skills))
	}
	if skills[0].name != "mytool" {
		t.Fatalf("expected SKILL.md-based name 'mytool', got '%s'", skills[0].name)
	}
}

func TestLoadSkillsFromCommandsDir_Namespaced(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, ".claude", "commands")
	subDir := filepath.Join(commandsDir, "sub")
	os.MkdirAll(subDir, 0o755)
	os.WriteFile(filepath.Join(subDir, "run.md"), []byte("# Run command"), 0o644)

	skills, _, err := LoadSkillsFromCommandsDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 namespaced skill, got %d", len(skills))
	}
	if skills[0].name != "sub:run" {
		t.Fatalf("expected namespaced name 'sub:run', got '%s'", skills[0].name)
	}
}

func TestLoadSkillsFromCommandsDir_DeeplyNested(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, ".claude", "commands")
	deepDir := filepath.Join(commandsDir, "a", "b", "c")
	os.MkdirAll(deepDir, 0o755)
	os.WriteFile(filepath.Join(deepDir, "cmd.md"), []byte("# Deep command"), 0o644)

	skills, _, err := LoadSkillsFromCommandsDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 deeply nested skill, got %d", len(skills))
	}
	expectedName := "a:b:c:cmd"
	if skills[0].name != expectedName {
		t.Fatalf("expected '%s', got '%s'", expectedName, skills[0].name)
	}
}

func TestLoadSkillsFromCommandsDir_FrontmatterName(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, ".claude", "commands")
	os.MkdirAll(commandsDir, 0o755)
	os.WriteFile(filepath.Join(commandsDir, "default.md"), []byte("---\nname: custom-name\n---\n\nContent."), 0o644)

	skills, _, err := LoadSkillsFromCommandsDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].name != "custom-name" {
		t.Fatalf("expected frontmatter name 'custom-name', got '%s'", skills[0].name)
	}
}

func TestLoadSkillsFromCommandsDir_DescriptionFallback(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, ".claude", "commands")
	os.MkdirAll(commandsDir, 0o755)
	os.WriteFile(filepath.Join(commandsDir, "plain.md"), []byte("This is the body text of the command."), 0o644)

	skills, _, err := LoadSkillsFromCommandsDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill with fallback description, got %d", len(skills))
	}
	if !strings.Contains(skills[0].description, "body text") {
		t.Fatalf("expected description extracted from body, got '%s'", skills[0].description)
	}
}

func TestBuildCommandsNamespace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{".", ""},
		{"sub", "sub"},
		{"a/b", "a:b"},
		{"a/b/c", "a:b:c"},
	}

	for _, tc := range tests {
		result := buildCommandsNamespace(tc.input)
		if result != tc.expected {
			t.Errorf("buildCommandsNamespace(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestBuildCommandName(t *testing.T) {
	tests := []struct {
		namespace string
		baseName  string
		expected  string
	}{
		{"", "foo", "foo"},
		{"sub", "cmd", "sub:cmd"},
		{"a:b", "run", "a:b:run"},
	}

	for _, tc := range tests {
		result := buildCommandName(tc.namespace, tc.baseName)
		if result != tc.expected {
			t.Errorf("buildCommandName(%q, %q) = %q, want %q", tc.namespace, tc.baseName, result, tc.expected)
		}
	}
}
