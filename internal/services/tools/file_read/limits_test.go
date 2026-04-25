package file_read

import (
	"os"
	"testing"
)

func TestGetEnvMaxTokens(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     int
	}{
		{"unset", "", 0},
		{"valid", "50000", 50000},
		{"invalid_string", "abc", 0},
		{"zero", "0", 0},
		{"negative", "-1", 0},
		{"float", "3.14", 0},
		{"whitespace", "  ", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(envMaxTokensOverride, tt.envValue)
				defer os.Unsetenv(envMaxTokensOverride)
			} else {
				os.Unsetenv(envMaxTokensOverride)
			}
			if got := getEnvMaxTokens(); got != tt.want {
				t.Errorf("getEnvMaxTokens() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBytesPerTokenForExtension(t *testing.T) {
	tests := []struct {
		ext  string
		want int
	}{
		{"json", 2},
		{"jsonl", 2},
		{"jsonc", 2},
		{"go", 4},
		{"txt", 4},
		{"md", 4},
		{"", 4},
		{"py", 4},
		{"ts", 4},
	}
	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			if got := bytesPerTokenForExtension(tt.ext); got != tt.want {
				t.Errorf("bytesPerTokenForExtension(%q) = %d, want %d", tt.ext, got, tt.want)
			}
		})
	}
}

func TestEstimateTokensForContent(t *testing.T) {
	// Build large strings for boundary tests.
	tests := []struct {
		name    string
		content string
		ext     string
		want    int
	}{
		{"empty", "", "go", 0},
		{"short_go", "hello", "go", 2},     // ceil(5/4) = 2
		{"short_json", "hello", "json", 3}, // ceil(5/2) = 3
		{"exact_4", "abcd", "go", 1},       // ceil(4/4) = 1
		{"exact_2", "ab", "json", 1},       // ceil(2/2) = 1
		{"boundary_25000_go", string(make([]byte, 100000)), "go", 25000},   // ceil(100000/4) = 25000
		{"boundary_25001_go", string(make([]byte, 100001)), "go", 25001},   // ceil(100001/4) = 25001
		{"boundary_25000_json", string(make([]byte, 50000)), "json", 25000}, // ceil(50000/2) = 25000
		{"boundary_25001_json", string(make([]byte, 50001)), "json", 25001}, // ceil(50001/2) = 25001
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := estimateTokensForContent(tt.content, tt.ext); got != tt.want {
				t.Errorf("estimateTokensForContent(%q, %q) = %d, want %d", tt.content, tt.ext, got, tt.want)
			}
		})
	}
}

func TestValidateContentTokens(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		ext       string
		maxTokens int
		wantErr   bool
	}{
		{"under_limit", "hello world", "go", 10, false},
		{"at_limit", string(make([]byte, 40)), "go", 10, false},       // ceil(40/4) = 10
		{"over_limit", string(make([]byte, 41)), "go", 10, true},      // ceil(41/4) = 11
		{"json_dense_over", string(make([]byte, 21)), "json", 10, true}, // ceil(21/2) = 11
		{"json_dense_under", string(make([]byte, 20)), "json", 10, false}, // ceil(20/2) = 10
		{"zero_maxtokens_fallback", string(make([]byte, 25001*4)), "go", 0, true}, // fallback to 25000
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContentTokens(tt.content, tt.ext, tt.maxTokens)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateContentTokens() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if _, ok := err.(*MaxFileReadTokenExceededError); !ok {
					t.Errorf("expected *MaxFileReadTokenExceededError, got %T", err)
				}
			}
		})
	}
}

func TestMaxFileReadTokenExceededError(t *testing.T) {
	err := &MaxFileReadTokenExceededError{TokenCount: 30000, MaxTokens: 25000}
	want := "File content (30000 tokens) exceeds maximum allowed tokens (25000). Use offset and limit parameters to read specific portions of the file, or search for specific content instead of reading the whole file."
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestGetDefaultFileReadingLimits(t *testing.T) {
	// Ensure default limits are populated with expected values.
	limits := getDefaultFileReadingLimits()
	if limits.MaxTokens != DEFAULT_MAX_OUTPUT_TOKENS {
		t.Errorf("MaxTokens = %d, want %d", limits.MaxTokens, DEFAULT_MAX_OUTPUT_TOKENS)
	}
	if limits.MaxSizeBytes != defaultMaxFileSizeBytes {
		t.Errorf("MaxSizeBytes = %d, want %d", limits.MaxSizeBytes, defaultMaxFileSizeBytes)
	}
}
