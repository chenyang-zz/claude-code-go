package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// TestStatsCommandMetadata verifies /stats is exposed with the expected canonical descriptor.
func TestStatsCommandMetadata(t *testing.T) {
	meta := StatsCommand{}.Metadata()
	if meta.Name != "stats" {
		t.Fatalf("Metadata().Name = %q, want stats", meta.Name)
	}
	if meta.Description != "Show your Claude Code usage statistics and activity" {
		t.Fatalf("Metadata().Description = %q, want stats description", meta.Description)
	}
	if meta.Usage != "/stats" {
		t.Fatalf("Metadata().Usage = %q, want /stats", meta.Usage)
	}
}

// TestStatsCommandExecute verifies /stats returns the stable usage-statistics fallback.
func TestStatsCommandExecute(t *testing.T) {
	result, err := StatsCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != statsCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, statsCommandFallback)
	}
}

// TestStatsCommandExecuteAnthropicSnapshot verifies /stats renders one live Anthropic quota snapshot when available.
func TestStatsCommandExecuteAnthropicSnapshot(t *testing.T) {
	result, err := StatsCommand{
		Config: coreconfig.Config{
			Provider: coreconfig.ProviderAnthropic,
			APIKey:   "test-key",
		},
		Probe: stubUsageLimitsProber{
			snapshot: UsageLimitsSnapshot{
				Supported:     true,
				Available:     true,
				Status:        "rejected",
				RateLimitType: "five_hour",
				ResetsAt:      1760000000,
				OverageStatus: "rejected",
				Summary:       "reachable (HTTP 429 from /v1/messages)",
			},
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output == statsCommandFallback {
		t.Fatalf("Execute() output = fallback, want Anthropic quota snapshot")
	}
}
