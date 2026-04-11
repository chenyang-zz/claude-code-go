package config

import (
	"reflect"
	"testing"
)

// TestMergeOverlaysPermissionConfig verifies minimal permission settings override the base runtime config without dropping untouched fields.
func TestMergeOverlaysPermissionConfig(t *testing.T) {
	base := Config{
		ApprovalMode: "default",
		Permissions: PermissionConfig{
			DefaultMode:           "default",
			Allow:                 []string{"Bash(ls)"},
			AdditionalDirectories: []string{"packages/base"},
		},
	}

	override := Config{
		ApprovalMode: "plan",
		Permissions: PermissionConfig{
			DefaultMode:                  "plan",
			Deny:                         []string{"Bash(rm -rf)"},
			Ask:                          []string{"Edit(*)"},
			AdditionalDirectories:        []string{"packages/feature"},
			DisableBypassPermissionsMode: "disable",
		},
	}

	got := Merge(base, override)
	want := Config{
		ApprovalMode: "plan",
		Permissions: PermissionConfig{
			DefaultMode:                  "plan",
			Allow:                        []string{"Bash(ls)"},
			Deny:                         []string{"Bash(rm -rf)"},
			Ask:                          []string{"Edit(*)"},
			AdditionalDirectories:        []string{"packages/feature"},
			DisableBypassPermissionsMode: "disable",
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Merge() = %#v, want %#v", got, want)
	}
}
