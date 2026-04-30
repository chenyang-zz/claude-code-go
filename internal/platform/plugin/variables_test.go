package plugin

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSubstitutePluginVariables(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		pluginPath   string
		pluginSource string
		want         string
	}{
		{
			name:       "plugin root substitution",
			value:      "root is ${CLAUDE_PLUGIN_ROOT}",
			pluginPath: "/path/to/plugin",
			want:       "root is /path/to/plugin",
		},
		{
			name:       "multiple substitutions",
			value:      "${CLAUDE_PLUGIN_ROOT}/bin and ${CLAUDE_PLUGIN_ROOT}/lib",
			pluginPath: "/path/to/plugin",
			want:       "/path/to/plugin/bin and /path/to/plugin/lib",
		},
		{
			name:       "no substitution",
			value:      "plain text without placeholders",
			pluginPath: "/path/to/plugin",
			want:       "plain text without placeholders",
		},
		{
			name:       "empty plugin path",
			value:      "root is ${CLAUDE_PLUGIN_ROOT}",
			pluginPath: "",
			want:       "root is ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubstitutePluginVariables(tt.value, tt.pluginPath, tt.pluginSource)
			if got != tt.want {
				t.Errorf("SubstitutePluginVariables() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSubstitutePluginVariables_DataDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", tmpDir)

	value := "data is ${CLAUDE_PLUGIN_DATA}"
	got := SubstitutePluginVariables(value, "/path/to/plugin", "my-plugin")

	expected := filepath.Join(tmpDir, "data", "my-plugin")
	want := "data is " + filepath.ToSlash(expected)
	if got != want {
		t.Errorf("SubstitutePluginVariables() = %q, want %q", got, want)
	}
}

func TestSubstitutePluginVariables_DataDir_NoSource(t *testing.T) {
	value := "data is ${CLAUDE_PLUGIN_DATA}"
	got := SubstitutePluginVariables(value, "/path/to/plugin", "")

	want := "data is ${CLAUDE_PLUGIN_DATA}"
	if got != want {
		t.Errorf("SubstitutePluginVariables() = %q, want %q", got, want)
	}
}

func TestSubstituteSkillDir(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		skillDir string
		want     string
	}{
		{
			name:     "skill dir substitution",
			value:    "skill at ${CLAUDE_SKILL_DIR}",
			skillDir: "/path/to/skill",
			want:     "skill at /path/to/skill",
		},
		{
			name:     "no skill dir",
			value:    "skill at ${CLAUDE_SKILL_DIR}",
			skillDir: "",
			want:     "skill at ${CLAUDE_SKILL_DIR}",
		},
		{
			name:     "no placeholder",
			value:    "plain text",
			skillDir: "/path/to/skill",
			want:     "plain text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubstituteSkillDir(tt.value, tt.skillDir)
			if got != tt.want {
				t.Errorf("SubstituteSkillDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSubstituteArguments_Arguments(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		args       []string
		want       string
		appendMode bool
	}{
		{
			name:    "$ARGUMENTS",
			content: "Args: $ARGUMENTS",
			args:    []string{"a", "b", "c"},
			want:    "Args: a b c",
		},
		{
			name:    "$ARGUMENTS not followed by bracket",
			content: "$ARGUMENTS extra",
			args:    []string{"x", "y"},
			want:    "x y extra",
		},
		{
			name:    "$ARGUMENTS should not match $ARGUMENTS[0]",
			content: "$ARGUMENTS[0]",
			args:    []string{"a", "b"},
			want:    "a",
		},
		{
			name:    "no args",
			content: "Args: $ARGUMENTS",
			args:    []string{},
			want:    "Args: $ARGUMENTS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubstituteArguments(tt.content, tt.args, nil, tt.appendMode)
			if got != tt.want {
				t.Errorf("SubstituteArguments() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSubstituteArguments_Indexed(t *testing.T) {
	tests := []struct {
		name    string
		content string
		args    []string
		want    string
	}{
		{
			name:    "$ARGUMENTS[0]",
			content: "first: $ARGUMENTS[0]",
			args:    []string{"alpha", "beta"},
			want:    "first: alpha",
		},
		{
			name:    "$ARGUMENTS[1]",
			content: "second: $ARGUMENTS[1]",
			args:    []string{"alpha", "beta"},
			want:    "second: beta",
		},
		{
			name:    "$ARGUMENTS[2] out of range",
			content: "third: $ARGUMENTS[2]",
			args:    []string{"alpha", "beta"},
			want:    "third: $ARGUMENTS[2]",
		},
		{
			name:    "multiple indexed",
			content: "$ARGUMENTS[0] and $ARGUMENTS[1]",
			args:    []string{"x", "y"},
			want:    "x and y",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubstituteArguments(tt.content, tt.args, nil, false)
			if got != tt.want {
				t.Errorf("SubstituteArguments() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSubstituteArguments_Shorthand(t *testing.T) {
	tests := []struct {
		name    string
		content string
		args    []string
		want    string
	}{
		{
			name:    "$1",
			content: "first: $1",
			args:    []string{"alpha", "beta"},
			want:    "first: alpha",
		},
		{
			name:    "$2",
			content: "second: $2",
			args:    []string{"alpha", "beta"},
			want:    "second: beta",
		},
		{
			name:    "$3 out of range",
			content: "third: $3",
			args:    []string{"alpha", "beta"},
			want:    "third: $3",
		},
		{
			name:    "$0 not valid",
			content: "$0",
			args:    []string{"alpha"},
			want:    "$0",
		},
		{
			name:    "multiple shorthand",
			content: "$1 and $2",
			args:    []string{"x", "y"},
			want:    "x and y",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubstituteArguments(tt.content, tt.args, nil, false)
			if got != tt.want {
				t.Errorf("SubstituteArguments() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSubstituteArguments_Named(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		args          []string
		argumentNames []string
		want          string
	}{
		{
			name:          "named arg $name",
			content:       "Hello $name",
			args:          []string{"Alice"},
			argumentNames: []string{"name"},
			want:          "Hello Alice",
		},
		{
			name:          "named arg $target",
			content:       "Deploy to $target",
			args:          []string{"prod", "us-east"},
			argumentNames: []string{"target", "region"},
			want:          "Deploy to prod",
		},
		{
			name:          "named arg missing value",
			content:       "Hello $name",
			args:          []string{},
			argumentNames: []string{"name"},
			want:          "Hello ",
		},
		{
			name:          "named arg not in content",
			content:       "Plain text",
			args:          []string{"Alice"},
			argumentNames: []string{"name"},
			want:          "Plain text",
		},
		{
			name:          "empty arg name skipped",
			content:       "$test",
			args:          []string{"val"},
			argumentNames: []string{""},
			want:          "$test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubstituteArguments(tt.content, tt.args, tt.argumentNames, false)
			if got != tt.want {
				t.Errorf("SubstituteArguments() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSubstituteArguments_AppendIfNoPlaceholder(t *testing.T) {
	content := "Plain text"
	args := []string{"a", "b"}

	got := SubstituteArguments(content, args, nil, true)
	want := "Plain text\n\nARGUMENTS: a b"
	if got != want {
		t.Errorf("SubstituteArguments() = %q, want %q", got, want)
	}

	// When appendIfNoPlaceholder is false, should not append.
	got = SubstituteArguments(content, args, nil, false)
	if got != content {
		t.Errorf("SubstituteArguments() = %q, want %q", got, content)
	}
}

func TestSubstituteArguments_NoPlaceholderWithArgs(t *testing.T) {
	content := "No placeholders here"
	args := []string{"x"}

	got := SubstituteArguments(content, args, nil, true)
	if !strings.Contains(got, "ARGUMENTS:") {
		t.Errorf("expected 'ARGUMENTS:' in output, got %q", got)
	}
}

func TestSubstituteArguments_Combined(t *testing.T) {
	content := "Plugin: ${CLAUDE_PLUGIN_ROOT}, Args: $ARGUMENTS, First: $1, Named: $name"
	args := []string{"alpha", "beta"}
	argumentNames := []string{"name"}

	got := SubstituteArguments(content, args, argumentNames, false)
	// Note: ${CLAUDE_PLUGIN_ROOT} is handled by SubstitutePluginVariables, not here.
	// But $ARGUMENTS, $1, and $name should be substituted.
	want := "Plugin: ${CLAUDE_PLUGIN_ROOT}, Args: alpha beta, First: alpha, Named: alpha"
	if got != want {
		t.Errorf("SubstituteArguments() = %q, want %q", got, want)
	}
}

func TestSubstituteArguments_WindowsPathNormalization(t *testing.T) {
	// On non-Windows platforms, filepath.ToSlash does not convert backslashes
	// because they are not path separators. Skip this test on non-Windows.
	if runtime.GOOS != "windows" {
		t.Skip("Windows path normalization test skipped on non-Windows systems")
	}
	// Plugin variables should normalize Windows paths.
	got := SubstitutePluginVariables("${CLAUDE_PLUGIN_ROOT}", `C:\Users\test\plugin`, "")
	want := "C:/Users/test/plugin"
	if got != want {
		t.Errorf("SubstitutePluginVariables() = %q, want %q", got, want)
	}
}
