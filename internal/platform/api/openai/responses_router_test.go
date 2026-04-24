package openai

import "testing"

func TestUseResponsesAPI(t *testing.T) {
	cases := []struct {
		model string
		want  bool
	}{
		{"o1-pro", true},
		{"o1-pro-2025-03-19", true},
		{"o3-mini", true},
		{"o3-mini-2025-01-31", true},
		{"o3", true},
		{"o1", true},
		{"o1-preview", false},
		{"o1-mini", false},
		{"computer-use-preview", true},
		{"gpt-4o", false},
		{"gpt-4o-mini", false},
		{"gpt-4", false},
		{"gpt-3.5-turbo", false},
		{"claude-3-5-sonnet", false},
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			got := UseResponsesAPI(tc.model)
			if got != tc.want {
				t.Errorf("UseResponsesAPI(%q) = %v, want %v", tc.model, got, tc.want)
			}
		})
	}
}
