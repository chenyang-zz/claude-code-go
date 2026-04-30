package skill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	agentloader "github.com/sheepzhao/claude-code-go/internal/services/tools/agent/loader"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Skill implements command.Command for a loaded skill definition.
// It stores frontmatter metadata, markdown body, and source/behaviour fields
// matching the TS SkillCommand type. Fields are unexported; use the constructor
// or loader functions to create instances.
type Skill struct {
	// Basic identity
	name        string
	displayName string
	description string
	// hasUserSpecifiedDescription tracks whether the description was written
	// explicitly in frontmatter (true) or auto-extracted from the markdown body.
	hasUserSpecifiedDescription bool
	whenToUse                  string
	version                    string
	content                    string
	baseDir                    string

	// Arguments and tools
	allowedTools  []string
	argumentHint  string
	argumentNames []string
	model         string

	// Behaviour controls
	disableModelInvocation bool
	userInvocable          bool

	// Advanced features — parsed and stored, runtime consumption deferred
	hooks           map[string]any
	executionContext string // "fork" or ""
	agent           string
	paths           []string
	effort          string
	shell           map[string]any

	// Source tracking
	source     string // "userSettings", "projectSettings", "policySettings", "bundled"
	loadedFrom string // "skills", "bundled", "commands_DEPRECATED"

	// bundledPrompt is an optional override set by RegisterBundledSkill when
	// the skill provides a custom GetPromptForCommand function.
	bundledPrompt func(args string) (string, error)
}

// NewSkill creates a Skill with the required fields and sensible defaults.
// This is the primary constructor for both file-based and bundled skills.
func NewSkill(opts SkillOptions) *Skill {
	if opts.UserInvocable == nil {
		t := true
		opts.UserInvocable = &t
	}
	s := &Skill{
		name:                       opts.Name,
		displayName:                opts.DisplayName,
		description:                opts.Description,
		hasUserSpecifiedDescription: opts.HasUserSpecifiedDescription,
		whenToUse:                  opts.WhenToUse,
		version:                    opts.Version,
		content:                    opts.Content,
		baseDir:                    opts.BaseDir,
		allowedTools:               opts.AllowedTools,
		argumentHint:               opts.ArgumentHint,
		argumentNames:              opts.ArgumentNames,
		model:                      opts.Model,
		disableModelInvocation:     opts.DisableModelInvocation,
		userInvocable:              *opts.UserInvocable,
		hooks:                      opts.Hooks,
		executionContext:           opts.ExecutionContext,
		agent:                      opts.Agent,
		paths:                      opts.Paths,
		effort:                     opts.Effort,
		shell:                      opts.Shell,
		source:                     opts.Source,
		loadedFrom:                 opts.LoadedFrom,
	}
	return s
}

// SkillOptions is the constructor parameter set for NewSkill.
type SkillOptions struct {
	Name, DisplayName, Description string
	HasUserSpecifiedDescription    bool
	WhenToUse, Version             string
	Content, BaseDir               string

	AllowedTools               []string
	ArgumentHint               string
	ArgumentNames              []string
	Model                      string

	DisableModelInvocation bool
	UserInvocable          *bool

	Hooks           map[string]any
	ExecutionContext string
	Agent           string
	Paths           []string
	Effort          string
	Shell           map[string]any

	Source, LoadedFrom string
}

// Metadata returns the skill's slash command descriptor.
func (s *Skill) Metadata() command.Metadata {
	return command.Metadata{
		Name:        s.name,
		Description: s.description,
		Usage:       "/" + s.name,
	}
}

// Execute returns the skill's markdown content as output text.
// The base directory is prepended and ${CLAUDE_SKILL_DIR} is replaced.
// Bundled skills with a custom GetPromptForCommand delegate to that function.
func (s *Skill) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	// Bundled skills with a custom prompt function delegate to it.
	if s.bundledPrompt != nil {
		content, err := s.bundledPrompt(s.buildRawArgString(args))
		if err != nil {
			return command.Result{}, err
		}
		return s.processAndReturn(ctx, content)
	}

	finalContent := s.content
	if s.baseDir != "" {
		normalizedDir := filepath.ToSlash(s.baseDir)
		finalContent = strings.ReplaceAll(finalContent, "${CLAUDE_SKILL_DIR}", normalizedDir)
		finalContent = "Base directory for this skill: " + normalizedDir + "\n\n" + finalContent
	}

	return s.processAndReturn(ctx, finalContent)
}

