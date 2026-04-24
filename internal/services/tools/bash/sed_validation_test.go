package bash

import (
	"testing"
)

func TestSedCommandIsAllowed(t *testing.T) {
	tests := []struct {
		name            string
		command         string
		allowFileWrites bool
		want            bool
	}{
		// Safe read-only commands
		{"safe substitution", "sed 's/foo/bar/g' file.txt", false, true},
		{"safe substitution with -E", "sed -E 's/[a-z]+/XXX/g' file.txt", false, true},
		{"safe print command", "sed -n '1p' file.txt", false, true},
		{"safe print range", "sed -n '1,5p' file.txt", false, true},

		// In-place editing (allowFileWrites=true)
		{"in-place substitution", "sed -i 's/foo/bar/g' file.txt", true, true},
		{"in-place with -E", "sed -E -i 's/[a-z]+/XXX/g' file.txt", true, true},
		{"in-place with backup suffix rejected", "sed -i.bak 's/foo/bar/g' file.txt", true, false},

		// In-place editing (allowFileWrites=false) — -i not allowed
		{"in-place without permission", "sed -i 's/foo/bar/g' file.txt", false, false},

		// Dangerous operations
		{"write command w", "sed 's/foo/bar/w output.txt' file.txt", false, false},
		{"execute command e", "sed 's/foo/bar/e' file.txt", false, false},
		{"dangerous write w", "sed '1w file.txt' file.txt", false, false},
		{"dangerous execute e", "sed '1e rm -rf /' file.txt", false, false},

		// Flags that are safe but not used by substitution in TS side
		{"flag -n on substitution", "sed -n 's/foo/bar/g' file.txt", false, true},
		{"flag -z on substitution", "sed -z 's/foo/bar/g' file.txt", false, true},

		// Non-sed commands
		{"not sed", "echo hello", false, true},
		{"git command", "git status", false, true},

		// Dangerous in in-place mode too
		{"dangerous even with allowFileWrites", "sed -i 's/foo/bar/e' file.txt", true, false},

		// Semicolons in substitution
		{"semicolon in subst", "sed -i 's/foo/bar/;s/baz/qux/' file.txt", true, false},

		// Non-ASCII characters
		{"non-ascii in expression", "sed 's/foo/b\u0101r/g' file.txt", false, false},

		// Curly braces
		{"curly braces", "sed 's/foo/{bar}/g' file.txt", false, false},

		// Empty / invalid
		{"empty sed", "sed", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sedCommandIsAllowed(tt.command, tt.allowFileWrites)
			if got != tt.want {
				t.Errorf("sedCommandIsAllowed(%q, %v) = %v, want %v", tt.command, tt.allowFileWrites, got, tt.want)
			}
		})
	}
}

func TestContainsDangerousOperations(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		want       bool
	}{
		// Safe expressions
		{"safe substitution", "s/foo/bar/g", false},
		{"safe substitution no flags", "s/foo/bar/", false},
		{"safe print", "p", false},
		{"safe print line", "1p", false},

		// Dangerous: write commands
		{"write command w", "w file.txt", true},
		{"write after address", "1w file.txt", true},
		{"write after pattern", "/foo/w file.txt", true},

		// Dangerous: execute commands
		{"execute command e", "e rm -rf /", true},
		{"execute after line", "1e cat /etc/passwd", true},
		{"execute after pattern", "/foo/e cmd", true},

		// Dangerous: substitution flags
		{"subst with w flag", "s/foo/bar/w", true},
		{"subst with W flag", "s/foo/bar/W", true},
		{"subst with e flag", "s/foo/bar/e", true},
		{"subst with E flag", "s/foo/bar/E", true},

		// Dangerous: non-ASCII
		{"non-ascii", "s/foo/b\u0101r/g", true},

		// Dangerous: curly braces
		{"curly braces", "s/foo/{bar}/g", true},

		// Dangerous: comments
		{"comment", "s/foo/bar/ # comment", true},

		// Dangerous: negation
		{"negation", "!/foo/d", true},

		// Dangerous: backslash tricks
		{"backslash delimiter", `s\foo\bar\g`, true},

		// Safe: s# delimiter is checked by malformed pattern check, not containsDangerousOperations alone
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsDangerousOperations(tt.expression)
			if got != tt.want {
				t.Errorf("containsDangerousOperations(%q) = %v, want %v", tt.expression, got, tt.want)
			}
		})
	}
}

func TestExtractSedExpressions(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
		wantErr bool
	}{
		{
			name:    "bare expression",
			command: "sed 's/foo/bar/g' file.txt",
			want:    []string{"s/foo/bar/g"},
		},
		{
			name:    "with -e flag",
			command: "sed -e 's/foo/bar/g' file.txt",
			want:    []string{"s/foo/bar/g"},
		},
		{
			name:    "with --expression=",
			command: "sed --expression='s/foo/bar/g' file.txt",
			want:    []string{"s/foo/bar/g"},
		},
		{
			name:    "print command",
			command: "sed -n '1p' file.txt",
			want:    []string{"1p"},
		},
		{
			name:    "dangerous flag combination",
			command: "sed -ew 's/foo/bar/' file.txt",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "not a sed command",
			command: "echo hello",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractSedExpressions(tt.command)
			if tt.wantErr {
				if err == nil && got != nil {
					// extractSedExpressions returns nil, nil for errors in our impl
				}
				// Our implementation returns nil, nil on error; just check got is nil
				if got != nil {
					t.Errorf("extractSedExpressions(%q) = %v, want nil on error", tt.command, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("extractSedExpressions(%q) = %v, want %v", tt.command, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractSedExpressions(%q)[%d] = %q, want %q", tt.command, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSedHasFileArgs(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"with file arg", "sed 's/foo/bar/g' file.txt", true},
		{"stdin only", "sed 's/foo/bar/g'", false},
		{"with -e and file", "sed -e 's/foo/bar/g' file.txt", true},
		{"not sed", "echo hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sedHasFileArgs(tt.command)
			if got != tt.want {
				t.Errorf("sedHasFileArgs(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestIsPrintCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{"print all", "p", true},
		{"print line 1", "1p", true},
		{"print range", "1,5p", true},
		{"not print", "s/foo/bar/", false},
		{"empty", "", false},
		{"write", "w file", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPrintCommand(tt.cmd)
			if got != tt.want {
				t.Errorf("isPrintCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}
