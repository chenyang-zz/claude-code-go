package bootstrap

import (
	"context"
	"testing"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	platformconfig "github.com/sheepzhao/claude-code-go/internal/platform/config"
)

func TestParseEarlyCLIOptionsOutputFormatFlag(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		wantFormat      string
		wantRemaining   []string
		wantErr         bool
		wantErrContains string
	}{
		{
			name:          "output-format with space",
			args:          []string{"--output-format", "stream-json", "hello"},
			wantFormat:    "stream-json",
			wantRemaining: []string{"hello"},
		},
		{
			name:          "output-format with equals",
			args:          []string{"--output-format=stream-json", "hello"},
			wantFormat:    "stream-json",
			wantRemaining: []string{"hello"},
		},
		{
			name:            "output-format missing value",
			args:            []string{"--output-format"},
			wantErr:         true,
			wantErrContains: "missing value",
		},
		{
			name:          "output-format empty value",
			args:          []string{"--output-format", "", "hello"},
			wantFormat:    "",
			wantRemaining: []string{"hello"},
		},
		{
			name:          "first output-format wins",
			args:          []string{"--output-format=stream-json", "--output-format=console"},
			wantFormat:    "stream-json",
			wantRemaining: []string{},
		},
		{
			name:          "mixed with other flags",
			args:          []string{"--output-format", "stream-json", "--remote", "--hello"},
			wantFormat:    "stream-json",
			wantRemaining: []string{"--hello"},
		},
		{
			name:            "output-format invalid value",
			args:            []string{"--output-format", "streamjson", "hello"},
			wantErr:         true,
			wantErrContains: "invalid value for --output-format",
		},
		{
			name:            "output-format invalid value with equals",
			args:            []string{"--output-format=ndjson", "hello"},
			wantErr:         true,
			wantErrContains: "invalid value for --output-format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options, remaining, err := ParseEarlyCLIOptions(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.wantErrContains != "" && !containsString(err.Error(), tt.wantErrContains) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if options.OutputFormat != tt.wantFormat {
				t.Fatalf("OutputFormat = %q, want %q", options.OutputFormat, tt.wantFormat)
			}
			if len(remaining) != len(tt.wantRemaining) {
				t.Fatalf("remaining args = %v, want %v", remaining, tt.wantRemaining)
			}
			for i := range remaining {
				if remaining[i] != tt.wantRemaining[i] {
					t.Fatalf("remaining[%d] = %q, want %q", i, remaining[i], tt.wantRemaining[i])
				}
			}
		})
	}
}

func TestEarlyOptionsLoaderOutputFormat(t *testing.T) {
	loader := earlyOptionsLoader{
		base: mockConfigLoader{output: coreconfig.DefaultConfig()},
		options: EarlyCLIOptions{
			OutputFormat: "stream-json",
		},
	}

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.OutputFormat != "stream-json" {
		t.Fatalf("OutputFormat = %q, want stream-json", cfg.OutputFormat)
	}
}

func TestParseEarlyCLIOptionsSettingSourcesFlag(t *testing.T) {
	options, remaining, err := ParseEarlyCLIOptions([]string{"--setting-sources", "project,local", "prompt"})
	if err != nil {
		t.Fatalf("ParseEarlyCLIOptions() error = %v", err)
	}
	if len(remaining) != 1 || remaining[0] != "prompt" {
		t.Fatalf("remaining = %#v, want [prompt]", remaining)
	}
	if !options.HasSettingSources {
		t.Fatal("HasSettingSources = false, want true")
	}
	if len(options.SettingSources) != 2 || options.SettingSources[0] != platformconfig.SettingSourceProjectSettings || options.SettingSources[1] != platformconfig.SettingSourceLocalSettings {
		t.Fatalf("SettingSources = %#v, want [projectSettings localSettings]", options.SettingSources)
	}
}

func TestEarlyOptionsLoaderLoadsSettingSourcesFlagForPassThrough(t *testing.T) {
	loader := earlyOptionsLoader{
		base: mockConfigLoader{output: coreconfig.DefaultConfig()},
		options: EarlyCLIOptions{
			SettingSources:    []platformconfig.SettingSource{platformconfig.SettingSourceProjectSettings, platformconfig.SettingSourceLocalSettings},
			HasSettingSources: true,
		},
	}

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.HasSettingSourcesFlag {
		t.Fatal("HasSettingSourcesFlag = false, want true")
	}
	if cfg.SettingSourcesFlag != "project,local" {
		t.Fatalf("SettingSourcesFlag = %q, want project,local", cfg.SettingSourcesFlag)
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type mockConfigLoader struct {
	output coreconfig.Config
}

func (m mockConfigLoader) Load(ctx context.Context) (coreconfig.Config, error) {
	return m.output, nil
}
