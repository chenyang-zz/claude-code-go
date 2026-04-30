package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSeparateConditionalSkills_NoPaths(t *testing.T) {
	defer ClearConditionalSkills()

	s1 := NewSkill(SkillOptions{Name: "skill1", Description: "desc", Content: "body"})
	s2 := NewSkill(SkillOptions{Name: "skill2", Description: "desc", Content: "body"})

	result := SeparateConditionalSkills([]*Skill{s1, s2})

	if len(result) != 2 {
		t.Fatalf("expected 2 unconditional skills, got %d", len(result))
	}
	if GetConditionalSkillCount() != 0 {
		t.Fatalf("expected 0 conditional skills, got %d", GetConditionalSkillCount())
	}
}

func TestSeparateConditionalSkills_WithPaths(t *testing.T) {
	defer ClearConditionalSkills()

	s1 := NewSkill(SkillOptions{Name: "skill1", Description: "desc", Content: "body", Paths: []string{"*.go"}})
	s2 := NewSkill(SkillOptions{Name: "skill2", Description: "desc", Content: "body"})

	result := SeparateConditionalSkills([]*Skill{s1, s2})

	if len(result) != 1 {
		t.Fatalf("expected 1 unconditional skill, got %d", len(result))
	}
	if result[0].name != "skill2" {
		t.Fatalf("expected skill2 to be unconditional, got %s", result[0].name)
	}
	if GetConditionalSkillCount() != 1 {
		t.Fatalf("expected 1 conditional skill, got %d", GetConditionalSkillCount())
	}
}

func TestSeparateConditionalSkills_AllEmptyPaths(t *testing.T) {
	defer ClearConditionalSkills()

	s1 := NewSkill(SkillOptions{Name: "skill1", Description: "desc", Content: "body", Paths: nil})
	s2 := NewSkill(SkillOptions{Name: "skill2", Description: "desc", Content: "body", Paths: []string{}})

	result := SeparateConditionalSkills([]*Skill{s1, s2})

	if len(result) != 2 {
		t.Fatalf("expected 2 unconditional skills, got %d", len(result))
	}
	if GetConditionalSkillCount() != 0 {
		t.Fatalf("expected 0 conditional skills, got %d", GetConditionalSkillCount())
	}
}

func TestActivateConditionalSkills_NoMatch(t *testing.T) {
	defer ClearDynamicSkills()

	s1 := NewSkill(SkillOptions{Name: "cond1", Description: "desc", Content: "body", Paths: []string{"*.go"}})
	_ = SeparateConditionalSkills([]*Skill{s1})

	activated := ActivateConditionalSkillsForPaths([]string{"README.md"}, "/project")

	if len(activated) != 0 {
		t.Fatalf("expected 0 activated, got %d", len(activated))
	}
	if GetConditionalSkillCount() != 1 {
		t.Fatalf("expected 1 conditional skill remaining, got %d", GetConditionalSkillCount())
	}
}

func TestActivateConditionalSkills_Match(t *testing.T) {
	defer ClearDynamicSkills()

	s1 := NewSkill(SkillOptions{Name: "cond1", Description: "desc", Content: "body", Paths: []string{"*.go"}})
	_ = SeparateConditionalSkills([]*Skill{s1})

	activated := ActivateConditionalSkillsForPaths([]string{"main.go"}, "/project")

	if len(activated) != 1 {
		t.Fatalf("expected 1 activated, got %d", len(activated))
	}
	if activated[0] != "cond1" {
		t.Fatalf("expected cond1 to be activated, got %s", activated[0])
	}
	if GetConditionalSkillCount() != 0 {
		t.Fatalf("expected 0 conditional skills remaining, got %d", GetConditionalSkillCount())
	}
	// Check it was moved to dynamicSkills
	if _, exists := dynamicSkills["cond1"]; !exists {
		t.Fatal("expected cond1 to be in dynamicSkills after activation")
	}
}

func TestActivateConditionalSkills_WildcardPattern(t *testing.T) {
	defer ClearDynamicSkills()

	s1 := NewSkill(SkillOptions{Name: "src_skill", Description: "desc", Content: "body", Paths: []string{"src/**"}})
	_ = SeparateConditionalSkills([]*Skill{s1})

	// Match file deep under src/
	activated := ActivateConditionalSkillsForPaths([]string{"src/internal/pkg/main.go"}, "/project")
	if len(activated) != 1 {
		t.Fatalf("expected activation for src/** matching deep file, got %d", len(activated))
	}
}

