package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// TestExtraUsageCommandMetadata verifies /extra-usage is exposed with the expected canonical descriptor.
func TestExtraUsageCommandMetadata(t *testing.T) {
	meta := ExtraUsageCommand{}.Metadata()
	if meta.Name != "extra-usage" {
		t.Fatalf("Metadata().Name = %q, want extra-usage", meta.Name)
	}
	if meta.Description != "Configure extra usage to keep working when limits are hit" {
		t.Fatalf("Metadata().Description = %q, want extra-usage description", meta.Description)
	}
	if meta.Usage != "/extra-usage" {
		t.Fatalf("Metadata().Usage = %q, want /extra-usage", meta.Usage)
	}
}

// TestExtraUsageCommandExecute verifies /extra-usage returns the stable browser-flow fallback.
func TestExtraUsageCommandExecute(t *testing.T) {
	result, err := ExtraUsageCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != extraUsageCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, extraUsageCommandFallback)
	}
}

// TestExtraUsageCommandExecuteAnthropicSnapshot verifies /extra-usage renders one live Anthropic overage summary when available.
func TestExtraUsageCommandExecuteAnthropicSnapshot(t *testing.T) {
	result, err := ExtraUsageCommand{
		Config: coreconfig.Config{
			Provider:  coreconfig.ProviderAnthropic,
			AuthToken: "auth-token",
		},
		Probe: stubUsageLimitsProber{
			snapshot: UsageLimitsSnapshot{
				Supported:             true,
				Available:             true,
				Status:                "rejected",
				OverageStatus:         "rejected",
				OverageDisabledReason: "out_of_credits",
				Summary:               "reachable (HTTP 429 from /v1/messages)",
			},
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output == extraUsageCommandFallback {
		t.Fatalf("Execute() output = fallback, want Anthropic overage snapshot")
	}
}
