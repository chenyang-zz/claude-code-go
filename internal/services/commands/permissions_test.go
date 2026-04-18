package commands

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// TestPermissionsCommandMetadata verifies /permissions exposes the migrated alias and descriptor.
func TestPermissionsCommandMetadata(t *testing.T) {
	got := PermissionsCommand{}.Metadata()
	want := command.Metadata{
		Name:        "permissions",
		Aliases:     []string{"allowed-tools"},
		Description: "Manage allow & deny tool permission rules",
		Usage:       "/permissions",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Metadata() = %#v, want %#v", got, want)
	}
}

// TestPermissionsCommandExecuteRendersSummary verifies /permissions reports the current minimal settings-derived summary.
func TestPermissionsCommandExecuteRendersSummary(t *testing.T) {
	result, err := PermissionsCommand{
		Config: coreconfig.Config{
			ApprovalMode: "default",
			Permissions: coreconfig.PermissionConfig{
				DefaultMode: "plan",
				Allow:       []string{"Bash(ls)", "Read(src/**)"},
				Deny:        []string{"Bash(rm -rf)"},
				Ask:         []string{"Edit(*)"},
				AdditionalDirectoryEntries: []coreconfig.AdditionalDirectoryConfig{
					{Path: "packages/app", Source: coreconfig.AdditionalDirectorySourceProjectSettings},
					{Path: "docs", Source: coreconfig.AdditionalDirectorySourceSession},
				},
				AdditionalDirectories:        []string{"packages/app", "docs"},
				DisableBypassPermissionsMode: "disable",
			},
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Permission settings:\n- Default mode: plan\n- Disable bypass-permissions mode: enabled\n- Allow rules: Bash(ls), Read(src/**)\n- Deny rules: Bash(rm -rf)\n- Ask rules: Edit(*)\n- Additional directories: packages/app (projectSettings), docs (session)\nRun /add-dir <path> to persist one extra working directory. Interactive permission rule editing is not available in the Go host yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestPermissionsCommandExecuteFallsBackToApprovalMode verifies the summary still reports one effective mode when permissions.defaultMode is absent.
func TestPermissionsCommandExecuteFallsBackToApprovalMode(t *testing.T) {
	result, err := PermissionsCommand{
		Config: coreconfig.Config{
			ApprovalMode: "acceptEdits",
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got := result.Output; !strings.HasPrefix(got, "Permission settings:\n- Default mode: acceptEdits") {
		t.Fatalf("Execute() output = %q, want acceptEdits fallback", got)
	}
}
