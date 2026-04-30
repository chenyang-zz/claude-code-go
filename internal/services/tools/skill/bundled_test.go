package skill

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

func TestRegisterBundledSkill_Basic(t *testing.T) {
	ClearBundledSkills()
	defer ClearBundledSkills()

	def := BundledSkillDefinition{
		Name:        "my-bundled-skill",
		Description: "A bundled skill for testing",
		WhenToUse:   "Use this for testing",
		GetPromptForCommand: func(args string) (string, error) {
			return "Bundled output for: " + args, nil
		},
	}
	RegisterBundledSkill(def)

	skills := GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	if s.name != "my-bundled-skill" {
		t.Fatalf("expected name 'my-bundled-skill', got %q", s.name)
	}
	if s.description != "A bundled skill for testing" {
		t.Fatalf("expected description, got %q", s.description)
	}
	if s.whenToUse != "Use this for testing" {
		t.Fatalf("expected whenToUse, got %q", s.whenToUse)
	}
	if s.source != "bundled" || s.loadedFrom != "bundled" {
		t.Fatalf("expected source/loadedFrom 'bundled', got %q/%q", s.source, s.loadedFrom)
	}
	if !s.userInvocable {
		t.Fatal("expected userInvocable to default to true")
	}
}

func TestGetBundledSkills_ReturnsCopy(t *testing.T) {
	ClearBundledSkills()
	defer ClearBundledSkills()

	RegisterBundledSkill(BundledSkillDefinition{
		Name: "skill-1",
		Description: "First",
		GetPromptForCommand: func(args string) (string, error) { return "1", nil },
	})

	skills := GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1, got %d", len(skills))
	}

	// Mutating the returned slice should not affect the registry.
	skills[0] = nil
	skills2 := GetBundledSkills()
	if len(skills2) != 1 || skills2[0] == nil {
		t.Fatal("GetBundledSkills should return a copy")
	}
}

func TestClearBundledSkills(t *testing.T) {
	ClearBundledSkills()

	RegisterBundledSkill(BundledSkillDefinition{
		Name: "temp",
		Description: "Temp skill",
		GetPromptForCommand: func(args string) (string, error) { return "temp", nil },
	})
	if len(GetBundledSkills()) != 1 {
		t.Fatal("expected 1 after register")
	}

	ClearBundledSkills()
	if len(GetBundledSkills()) != 0 {
		t.Fatal("expected 0 after clear")
	}
}

func TestRegisterBundledSkill_Execute(t *testing.T) {
	ClearBundledSkills()
	defer ClearBundledSkills()

	def := BundledSkillDefinition{
		Name:        "exec-test",
		Description: "Test execution",
		GetPromptForCommand: func(args string) (string, error) {
			return "Hello, " + args + "!", nil
		},
	}
	RegisterBundledSkill(def)

	skills := GetBundledSkills()
	s := skills[0]

	result, err := s.Execute(context.TODO(), command.Args{RawLine: "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "Hello, world!" {
		t.Fatalf("expected 'Hello, world!', got %q", result.Output)
	}
}

func TestRegisterBundledSkill_WithFiles(t *testing.T) {
	ClearBundledSkills()
	defer ClearBundledSkills()

	files := map[string]string{
		"script.sh": "#!/bin/sh\necho hello",
		"data/config.json": `{"key": "value"}`,
	}

	def := BundledSkillDefinition{
		Name:        "file-skill",
		Description: "Skill with files",
		Files:       files,
		GetPromptForCommand: func(args string) (string, error) {
			return "Files extracted", nil
		},
	}
	RegisterBundledSkill(def)

	skills := GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	if s.baseDir == "" {
		t.Fatal("expected baseDir to be set when files are present")
	}

	// Execute — files should be extracted on first call.
	result, err := s.Execute(context.TODO(), command.Args{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output == "" {
		t.Fatal("expected output")
	}

	// Verify files were extracted.
	extractDir := getBundledSkillExtractDir("file-skill")
	scriptPath := filepath.Join(extractDir, "script.sh")
	stat, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("expected extracted script.sh: %v", err)
	}
	if stat.IsDir() {
		t.Fatal("script.sh should be a file")
	}

	configPath := filepath.Join(extractDir, "data", "config.json")
	stat2, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("expected extracted config.json: %v", err)
	}
	if stat2.IsDir() {
		t.Fatal("config.json should be a file")
	}

	// Verify file content.
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read script.sh: %v", err)
	}
	if string(data) != "#!/bin/sh\necho hello" {
		t.Fatalf("unexpected script.sh content: %q", string(data))
	}
}

func TestRegisterBundledSkill_FilesExtractedOnce(t *testing.T) {
	ClearBundledSkills()
	defer ClearBundledSkills()

	files := map[string]string{
		"once.txt": "first write",
	}

	def := BundledSkillDefinition{
		Name:  "once-skill",
		Description: "Extract once",
		Files: files,
		GetPromptForCommand: func(args string) (string, error) {
			return "ok", nil
		},
	}
	RegisterBundledSkill(def)

	skills := GetBundledSkills()
	s := skills[0]

	// First execute extracts files.
	_, err := s.Execute(context.TODO(), command.Args{})
	if err != nil {
		t.Fatalf("first execute: %v", err)
	}

	// Delete the extracted file.
	extractDir := getBundledSkillExtractDir("once-skill")
	filePath := filepath.Join(extractDir, "once.txt")
	if err := os.Remove(filePath); err != nil {
		t.Fatalf("remove: %v", err)
	}

	// Second execute should NOT re-extract (extracted flag is set).
	_, err = s.Execute(context.TODO(), command.Args{})
	if err != nil {
		t.Fatalf("second execute: %v", err)
	}

	// File should still be gone (extraction happened only once).
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatal("file should not exist after second execute (extract-once)")
	}
}

func TestResolveSkillFilePath_RejectsAbsolute(t *testing.T) {
	_, err := resolveSkillFilePath("/tmp/base", "/etc/passwd")
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestResolveSkillFilePath_RejectsTraversal(t *testing.T) {
	_, err := resolveSkillFilePath("/tmp/base", "../outside")
	if err == nil {
		t.Fatal("expected error for traversal path")
	}

	_, err = resolveSkillFilePath("/tmp/base", "a/../../outside")
	if err == nil {
		t.Fatal("expected error for traversal with subdirectory")
	}
}

func TestResolveSkillFilePath_AllowsNormal(t *testing.T) {
	result, err := resolveSkillFilePath("/tmp/base", "scripts/helper.sh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join("/tmp/base", "scripts", "helper.sh")
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestSafeWriteFile_Excl(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	// First write succeeds.
	if err := safeWriteFile(path, "hello"); err != nil {
		t.Fatalf("first write: %v", err)
	}

	// Verify content.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected 'hello', got %q", string(data))
	}

	// Second write to same path should fail (O_EXCL).
	err = safeWriteFile(path, "world")
	if err == nil {
		t.Fatal("expected O_EXCL error on second write")
	}
}

