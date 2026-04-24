package bash

import (
	"testing"
)

func TestApplySedSubstitution(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		info     *SedEditInfo
		expected string
	}{
		{
			name:    "basic replacement",
			content: "hello world",
			info: &SedEditInfo{
				Pattern:     "world",
				Replacement: "universe",
				Flags:       "",
			},
			expected: "hello universe",
		},
		{
			name:    "global replacement",
			content: "foo bar foo baz",
			info: &SedEditInfo{
				Pattern:     "foo",
				Replacement: "XXX",
				Flags:       "g",
			},
			expected: "XXX bar XXX baz",
		},
		{
			name:    "first match only",
			content: "foo bar foo baz",
			info: &SedEditInfo{
				Pattern:     "foo",
				Replacement: "XXX",
				Flags:       "",
			},
			expected: "XXX bar foo baz",
		},
		{
			name:    "case insensitive first match only",
			content: "Hello World HELLO",
			info: &SedEditInfo{
				Pattern:     "hello",
				Replacement: "hi",
				Flags:       "i",
			},
			expected: "hi World HELLO",
		},
		{
			name:    "case insensitive global",
			content: "Hello World HELLO",
			info: &SedEditInfo{
				Pattern:     "hello",
				Replacement: "hi",
				Flags:       "ig",
			},
			expected: "hi World hi",
		},
		{
			name:    "ampersand full match",
			content: "foo bar",
			info: &SedEditInfo{
				Pattern:     "foo",
				Replacement: "[&]",
				Flags:       "g",
			},
			expected: "[foo] bar",
		},
		{
			name:    "escaped ampersand literal",
			content: "foo bar",
			info: &SedEditInfo{
				Pattern:     "foo",
				Replacement: `\&`,
				Flags:       "g",
			},
			expected: "& bar",
		},
		{
			name:    "BRE escaped plus matches one or more a",
			content: "aab c+d",
			info: &SedEditInfo{
				Pattern:       `a\+b`,
				Replacement:   "XXX",
				Flags:         "g",
				ExtendedRegex: false,
			},
			expected: "XXX c+d",
		},
		{
			name:    "ERE plus matches one or more a",
			content: "aab c+d",
			info: &SedEditInfo{
				Pattern:       `a+b`,
				Replacement:   "XXX",
				Flags:         "g",
				ExtendedRegex: true,
			},
			expected: "XXX c+d",
		},
		{
			name:    "BRE literal plus matches literal plus",
			content: "a+b",
			info: &SedEditInfo{
				Pattern:       "a+b",
				Replacement:   "XXX",
				Flags:         "g",
				ExtendedRegex: false,
			},
			expected: "XXX",
		},
		{
			name:    "newline in replacement",
			content: "foo bar",
			info: &SedEditInfo{
				Pattern:     " ",
				Replacement: `\n`,
				Flags:       "g",
			},
			expected: "foo\nbar",
		},
		{
			name:    "invalid regex returns original",
			content: "hello world",
			info: &SedEditInfo{
				Pattern:     "[invalid",
				Replacement: "XXX",
				Flags:       "g",
			},
			expected: "hello world",
		},
		{
			name:    "multiline flag",
			content: "line1\nline2",
			info: &SedEditInfo{
				Pattern:     "^line",
				Replacement: "L",
				Flags:       "gm",
			},
			expected: "L1\nL2",
		},
		{
			name:    "escaped slash in pattern",
			content: "/path/to/file",
			info: &SedEditInfo{
				Pattern:     `\/path\/to`,
				Replacement: "/new/path",
				Flags:       "g",
			},
			expected: "/new/path/file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applySedSubstitution(tt.content, tt.info)
			if got != tt.expected {
				t.Errorf("applySedSubstitution(%q) = %q, want %q", tt.content, got, tt.expected)
			}
		})
	}
}
