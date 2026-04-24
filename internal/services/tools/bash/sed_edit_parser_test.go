package bash

import (
	"testing"
)

func TestIsSedInPlaceEdit(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"basic substitution", "sed -i 's/foo/bar/g' file.txt", true},
		{"with backup suffix", "sed -i.bak 's/foo/bar/g' file.txt", true},
		{"with extended regex", "sed -E -i 's/foo/bar/g' file.txt", true},
		{"macOS empty suffix", "sed -i '' 's/foo/bar/g' file.txt", true},
		{"with -e flag", "sed -i -e 's/foo/bar/g' file.txt", true},
		{"with --expression", "sed -i --expression='s/foo/bar/g' file.txt", true},
		{"no -i flag", "sed 's/foo/bar/g' file.txt", false},
		{"not sed command", "echo hello", false},
		{"multiple files", "sed -i 's/foo/bar/g' file1.txt file2.txt", false},
		{"print command", "sed -i 'p' file.txt", false},
		{"unknown flag", "sed -i -n 's/foo/bar/g' file.txt", false},
		{"compound command", "sed -i 's/foo/bar/g' file.txt && echo done", false},
		{"sedfoo not sed", "sedfoo -i 's/foo/bar/g' file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSedInPlaceEdit(tt.command)
			if got != tt.want {
				t.Errorf("isSedInPlaceEdit(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestParseSedEditCommand(t *testing.T) {
	tests := []struct {
		name            string
		command         string
		wantFilePath    string
		wantPattern     string
		wantReplacement string
		wantFlags       string
		wantExtended    bool
		wantNil         bool
	}{
		{
			name:            "basic substitution",
			command:         "sed -i 's/foo/bar/g' file.txt",
			wantFilePath:    "file.txt",
			wantPattern:     "foo",
			wantReplacement: "bar",
			wantFlags:       "g",
		},
		{
			name:            "with extended regex",
			command:         "sed -E -i 's/[a-z]+/XXX/g' file.txt",
			wantFilePath:    "file.txt",
			wantPattern:     "[a-z]+",
			wantReplacement: "XXX",
			wantFlags:       "g",
			wantExtended:    true,
		},
		{
			name:            "with backup suffix",
			command:         "sed -i.bak 's/foo/bar/' file.txt",
			wantFilePath:    "file.txt",
			wantPattern:     "foo",
			wantReplacement: "bar",
			wantFlags:       "",
		},
		{
			name:            "with case insensitive flag",
			command:         "sed -i 's/foo/bar/i' file.txt",
			wantFilePath:    "file.txt",
			wantPattern:     "foo",
			wantReplacement: "bar",
			wantFlags:       "i",
		},
		{
			name:            "escaped delimiter in pattern",
			command:         "sed -i 's/foo\\/bar/baz/g' file.txt",
			wantFilePath:    "file.txt",
			wantPattern:     "foo\\/bar",
			wantReplacement: "baz",
			wantFlags:       "g",
		},
		{
			name:    "no -i flag",
			command: "sed 's/foo/bar/g' file.txt",
			wantNil: true,
		},
		{
			name:    "multiple expressions",
			command: "sed -i -e 's/foo/bar/' -e 's/baz/qux/' file.txt",
			wantNil: true,
		},
		{
			name:    "multiple files",
			command: "sed -i 's/foo/bar/g' file1.txt file2.txt",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSedEditCommand(tt.command)
			if tt.wantNil {
				if got != nil {
					t.Errorf("parseSedEditCommand(%q) = %+v, want nil", tt.command, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("parseSedEditCommand(%q) = nil, want non-nil", tt.command)
			}
			if got.FilePath != tt.wantFilePath {
				t.Errorf("FilePath = %q, want %q", got.FilePath, tt.wantFilePath)
			}
			if got.Pattern != tt.wantPattern {
				t.Errorf("Pattern = %q, want %q", got.Pattern, tt.wantPattern)
			}
			if got.Replacement != tt.wantReplacement {
				t.Errorf("Replacement = %q, want %q", got.Replacement, tt.wantReplacement)
			}
			if got.Flags != tt.wantFlags {
				t.Errorf("Flags = %q, want %q", got.Flags, tt.wantFlags)
			}
			if got.ExtendedRegex != tt.wantExtended {
				t.Errorf("ExtendedRegex = %v, want %v", got.ExtendedRegex, tt.wantExtended)
			}
		})
	}
}

func TestTokenizeShellCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     []string
		wantOk   bool
	}{
		{
			name:   "simple tokens",
			input:  "-i 's/foo/bar/g' file.txt",
			want:   []string{"-i", "s/foo/bar/g", "file.txt"},
			wantOk: true,
		},
		{
			name:   "double quotes",
			input:  `-E "s/foo/bar/g" file.txt`,
			want:   []string{"-E", "s/foo/bar/g", "file.txt"},
			wantOk: true,
		},
		{
			name:   "escaped space",
			input:  `-i s/foo/bar/g file\ name.txt`,
			want:   []string{"-i", "s/foo/bar/g", "file name.txt"},
			wantOk: true,
		},
		{
			name:   "single quote with spaces inside",
			input:  `-i 's/foo bar/baz qux/g' file.txt`,
			want:   []string{"-i", "s/foo bar/baz qux/g", "file.txt"},
			wantOk: true,
		},
		{
			name:   "unbalanced single quote",
			input:  `-i 's/foo/bar/g file.txt`,
			want:   nil,
			wantOk: false,
		},
		{
			name:   "unbalanced double quote",
			input:  `-i "s/foo/bar/g file.txt`,
			want:   nil,
			wantOk: false,
		},
		{
			name:   "multiple spaces",
			input:  `  -i   's/foo/bar/g'   file.txt  `,
			want:   []string{"-i", "s/foo/bar/g", "file.txt"},
			wantOk: true,
		},
		{
			name:   "backslash in single quote is literal",
			input:  `-i 's/foo\/bar/baz/g' file.txt`,
			want:   []string{"-i", `s/foo\/bar/baz/g`, "file.txt"},
			wantOk: true,
		},
		{
			name:   "empty input",
			input:  "",
			want:   nil,
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tokenizeShellCommand(tt.input)
			if ok != tt.wantOk {
				t.Errorf("tokenizeShellCommand(%q) ok = %v, want %v", tt.input, ok, tt.wantOk)
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("tokenizeShellCommand(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("tokenizeShellCommand(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
