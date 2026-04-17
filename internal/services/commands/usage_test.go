package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// TestUsageCommandMetadata verifies /usage is exposed with the expected canonical descriptor.
func TestUsageCommandMetadata(t *testing.T) {
	meta := UsageCommand{}.Metadata()
	if meta.Name != "usage" {
		t.Fatalf("Metadata().Name = %q, want usage", meta.Name)
	}
	if meta.Description != "Show plan usage limits" {
		t.Fatalf("Metadata().Description = %q, want usage description", meta.Description)
	}
	if meta.Usage != "/usage" {
		t.Fatalf("Metadata().Usage = %q, want /usage", meta.Usage)
	}
}

// TestUsageCommandExecute verifies /usage returns the stable usage-limit fallback.
func TestUsageCommandExecute(t *testing.T) {
	result, err := UsageCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != usageCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, usageCommandFallback)
	}
}

// TestUsageCommandExecuteAnthropicSnapshot verifies /usage renders one Anthropic quota snapshot when the probe is available.
func TestUsageCommandExecuteAnthropicSnapshot(t *testing.T) {
	result, err := UsageCommand{
		Config: coreconfig.Config{
			Provider: coreconfig.ProviderAnthropic,
			APIKey:   "test-key",
		},
		Probe: stubUsageLimitsProber{
			snapshot: UsageLimitsSnapshot{
				Supported:         true,
				Available:         true,
				Provider:          coreconfig.ProviderAnthropic,
				Status:            "allowed_warning",
				RateLimitType:     "seven_day",
				ResetsAt:          1760000000,
				Utilization:       0.75,
				HasUtilization:    true,
				OverageStatus:     "allowed",
				OverageResetsAt:   1760500000,
				FallbackAvailable: true,
				Summary:           "reachable (HTTP 200 from /v1/messages)",
			},
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output == usageCommandFallback {
		t.Fatalf("Execute() output = fallback, want Anthropic quota snapshot")
	}
}