// processAndReturn runs shell command processing on the skill content (if
// ShellExecutor is configured) and returns the final command result.
func (s *Skill) processAndReturn(ctx context.Context, content string) (command.Result, error) {
	if ShellExecutor != nil {
		workingDir := s.baseDir
		processed, err := processShellCommands(ctx, content, ShellExecutor, workingDir)
		if err != nil {
			return command.Result{}, err
		}
		content = processed
	}
	return command.Result{
		Output: content,
	}, nil
}

// buildRawArgString converts command.Args back into a raw argument string
// for bundled skills that expect the raw user input.
func (s *Skill) buildRawArgString(args command.Args) string {
	if args.RawLine != "" {
		return args.RawLine
	}
	return strings.Join(args.Raw, " ")
}

// --- Multi-source loading ---

// LoadUserSkills discovers and loads skill definitions from the user's global
// skills directory (~/.claude/skills/).
func LoadUserSkills(homeDir string) ([]*Skill, []LoadError, error) {
	skillsDir := filepath.Join(homeDir, ".claude", "skills")
	return loadSkillsFromDir(skillsDir, "userSettings")
}

// LoadManagedSkills discovers and loads skill definitions from the managed
// skills directory (~/.claude/managed/.claude/skills/).
func LoadManagedSkills(homeDir string) ([]*Skill, []LoadError, error) {
	skillsDir := filepath.Join(homeDir, ".claude", "managed", ".claude", "skills")
	return loadSkillsFromDir(skillsDir, "policySettings")
}

// LoadProjectSkills discovers and loads skill definitions from the project's
// .claude/skills/ directory and all parent directories up to (but not
// including) the home directory. Skills closer to the project root take
// precedence when names conflict (first-in wins at registration).
func LoadProjectSkills(projectDir string, homeDir string) ([]*Skill, []LoadError, error) {
	dirs := getProjectDirsUpToHome("skills", projectDir, homeDir)
	var allSkills []*Skill
	var allErrors []LoadError

	// Walk from shallowest (closest to home) to deepest (CWD) so deeper
	// skills are appended later and take precedence during registration.
	for i := len(dirs) - 1; i >= 0; i-- {
		skills, errs, _ := loadSkillsFromDir(dirs[i], "projectSettings")
		allSkills = append(allSkills, skills...)
		allErrors = append(allErrors, errs...)
	}

	return allSkills, allErrors, nil
}

// LoadAdditionalDirSkills loads skills from .claude/skills/ directories under
// each --add-dir path.
func LoadAdditionalDirSkills(dirs []string) ([]*Skill, []LoadError, error) {
	var allSkills []*Skill
	var allErrors []LoadError

	for _, dir := range dirs {
		skillsDir := filepath.Join(dir, ".claude", "skills")
		skills, errs, _ := loadSkillsFromDir(skillsDir, "projectSettings")
		allSkills = append(allSkills, skills...)
		allErrors = append(allErrors, errs...)
	}

	return allSkills, allErrors, nil
}

// LoadError records a single skill directory that failed to load or parse.
type LoadError struct {
	Name  string
	Error string
}

// getProjectDirsUpToHome walks from cwd upward to homeDir and returns every
// .claude/<subdir> directory found along the path (in order from cwd to home).
func getProjectDirsUpToHome(subdir, cwd, homeDir string) []string {
	var dirs []string
	dir := cwd
	for {
		skillsDir := filepath.Join(dir, ".claude", subdir)
		if info, err := os.Stat(skillsDir); err == nil && info.IsDir() {
			dirs = append(dirs, skillsDir)
		}
		if dir == homeDir || dir == filepath.Dir(dir) {
			break
		}
		dir = filepath.Dir(dir)
	}
	return dirs
}

// DeduplicateByPath canonicalises each skill's base directory via
// filepath.EvalSymlinks and removes duplicates (first-loaded wins).
// Skills that cannot be canonicalised are kept as-is.
func DeduplicateByPath(skills []*Skill) []*Skill {
	seen := make(map[string]string) // canonicalPath -> source
	var result []*Skill

	for _, s := range skills {
		canonical := s.baseDir
		if realPath, err := filepath.EvalSymlinks(s.baseDir); err == nil {
			canonical = realPath
		}

		if existingSource, ok := seen[canonical]; ok {
			logger.DebugCF("skill", "skipping duplicate skill", map[string]any{
				"name":             s.name,
				"canonical_path":   canonical,
				"source":           s.source,
				"existing_source":  existingSource,
			})
			continue
		}

		seen[canonical] = s.source
		result = append(result, s)
	}

	return result
}

