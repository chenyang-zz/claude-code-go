package commands

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

type mockProviderHealth struct {
	result model.HealthResult
}

func (m *mockProviderHealth) Check(_ context.Context) model.HealthResult {
	return m.result
}

func TestProviderHealthStatus_WithResults(t *testing.T) {
	hc := model.NewHealthChecker()
	hc.Register("anthropic", &mockProviderHealth{result: model.HealthResult{
		Provider: "anthropic", Status: model.HealthStatusHealthy, CheckedAt: time.Now(),
	}})
	hc.Register("openai-compatible", &mockProviderHealth{result: model.HealthResult{
		Provider: "openai-compatible", Status: model.HealthStatusUnhealthy, CheckedAt: time.Now(),
	}})

	cmd := StatusCommand{HealthChecker: hc}
	lines := cmd.providerHealthStatus(context.Background())

	if len(lines) != 1 {
		t.Fatalf("len(lines) = %d, want 1", len(lines))
	}
	if !strings.Contains(lines[0], "anthropic=healthy") {
		t.Fatalf("missing anthropic=healthy in %q", lines[0])
	}
	if !strings.Contains(lines[0], "openai-compatible=unhealthy") {
		t.Fatalf("missing openai-compatible=unhealthy in %q", lines[0])
	}
}

func TestProviderHealthStatus_NilChecker(t *testing.T) {
	cmd := StatusCommand{HealthChecker: nil}
	lines := cmd.providerHealthStatus(context.Background())
	if lines != nil {
		t.Fatalf("expected nil, got %v", lines)
	}
}

func TestProviderHealthStatus_EmptyResults(t *testing.T) {
	cmd := StatusCommand{HealthChecker: model.NewHealthChecker()}
	lines := cmd.providerHealthStatus(context.Background())
	if lines != nil {
		t.Fatalf("expected nil for empty checker, got %v", lines)
	}
}

func TestStatusCommand_Execute_WithHealth(t *testing.T) {
	hc := model.NewHealthChecker()
	hc.Register("anthropic", &mockProviderHealth{result: model.HealthResult{
		Provider: "anthropic", Status: model.HealthStatusHealthy, CheckedAt: time.Now(),
	}})

	cmd := StatusCommand{
		Config:        config.Config{Provider: "anthropic", Model: "claude-test"},
		HealthChecker: hc,
		APIProbe:      nil,
	}
	result, err := cmd.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(result.Output, "Provider health:") {
		t.Fatalf("output missing provider health: %s", result.Output)
	}
	if !strings.Contains(result.Output, "anthropic=healthy") {
		t.Fatalf("output missing anthropic=healthy: %s", result.Output)
	}
}
