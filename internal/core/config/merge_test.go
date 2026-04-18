package config

import (
	"reflect"
	"testing"
)

// TestMergeOverlaysPermissionConfig verifies minimal permission settings override the base runtime config without dropping untouched fields.
func TestMergeOverlaysPermissionConfig(t *testing.T) {
	base := Config{
		ApprovalMode: "default",
		Env: map[string]string{
			"HOME_ONLY": "1",
			"SHARED":    "base",
		},
		AuthToken: "base-token",
		OAuthAccount: OAuthAccountConfig{
			AccountUUID:      "base-account",
			OrganizationName: "Base Org",
		},
		Permissions: PermissionConfig{
			DefaultMode: "default",
			Allow:       []string{"Bash(ls)"},
			AdditionalDirectoryEntries: []AdditionalDirectoryConfig{
				{
					Path:   "packages/base",
					Source: AdditionalDirectorySourceProjectSettings,
				},
			},
			AdditionalDirectories: []string{"packages/base"},
		},
	}

	override := Config{
		ApprovalMode: "plan",
		Env: map[string]string{
			"SHARED": "override",
			"LOCAL":  "1",
		},
		AuthToken: "override-token",
		OAuthAccount: OAuthAccountConfig{
			EmailAddress:     "user@example.com",
			OrganizationUUID: "org-123",
		},
		Permissions: PermissionConfig{
			DefaultMode: "plan",
			Deny:        []string{"Bash(rm -rf)"},
			Ask:         []string{"Edit(*)"},
			AdditionalDirectoryEntries: []AdditionalDirectoryConfig{
				{
					Path:   "packages/feature",
					Source: AdditionalDirectorySourceLocalSettings,
				},
			},
			AdditionalDirectories:        []string{"packages/feature"},
			DisableBypassPermissionsMode: "disable",
		},
	}

	got := Merge(base, override)
	want := Config{
		ApprovalMode: "plan",
		Env: map[string]string{
			"HOME_ONLY": "1",
			"SHARED":    "override",
			"LOCAL":     "1",
		},
		AuthToken: "override-token",
		OAuthAccount: OAuthAccountConfig{
			AccountUUID:      "base-account",
			EmailAddress:     "user@example.com",
			OrganizationUUID: "org-123",
			OrganizationName: "Base Org",
		},
		Permissions: PermissionConfig{
			DefaultMode: "plan",
			Allow:       []string{"Bash(ls)"},
			Deny:        []string{"Bash(rm -rf)"},
			Ask:         []string{"Edit(*)"},
			AdditionalDirectoryEntries: []AdditionalDirectoryConfig{
				{
					Path:   "packages/feature",
					Source: AdditionalDirectorySourceLocalSettings,
				},
			},
			AdditionalDirectories:        []string{"packages/feature"},
			DisableBypassPermissionsMode: "disable",
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Merge() = %#v, want %#v", got, want)
	}
}
