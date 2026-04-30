package skill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

func TestLoadProjectSkills_EmptyDir(t *testing.T) {
	projectDir := t.TempDir()
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	skills, errors, err := LoadProjectSkills(projectDir, projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills in empty dir, got %d", len(skills))
	}
	if len(errors) != 0 {
		t.Fatalf("expected 0 load errors, got %d", len(errors))
	}
}

func TestLoadProjectSkills_NoSkillsDir(t *testing.T) {
	projectDir := t.TempDir()

	skills, errors, err := LoadProjectSkills(projectDir, projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(skills))
	}
	if len(errors) != 0 {
		t.Fatalf("expected 0 load errors, got %d", len(errors))
	}
}

func TestLoadProjectSkills_SingleSkill(t *testing.T) {
	projectDir := t.TempDir()
	skillDir := filepath.Join(projectDir, ".claude", "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
name: My Skill
description: A test skill
when_to_use: Use this when testing
---
# My Skill

This is the skill content.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, errors, err := LoadProjectSkills(projectDir, projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errors) != 0 {
		t.Fatalf("unexpected load errors: %v", errors)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	s := skills[0]
	meta := s.Metadata()
	if meta.Name != "My Skill" {
		t.Fatalf("expected name 'My Skill', got %q", meta.Name)
	}
	if meta.Description != "A test skill" {
		t.Fatalf("expected description 'A test skill', got %q", meta.Description)
	}
	if meta.Usage != "/My Skill" {
		t.Fatalf("expected usage '/My Skill', got %q", meta.Usage)
	}

	// Test execution
	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("unexpected execute error: %v", err)
	}
	if !strings.Contains(result.Output, "This is the skill content.") {
		t.Fatalf("expected skill content in output, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Base directory for this skill:") {
		t.Fatalf("expected base dir in output, got: %s", result.Output)
	}
}

func TestLoadProjectSkills_MultipleSkills(t *testing.T) {
	projectDir := t.TempDir()
	for _, name := range []string{"skill-a", "skill-b", "skill-c"} {
		skillDir := filepath.Join(projectDir, ".claude", "skills", name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\ndescription: Skill " + name + "\n---\n# " + name + "\nContent for " + name + ".\n"
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	skills, errors, err := LoadProjectSkills(projectDir, projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errors) != 0 {
		t.Fatalf("unexpected load errors: %v", errors)
	}
	if len(skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(skills))
	}
}

func TestLoadProjectSkills_NoFrontmatter(t *testing.T) {
	projectDir := t.TempDir()
	skillDir := filepath.Join(projectDir, ".claude", "skills", "plain-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// No frontmatter — directory name should be used as skill name.
	content := "This is a plain skill without frontmatter.\n\nIt has multiple paragraphs."
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, errors, err := LoadProjectSkills(projectDir, projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errors) != 0 {
		t.Fatalf("unexpected load errors: %v", errors)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	meta := skills[0].Metadata()
	if meta.Name != "plain-skill" {
		t.Fatalf("expected name from directory, got %q", meta.Name)
	}
	// Description should be extracted from first paragraph.
	if !strings.Contains(meta.Description, "plain skill without frontmatter") {
		t.Fatalf("expected description extracted from content, got %q", meta.Description)
	}
}

func TestLoadProjectSkills_MissingSkillMd(t *testing.T) {
	projectDir := t.TempDir()
	// Subdirectory without SKILL.md should be skipped.
	skillDir := filepath.Join(projectDir, ".claude", "skills", "empty-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Only put some other file.
	if err := os.WriteFile(filepath.Join(skillDir, "README.md"), []byte("not a skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, errors, err := LoadProjectSkills(projectDir, projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errors) != 0 {
		t.Fatalf("unexpected load errors: %v", errors)
	}
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills (no SKILL.md), got %d", len(skills))
	}
}

func TestLoadProjectSkills_SkipsFiles(t *testing.T) {
	projectDir := t.TempDir()
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Put a regular file directly in skills/ — should be skipped (not a directory).
	if err := os.WriteFile(filepath.Join(skillsDir, "standalone.md"), []byte("not loaded"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, _, err := LoadProjectSkills(projectDir, projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills (single .md files not supported), got %d", len(skills))
	}
}

func TestLoadProjectSkills_CLAUDE_SKILL_DIR_Substitution(t *testing.T) {
	projectDir := t.TempDir()
	skillDir := filepath.Join(projectDir, ".claude", "skills", "scripted-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ndescription: Script skill\n---\nRun `${CLAUDE_SKILL_DIR}/run.sh` to execute.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, _, err := LoadProjectSkills(projectDir, projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	result, err := skills[0].Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("unexpected execute error: %v", err)
	}

	// ${CLAUDE_SKILL_DIR} should be replaced with the actual skill directory.
	expectedSlash := filepath.ToSlash(skillDir)
	if !strings.Contains(result.Output, expectedSlash+"/run.sh") {
		t.Fatalf("expected CLAUDE_SKILL_DIR substitution, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Base directory for this skill:") {
		t.Fatalf("expected base dir line, got: %s", result.Output)
	}
}

func TestRegisterSkills_AllNew(t *testing.T) {
	registry := command.NewInMemoryRegistry()
	skills := []*Skill{
		{name: "skill-a", description: "First skill", content: "a"},
		{name: "skill-b", description: "Second skill", content: "b"},
	}

	registered := RegisterSkills(registry, skills, "projectSettings")
	if registered != 2 {
		t.Fatalf("expected 2 registered, got %d", registered)
	}

	if _, ok := registry.Get("skill-a"); !ok {
		t.Fatal("expected skill-a in registry")
	}
	if _, ok := registry.Get("skill-b"); !ok {
		t.Fatal("expected skill-b in registry")
	}
}

func TestRegisterSkills_SkipsConflicts(t *testing.T) {
	registry := command.NewInMemoryRegistry()
	// Register a built-in command first.
	if err := registry.Register(&Skill{name: "skill-a", description: "Built-in", content: "built-in"}); err != nil {
		t.Fatal(err)
	}

	skills := []*Skill{
		{name: "skill-a", description: "Should be skipped", content: "a"},
		{name: "skill-b", description: "Should register", content: "b"},
	}

	registered := RegisterSkills(registry, skills, "projectSettings")
	if registered != 1 {
		t.Fatalf("expected 1 registered (skill-a should be skipped), got %d", registered)
	}

	cmd, ok := registry.Get("skill-a")
	if !ok {
		t.Fatal("expected skill-a to still exist")
	}
	// Should be the original built-in.
	if cmd.Metadata().Description != "Built-in" {
		t.Fatalf("expected built-in skill-a to be preserved, got %q", cmd.Metadata().Description)
	}
}

func TestRegisterSkills_NilRegistry(t *testing.T) {
	skills := []*Skill{
		{name: "skill-a", description: "test", content: "test"},
	}
	registered := RegisterSkills(nil, skills, "test")
	if registered != 0 {
		t.Fatalf("expected 0 registered with nil registry, got %d", registered)
	}
}

func TestRegisterSkills_EmptySkills(t *testing.T) {
	registry := command.NewInMemoryRegistry()
	registered := RegisterSkills(registry, nil, "test")
	if registered != 0 {
		t.Fatalf("expected 0 registered, got %d", registered)
	}
}

func TestSkillExecute_NoBaseDir(t *testing.T) {
	s := &Skill{
		name:        "test",
		description: "test",
		content:     "test content",
		// baseDir is empty
	}
	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "test content" {
		t.Fatalf("expected plain content without base dir, got: %s", result.Output)
	}
	if strings.Contains(result.Output, "Base directory") {
		t.Fatal("expected no base dir prefix")
	}
}

func TestExtractDescription_FirstParagraph(t *testing.T) {
	desc := extractDescription("This is the first paragraph.\n\nThis is the second paragraph.", "fallback")
	if desc != "This is the first paragraph." {
		t.Fatalf("expected first paragraph, got %q", desc)
	}
}

func TestExtractDescription_SkipsHeadings(t *testing.T) {
	desc := extractDescription("# Title\n\nFirst real paragraph.", "fallback")
	if desc != "First real paragraph." {
		t.Fatalf("expected paragraph after heading, got %q", desc)
	}
}

func TestExtractDescription_Fallback(t *testing.T) {
	desc := extractDescription("", "fallback-name")
	if desc != "Skill fallback-name" {
		t.Fatalf("expected fallback, got %q", desc)
	}
}

func TestLoadProjectSkills_InvalidYamlFrontmatter(t *testing.T) {
	projectDir := t.TempDir()
	skillDir := filepath.Join(projectDir, ".claude", "skills", "bad-yaml")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ninvalid: [unclosed\n---\nBody."
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, errors, err := LoadProjectSkills(projectDir, projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills (invalid YAML skipped), got %d", len(skills))
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 load error, got %d", len(errors))
	}
}

func TestLoadUserSkills(t *testing.T) {
	homeDir := t.TempDir()
	skillDir := filepath.Join(homeDir, ".claude", "skills", "user-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ndescription: A user skill\n---\n# User Skill\nContent.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, errors, err := LoadUserSkills(homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errors) != 0 {
		t.Fatalf("unexpected load errors: %v", errors)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].source != "userSettings" {
		t.Fatalf("expected source 'userSettings', got %q", skills[0].source)
	}
}

func TestLoadManagedSkills(t *testing.T) {
	homeDir := t.TempDir()
	skillDir := filepath.Join(homeDir, ".claude", "managed", ".claude", "skills", "managed-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ndescription: A managed skill\n---\n# Managed\nContent.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, errors, err := LoadManagedSkills(homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errors) != 0 {
		t.Fatalf("unexpected load errors: %v", errors)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].source != "policySettings" {
		t.Fatalf("expected source 'policySettings', got %q", skills[0].source)
	}
}

func TestLoadProjectSkills_WithParentTraversal(t *testing.T) {
	homeDir := t.TempDir()
	projectDir := filepath.Join(homeDir, "projects", "myapp")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projSkillDir := filepath.Join(projectDir, ".claude", "skills", "project-skill")
	if err := os.MkdirAll(projSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projSkillDir, "SKILL.md"), []byte("---\ndescription: Project\n---\nContent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	parentDir := filepath.Join(homeDir, "projects")
	parentSkillDir := filepath.Join(parentDir, ".claude", "skills", "parent-skill")
	if err := os.MkdirAll(parentSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parentSkillDir, "SKILL.md"), []byte("---\ndescription: Parent\n---\nContent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, errors, err := LoadProjectSkills(projectDir, homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errors) != 0 {
		t.Fatalf("unexpected load errors: %v", errors)
	}
	if len(skills) < 2 {
		t.Fatalf("expected at least 2 skills, got %d", len(skills))
	}
	names := make(map[string]bool)
	for _, s := range skills {
		names[s.name] = true
	}
	if !names["project-skill"] || !names["parent-skill"] {
		t.Fatalf("expected both project-skill and parent-skill, got %v", names)
	}
}

func TestLoadAdditionalDirSkills(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, ".claude", "skills", "extra-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: Extra\n---\nContent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, errors, err := LoadAdditionalDirSkills([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errors) != 0 {
		t.Fatalf("unexpected load errors: %v", errors)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].name != "extra-skill" {
		t.Fatalf("expected name 'extra-skill', got %q", skills[0].name)
	}
}

func TestLoadSkill_AllFrontmatterFields(t *testing.T) {
	projectDir := t.TempDir()
	skillDir := filepath.Join(projectDir, ".claude", "skills", "full-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: Full Skill Display\ndescription: A skill with all fields\nwhen_to_use: Use when testing\nversion: \"1.0\"\nallowed-tools: bash, read, edit\nargument-hint: \"<file>\"\narguments: input, output\nmodel: claude-sonnet-4-6\ndisable-model-invocation: true\nuser-invocable: false\ncontext: fork\nagent: test-agent\npaths:\n  - \"*.go\"\n  - \"internal/**\"\neffort: 3\n---\n# Full Skill\nContent body.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, errors, err := LoadProjectSkills(projectDir, projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errors) != 0 {
		t.Fatalf("unexpected load errors: %v", errors)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	s := skills[0]
	if s.displayName != "Full Skill Display" {
		t.Fatalf("expected displayName, got %q", s.displayName)
	}
	if !s.hasUserSpecifiedDescription {
		t.Fatal("expected hasUserSpecifiedDescription to be true")
	}
	if s.whenToUse != "Use when testing" {
		t.Fatalf("expected whenToUse, got %q", s.whenToUse)
	}
	if s.version != "1.0" {
		t.Fatalf("expected version, got %q", s.version)
	}
	if len(s.allowedTools) != 3 {
		t.Fatalf("expected 3 allowedTools, got %d", len(s.allowedTools))
	}
	if s.argumentHint != "<file>" {
		t.Fatalf("expected argumentHint, got %q", s.argumentHint)
	}
	if len(s.argumentNames) != 2 {
		t.Fatalf("expected 2 argumentNames, got %d", len(s.argumentNames))
	}
	if s.model != "claude-sonnet-4-6" {
		t.Fatalf("expected model, got %q", s.model)
	}
	if !s.disableModelInvocation {
		t.Fatal("expected disableModelInvocation to be true")
	}
	if s.userInvocable {
		t.Fatal("expected userInvocable to be false")
	}
	if s.executionContext != "fork" {
		t.Fatalf("expected executionContext 'fork', got %q", s.executionContext)
	}
	if s.agent != "test-agent" {
		t.Fatalf("expected agent, got %q", s.agent)
	}
	if len(s.paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(s.paths))
	}
	if s.effort != "3" {
		t.Fatalf("expected effort '3', got %q", s.effort)
	}
}

func TestLoadSkill_ModelInherit(t *testing.T) {
	projectDir := t.TempDir()
	skillDir := filepath.Join(projectDir, ".claude", "skills", "inherit-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nmodel: inherit\n---\nContent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, _, _ := LoadProjectSkills(projectDir, projectDir)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].model != "" {
		t.Fatalf("expected empty model for 'inherit', got %q", skills[0].model)
	}
}

func TestLoadSkill_UserInvocableDefaultsTrue(t *testing.T) {
	projectDir := t.TempDir()
	skillDir := filepath.Join(projectDir, ".claude", "skills", "default-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: Default\n---\nContent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, _, _ := LoadProjectSkills(projectDir, projectDir)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if !skills[0].userInvocable {
		t.Fatal("expected userInvocable to default to true")
	}
}

func TestLoadSkill_PathsFilterMatchAll(t *testing.T) {
	projectDir := t.TempDir()
	skillDir := filepath.Join(projectDir, ".claude", "skills", "paths-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: Paths skill\npaths:\n  - \"**\"\n  - \"**/**\"\n---\nContent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, _, _ := LoadProjectSkills(projectDir, projectDir)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if len(skills[0].paths) != 0 {
		t.Fatalf("expected 0 paths (match-all filtered), got %v", skills[0].paths)
	}
}

func TestDeduplicateByPath_NoDuplicates(t *testing.T) {
	skills := []*Skill{
		{name: "a", baseDir: "/tmp/skills/a"},
		{name: "b", baseDir: "/tmp/skills/b"},
	}
	result := DeduplicateByPath(skills)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

func TestDeduplicateByPath_RemovesDuplicates(t *testing.T) {
	skills := []*Skill{
		{name: "a", baseDir: "/tmp/skills/a", source: "projectSettings"},
		{name: "a-dup", baseDir: "/tmp/skills/a", source: "userSettings"},
	}
	result := DeduplicateByPath(skills)
	if len(result) != 1 {
		t.Fatalf("expected 1 after dedup, got %d", len(result))
	}
	if result[0].source != "projectSettings" {
		t.Fatalf("expected first skill to win, got source %q", result[0].source)
	}
}

func TestDeduplicateByPath_FirstWins(t *testing.T) {
	skills := []*Skill{
		{name: "first", baseDir: t.TempDir(), source: "userSettings"},
		{name: "second", baseDir: t.TempDir(), source: "projectSettings"},
	}
	skills[1].baseDir = skills[0].baseDir

	result := DeduplicateByPath(skills)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0].name != "first" {
		t.Fatalf("expected first skill, got %q", result[0].name)
	}
}

func TestParseEffort_Numeric(t *testing.T) {
	fm := map[string]any{"effort": float64(3)}
	if result := parseEffort(fm, "effort"); result != "3" {
		t.Fatalf("expected '3', got %q", result)
	}
}

func TestParseEffort_Label(t *testing.T) {
	fm := map[string]any{"effort": "high"}
	if result := parseEffort(fm, "effort"); result != "3" {
		t.Fatalf("expected '3' for HIGH, got %q", result)
	}
}

func TestParseEffort_ClampLow(t *testing.T) {
	fm := map[string]any{"effort": float64(0)}
	if result := parseEffort(fm, "effort"); result != "1" {
		t.Fatalf("expected '1' (clamped from 0), got %q", result)
	}
}

func TestParseEffort_ClampHigh(t *testing.T) {
	fm := map[string]any{"effort": float64(10)}
	if result := parseEffort(fm, "effort"); result != "5" {
		t.Fatalf("expected '5' (clamped from 10), got %q", result)
	}
}

func TestToBool_Variants(t *testing.T) {
	if toBool(map[string]any{"flag": true}, "flag") != true {
		t.Fatal("expected true for bool true")
	}
	if toBool(map[string]any{"flag": "true"}, "flag") != true {
		t.Fatal("expected true for string 'true'")
	}
	if toBool(map[string]any{"flag": "TRUE"}, "flag") != true {
		t.Fatal("expected true for string 'TRUE'")
	}
	if toBool(map[string]any{"flag": false}, "flag") != false {
		t.Fatal("expected false for bool false")
	}
	if toBool(map[string]any{}, "flag") != false {
		t.Fatal("expected false for missing key")
	}
	if toBool(nil, "flag") != false {
		t.Fatal("expected false for nil map")
	}
}

func TestParseSlashCommandTools_String(t *testing.T) {
	fm := map[string]any{"allowed-tools": "bash, read, edit"}
	result := parseSlashCommandTools(fm, "allowed-tools")
	if len(result) != 3 {
		t.Fatalf("expected 3 tools, got %d: %v", len(result), result)
	}
}

func TestParseSlashCommandTools_StringSlice(t *testing.T) {
	fm := map[string]any{"allowed-tools": []string{"bash", "read"}}
	result := parseSlashCommandTools(fm, "allowed-tools")
	if len(result) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result))
	}
}