func TestActivateConditionalSkills_AlreadyActivated(t *testing.T) {
	defer ClearDynamicSkills()

	s1 := NewSkill(SkillOptions{Name: "cond1", Description: "desc", Content: "body", Paths: []string{"*.go"}})
	_ = SeparateConditionalSkills([]*Skill{s1})

	// First activation
	ActivateConditionalSkillsForPaths([]string{"main.go"}, "/project")
	// Now create another skill with the same name and separate again
	s2 := NewSkill(SkillOptions{Name: "cond1", Description: "desc2", Content: "body2", Paths: []string{"*.go"}})
	result := SeparateConditionalSkills([]*Skill{s2})

	// Should be returned as unconditional because the name is permanently activated
	if len(result) != 1 {
		t.Fatalf("expected 1 unconditional (permanently activated), got %d", len(result))
	}
}

func TestActivateConditionalSkills_AbsolutePath(t *testing.T) {
	defer ClearDynamicSkills()

	s1 := NewSkill(SkillOptions{Name: "cond1", Description: "desc", Content: "body", Paths: []string{"*.go"}})
	_ = SeparateConditionalSkills([]*Skill{s1})

	cwd, _ := os.Getwd()
	absPath := filepath.Join(cwd, "main.go")

	activated := ActivateConditionalSkillsForPaths([]string{absPath}, cwd)
	if len(activated) != 1 {
		t.Fatalf("expected 1 activated for absolute path, got %d", len(activated))
	}
}

func TestActivateConditionalSkills_EscapedPath(t *testing.T) {
	defer ClearDynamicSkills()

	s1 := NewSkill(SkillOptions{Name: "cond1", Description: "desc", Content: "body", Paths: []string{"*.go"}})
	_ = SeparateConditionalSkills([]*Skill{s1})

	// Path outside cwd should not match
	activated := ActivateConditionalSkillsForPaths([]string{"../outside/main.go"}, "/project")
	if len(activated) != 0 {
		t.Fatalf("expected 0 activated for escaped path, got %d", len(activated))
	}
}

func TestActivateConditionalSkills_EmptyConditionalStore(t *testing.T) {
	defer ClearDynamicSkills()

	activated := ActivateConditionalSkillsForPaths([]string{"main.go"}, "/project")
	if activated != nil {
		t.Fatalf("expected nil for empty conditional store, got %v", len(activated))
	}
}

func TestClearConditionalSkills(t *testing.T) {
	s1 := NewSkill(SkillOptions{Name: "cond1", Description: "desc", Content: "body", Paths: []string{"*.go"}})
	_ = SeparateConditionalSkills([]*Skill{s1})

	if GetConditionalSkillCount() != 1 {
		t.Fatal("expected 1 conditional skill before clear")
	}

	ClearConditionalSkills()

	if GetConditionalSkillCount() != 0 {
		t.Fatal("expected 0 conditional skills after clear")
	}
}

func TestMatchGlobSegments_Exact(t *testing.T) {
	if !matchGlobSegments(splitPathSegments("foo/bar"), splitPathSegments("foo/bar")) {
		t.Fatal("expected exact match")
	}
}

func TestMatchGlobSegments_Star(t *testing.T) {
	if !matchGlobSegments(splitPathSegments("foo/bar.go"), splitPathSegments("foo/*.go")) {
		t.Fatal("expected * to match")
	}
	if matchGlobSegments(splitPathSegments("foo/bar/baz.go"), splitPathSegments("foo/*.go")) {
		t.Fatal("expected * not to match across directory")
	}
}

func TestMatchGlobSegments_DoubleStar(t *testing.T) {
	if !matchGlobSegments(splitPathSegments("foo/bar/baz.go"), splitPathSegments("foo/**")) {
		t.Fatal("expected ** to match nested path")
	}
	if !matchGlobSegments(splitPathSegments("src/a/b/c.go"), splitPathSegments("src/**")) {
		t.Fatal("expected ** to match deep nested path")
	}
}

func TestMatchGlobSegments_Question(t *testing.T) {
	if !matchGlobSegments(splitPathSegments("foo/a.go"), splitPathSegments("foo/?.go")) {
		t.Fatal("expected ? to match single char")
	}
	if matchGlobSegments(splitPathSegments("foo/ab.go"), splitPathSegments("foo/?.go")) {
		t.Fatal("expected ? not to match two chars")
	}
}

func TestMatchSkillPath_MultiplePatterns(t *testing.T) {
	patterns := []string{"*.go", "*.ts"}
	if !matchSkillPath("main.go", patterns) {
		t.Fatal("expected match for *.go")
	}
	if !matchSkillPath("app.ts", patterns) {
		t.Fatal("expected match for *.ts")
	}
	if matchSkillPath("README.md", patterns) {
		t.Fatal("expected no match for README.md")
	}
}
