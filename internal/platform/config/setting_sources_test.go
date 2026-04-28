package config

import "testing"

// TestFormatSettingSourcesFlag verifies parsed setting sources round-trip into canonical CLI tokens.
func TestFormatSettingSourcesFlag(t *testing.T) {
	tests := []struct {
		name    string
		sources []SettingSource
		want    string
	}{
		{
			name:    "empty",
			sources: []SettingSource{},
			want:    "",
		},
		{
			name:    "user project local",
			sources: []SettingSource{SettingSourceUserSettings, SettingSourceProjectSettings, SettingSourceLocalSettings},
			want:    "user,project,local",
		},
		{
			name:    "project only",
			sources: []SettingSource{SettingSourceProjectSettings},
			want:    "project",
		},
		{
			name:    "ignores non-disk sources",
			sources: []SettingSource{SettingSourcePolicySettings, SettingSourceFlagSettings, SettingSourceLocalSettings},
			want:    "local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSettingSourcesFlag(tt.sources)
			if got != tt.want {
				t.Fatalf("FormatSettingSourcesFlag() = %q, want %q", got, tt.want)
			}
		})
	}
}