// --- Internal loader ---

// loadSkillsFromDir walks a skills directory and loads each subdirectory's SKILL.md.
func loadSkillsFromDir(skillsDir string, source string) ([]*Skill, []LoadError, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("skill: read skills dir %s: %w", skillsDir, err)
	}

	var skills []*Skill
	var loadErrors []LoadError

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		skillDir := filepath.Join(skillsDir, skillName)
		skillFile := filepath.Join(skillDir, "SKILL.md")

		skill, err := loadSkillFromFile(skillFile, skillName, skillDir, source)
		if err != nil {
			logger.DebugCF("skill", "failed to load skill", map[string]any{
				"name":   skillName,
				"path":   skillFile,
				"source": source,
				"error":  err.Error(),
			})
			loadErrors = append(loadErrors, LoadError{
				Name:  skillName,
				Error: err.Error(),
			})
			continue
		}
		if skill != nil {
			skills = append(skills, skill)
		}
	}

	return skills, loadErrors, nil
}

// loadSkillFromFile reads a SKILL.md file, parses its frontmatter, and builds a Skill.
// source is written into the Skill for tracking.
func loadSkillFromFile(skillFile, skillName, skillDir, source string) (*Skill, error) {
	data, err := os.ReadFile(skillFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read skill file: %w", err)
	}

	frontmatter, body, err := agentloader.ParseFrontmatter(string(data))
	if err != nil {
		logger.DebugCF("skill", "failed to parse frontmatter", map[string]any{
			"name": skillName,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	// Frontmatter can be nil when the file has no --- delimiters.
	resolvedName := toString(frontmatter, "name")
	if resolvedName == "" {
		resolvedName = skillName
	}

	// Description: prefer explicit frontmatter, fall back to body extraction.
	rawDescription := toString(frontmatter, "description")
	hasUserSpecifiedDescription := rawDescription != ""
	description := rawDescription
	if description == "" {
		description = extractDescription(body, resolvedName)
	}

	// when_to_use
	whenToUse := toString(frontmatter, "when_to_use")

	// allowed-tools
	allowedTools := parseSlashCommandTools(frontmatter, "allowed-tools")

	// argument-hint
	argumentHint := toString(frontmatter, "argument-hint")

	// arguments
	argumentNames := parseArgumentNames(frontmatter, "arguments")

	// model — "inherit" means no override (nil).
	model := ""
	if rawModel := toString(frontmatter, "model"); rawModel != "" && rawModel != "inherit" {
		model = rawModel
	}

	// boolean flags — default user-invocable to true.
	disableModelInvocation := toBool(frontmatter, "disable-model-invocation")
	userInvocable := toBoolExplicit(frontmatter, "user-invocable", true)

	// hooks
	var hooks map[string]any
	if raw, ok := frontmatter["hooks"]; ok {
		if m, ok := raw.(map[string]any); ok {
			hooks = m
		}
	}

	// context
	executionContext := ""
	if toString(frontmatter, "context") == "fork" {
		executionContext = "fork"
	}

	// agent
	agent := toString(frontmatter, "agent")

	// paths — filter out "**" (match-all).
	paths := parseSkillPaths(frontmatter, "paths")

	// effort
	effort := parseEffort(frontmatter, "effort")

	// shell
	var shell map[string]any
	if raw, ok := frontmatter["shell"]; ok {
		if m, ok := raw.(map[string]any); ok {
			shell = m
		}
	}

	// version
	version := toString(frontmatter, "version")

	userInvocableVal := userInvocable
	skill := NewSkill(SkillOptions{
		Name:                       resolvedName,
		DisplayName:                toString(frontmatter, "name"),
		Description:                description,
		HasUserSpecifiedDescription: hasUserSpecifiedDescription,
		WhenToUse:                  whenToUse,
		Version:                    version,
		Content:                    body,
		BaseDir:                    skillDir,
		AllowedTools:               allowedTools,
		ArgumentHint:               argumentHint,
		ArgumentNames:              argumentNames,
		Model:                      model,
		DisableModelInvocation:     disableModelInvocation,
		UserInvocable:              &userInvocableVal,
		Hooks:                      hooks,
		ExecutionContext:           executionContext,
		Agent:                      agent,
		Paths:                      paths,
		Effort:                     effort,
		Shell:                      shell,
		Source:                     source,
		LoadedFrom:                 "skills",
	})

	logger.DebugCF("skill", "loaded skill", map[string]any{
		"name":             skill.name,
		"source":           source,
		"has_when_to_use":  whenToUse != "",
		"has_paths":        len(paths) > 0,
		"user_invocable":   userInvocable,
	})

	return skill, nil
}

// --- Frontmatter helpers ---

// toString extracts a string value from a frontmatter map.
func toString(fm map[string]any, key string) string {
	if fm == nil {
		return ""
	}
	v, ok := fm[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

// toBool parses a boolean from frontmatter (bool or string "true"/"false").
func toBool(fm map[string]any, key string) bool {
	if fm == nil {
		return false
	}
	v, ok := fm[key]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true")
	default:
		return false
	}
}

// toBoolExplicit is like toBool but returns the given default when the key is absent.
func toBoolExplicit(fm map[string]any, key string, def bool) bool {
	if fm == nil {
		return def
	}
	if _, ok := fm[key]; !ok {
		return def
	}
	return toBool(fm, key)
}

// parseSlashCommandTools extracts allowed-tools from frontmatter.
// Accepts string (comma-separated), []string, or []any.
func parseSlashCommandTools(fm map[string]any, key string) []string {
	if fm == nil {
		return nil
	}
	v, ok := fm[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case string:
		parts := strings.Split(t, ",")
		var result []string
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	case []string:
		var result []string
		for _, s := range t {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	case []any:
		var result []string
		for _, item := range t {
			if s, ok := item.(string); ok {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					result = append(result, trimmed)
				}
			}
		}
		return result
	default:
		return nil
	}
}

// parseArgumentNames parses the arguments frontmatter field.
// Accepts string (comma-separated) or []string/[]any.
func parseArgumentNames(fm map[string]any, key string) []string {
	return parseSlashCommandTools(fm, key)
}

// parseSkillPaths extracts path patterns and filters out "**" (match-all).
func parseSkillPaths(fm map[string]any, key string) []string {
	raw := parseSlashCommandTools(fm, key)
	if len(raw) == 0 {
		return nil
	}
	// Strip trailing /** suffix and filter empty / match-all patterns.
	var patterns []string
	for _, p := range raw {
		p = strings.TrimSuffix(p, "/**")
		if p != "" && p != "**" {
			patterns = append(patterns, p)
		}
	}
	if len(patterns) == 0 {
		return nil
	}
	return patterns
}

// parseEffort parses the effort frontmatter field (1-5 integer or label).
func parseEffort(fm map[string]any, key string) string {
	if fm == nil {
		return ""
	}
	v, ok := fm[key]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case float64:
		n := int(t)
		if n < 1 {
			n = 1
		}
		if n > 5 {
			n = 5
		}
		return fmt.Sprintf("%d", n)
	case int:
		n := t
		if n < 1 {
			n = 1
		}
		if n > 5 {
			n = 5
		}
		return fmt.Sprintf("%d", n)
	case string:
		labels := []string{"LOW", "MEDIUM", "HIGH", "VERY_HIGH", "MAXIMUM"}
		for i, label := range labels {
			if strings.EqualFold(t, label) {
				return fmt.Sprintf("%d", i+1)
			}
		}
		return ""
	default:
		return ""
	}
}

// extractDescription extracts the first meaningful paragraph from markdown
// content to use as a fallback description.
func extractDescription(content string, fallbackName string) string {
	lines := strings.Split(content, "\n")
	var para []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			if len(para) > 0 {
				break
			}
			continue
		}
		para = append(para, trimmed)
	}
	if len(para) > 0 {
		return strings.Join(para, " ")
	}
	return fmt.Sprintf("Skill %s", fallbackName)
}

// --- Registration ---

// RegisterSkills registers skills into the given command registry.
// Skills whose names conflict with already-registered commands are skipped
// (built-in commands take precedence over file-based skills).
// Returns the number of skills successfully registered.
func RegisterSkills(registry command.Registry, skills []*Skill, source string) int {
	if registry == nil {
		return 0
	}

	registered := 0
	for _, skill := range skills {
		if _, exists := registry.Get(skill.name); exists {
			logger.DebugCF("skill", "skipping skill with conflicting name", map[string]any{
				"name":   skill.name,
				"source": source,
			})
			continue
		}

		if err := registry.Register(skill); err != nil {
			logger.DebugCF("skill", "failed to register skill", map[string]any{
				"name":   skill.name,
				"source": source,
				"error":  err.Error(),
			})
			continue
		}
		registered++
	}

	if registered > 0 {
		logger.InfoCF("skill", "registered skills", map[string]any{
			"count":  registered,
			"source": source,
		})
	}

	return registered
}
