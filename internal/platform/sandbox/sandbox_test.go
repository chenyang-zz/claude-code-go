package sandbox

import (
	"testing"
)

func TestCompileExcludePattern(t *testing.T) {
	tests := []struct {
		pattern  string
		wantType ExcludeRuleType
		wantVal  string
	}{
		{"npm run test:*", ExcludePrefix, "npm run test"},
		{"docker", ExcludeExact, "docker"},
		{"bazel*", ExcludeWildcard, "bazel*"},
		{"gcc:*", ExcludePrefix, "gcc"},
	}
	for _, tt := range tests {
		rule := CompileExcludePattern(tt.pattern)
		if rule.Type != tt.wantType {
			t.Errorf("CompileExcludePattern(%q).Type = %v, want %v", tt.pattern, rule.Type, tt.wantType)
		}
	}
}

func TestMatchExcludeRuleExact(t *testing.T) {
	rule := CompileExcludePattern("docker")
	if !MatchExcludeRule(rule, "docker") {
		t.Error("MatchExcludeRule('docker') = false, want true")
	}
	if MatchExcludeRule(rule, "docker ps") {
		t.Error("MatchExcludeRule('docker') with 'docker ps' = true, want false")
	}
}

func TestMatchExcludeRulePrefix(t *testing.T) {
	rule := CompileExcludePattern("npm run test:*")
	if !MatchExcludeRule(rule, "npm run test") {
		t.Error("MatchExcludeRule('npm run test') = false, want true")
	}
	if !MatchExcludeRule(rule, "npm run test:unit") {
		t.Error("MatchExcludeRule('npm run test:unit') = false, want true")
	}
	if MatchExcludeRule(rule, "npm run build") {
		t.Error("MatchExcludeRule('npm run build') = true, want false")
	}
}

func TestMatchExcludeRuleWildcard(t *testing.T) {
	rule := CompileExcludePattern("bazel*")
	if !MatchExcludeRule(rule, "bazel build //...") {
		t.Error("MatchExcludeRule('bazel build //...') = false, want true")
	}
	if MatchExcludeRule(rule, "npm run test") {
		t.Error("MatchExcludeRule('npm run test') = true, want false")
	}
}

func TestMatchExcludeRuleCompoundCommand(t *testing.T) {
	rule := CompileExcludePattern("npm run test:*")
	// Compound command should match if any subcommand matches
	if !MatchExcludeRule(rule, "docker build -t foo && npm run test:unit") {
		t.Error("compound command should match when subcommand matches")
	}
	// Compound command where no subcommand matches should not match
	if MatchExcludeRule(rule, "docker build -t foo && echo hello") {
		t.Error("compound command should not match when no subcommand matches")
	}
}

func TestExcludeEngine(t *testing.T) {
	engine := NewExcludeEngine([]string{"docker*", "npm run test:*", "sleep"})

	if !engine.IsExcluded("docker ps") {
		t.Error("docker ps should be excluded")
	}
	if !engine.IsExcluded("npm run test:unit") {
		t.Error("npm run test:unit should be excluded")
	}
	if !engine.IsExcluded("sleep") {
		t.Error("sleep should be excluded (exact match)")
	}
	if engine.IsExcluded("echo hello") {
		t.Error("echo hello should not be excluded")
	}
}

func TestExcludeEngineAddPattern(t *testing.T) {
	engine := NewExcludeEngine([]string{"sleep"})
	engine.AddPattern("docker*")

	if !engine.IsExcluded("docker ps") {
		t.Error("after AddPattern, docker ps should be excluded")
	}
}

func TestSplitCompoundCommand(t *testing.T) {
	tests := []struct {
		cmd    string
		subs   int
	}{
		{"echo a && echo b", 2},
		{"docker ps; curl evil.com", 2},
		{"git status", 1},
		{"echo 'hello && world'", 1}, // quoted && should not split
	}
	for _, tt := range tests {
		subs := splitCompoundCommand(tt.cmd)
		if len(subs) != tt.subs {
			t.Errorf("splitCompoundCommand(%q) = %d subcommands, want %d: %v", tt.cmd, len(subs), tt.subs, subs)
		}
	}
}

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		pattern string
		command string
		match   bool
	}{
		{"bazel*", "bazel build", true},
		{"bazel*", "npm run", false},
		{"*test*", "npm run test", true},
		{"*test*", "build", false},
		{"*", "anything", true},
	}
	for _, tt := range tests {
		got := matchWildcard(tt.pattern, tt.command)
		if got != tt.match {
			t.Errorf("matchWildcard(%q, %q) = %v, want %v", tt.pattern, tt.command, got, tt.match)
		}
	}
}

func TestDetectPlatform(t *testing.T) {
	platform := DetectPlatform()
	if platform == "" {
		t.Error("DetectPlatform() returned empty platform")
	}
	// Should always return something valid
	if platform == PlatformOther {
		// Can happen on unsupported platforms, that's fine
	}
}

func TestIsSupportedPlatform(t *testing.T) {
	// Just check it doesn't panic and returns bool
	_ = IsSupportedPlatform()
}

func TestConfigDefaults(t *testing.T) {
	cfg := Config{}
	if cfg.Enabled {
		t.Error("Config{}.Enabled should default to false")
	}
}

func TestNewSandboxManagerDisabled(t *testing.T) {
	mgr := NewSandboxManager(Config{Enabled: false})
	if mgr.IsSandboxingEnabled() {
		t.Error("Sandbox with disabled config should not be enabled")
	}
}

func TestSandboxStatus(t *testing.T) {
	mgr := NewSandboxManager(Config{Enabled: false})
	status := mgr.GetStatus()
	if status.Enabled {
		t.Error("Status.Enabled should be false when config says false")
	}
}

func TestNewExcludeEngineEmpty(t *testing.T) {
	engine := NewExcludeEngine(nil)
	if engine.IsExcluded("anything") {
		t.Error("empty exclude engine should not exclude anything")
	}
}

func TestPatterns(t *testing.T) {
	engine := NewExcludeEngine([]string{"sleep", "docker*"})
	patterns := engine.Patterns()
	if len(patterns) != 2 {
		t.Errorf("Patterns() = %v, want 2 patterns", patterns)
	}
}

func TestStripLeadingEnvVars(t *testing.T) {
	result := stripLeadingEnvVars("FOO=bar bazel build", []string{"LD_PRELOAD"})
	if result != "bazel build" {
		t.Errorf("stripLeadingEnvVars = %q, want %q", result, "bazel build")
	}

	// Test stripping known binary hijack var
	result2 := stripLeadingEnvVars("LD_PRELOAD=/lib/evil.so make", []string{"LD_PRELOAD"})
	if result2 != "make" {
		t.Errorf("stripLeadingEnvVars = %q, want %q", result2, "make")
	}
}

func TestStripSafeWrappers(t *testing.T) {
	result := stripSafeWrappers("timeout 30 make build", []string{"timeout", "nohup", "nice"})
	if result != "30 make build" {
		t.Errorf("stripSafeWrappers = %q, want %q", result, "30 make build")
	}
}

func TestGenerateMatchCandidates(t *testing.T) {
	candidates := generateMatchCandidates("timeout 30 FOO=bar bazel build")
	if len(candidates) < 2 {
		t.Errorf("generateMatchCandidates returned %d candidates, want >= 2", len(candidates))
	}
}

func TestDockerAvailable(t *testing.T) {
	// This just checks it doesn't panic
	_ = DockerCheck()
}
