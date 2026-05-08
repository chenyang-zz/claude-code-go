package analytics

import (
	"os"
	"runtime"
	"testing"
	"time"
)

func TestGetEnrichedMetadataBasic(t *testing.T) {
	params := EnrichmentParams{
		SessionID:     "test-session",
		Model:         "claude-opus-4",
		UserType:      "pro",
		IsInteractive: true,
		ClientType:    "cli",
		EnvVersion:    "2.0.0",
		EnvBuildTime:  "2026-01-01",
	}
	meta := GetEnrichedMetadata(params)
	if meta.SessionID != "test-session" {
		t.Errorf("expected test-session, got %s", meta.SessionID)
	}
	if meta.Model != "claude-opus-4" {
		t.Errorf("expected claude-opus-4, got %s", meta.Model)
	}
	if meta.UserType != "pro" {
		t.Errorf("expected pro, got %s", meta.UserType)
	}
	if meta.IsInteractive != "true" {
		t.Errorf("expected true, got %s", meta.IsInteractive)
	}
	if meta.ClientType != "cli" {
		t.Errorf("expected cli, got %s", meta.ClientType)
	}
}

func TestGetEnrichedMetadataWithOptionalFields(t *testing.T) {
	params := EnrichmentParams{
		SessionID:       "s-1",
		Model:           "claude-sonnet-4",
		Betas:           "max-output-20k",
		UserType:        "max",
		AgentID:         "agent-1",
		TeamName:        "my-team",
		SubscriptionType: "max",
		RepoRemoteHash:  "abc123",
		EnvVersion:      "2.1.0",
		EnvBuildTime:    "2026-05-01",
	}
	meta := GetEnrichedMetadata(params)
	if meta.Betas != "max-output-20k" {
		t.Errorf("expected max-output-20k, got %s", meta.Betas)
	}
	if meta.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", meta.AgentID)
	}
	if meta.TeamName != "my-team" {
		t.Errorf("expected my-team, got %s", meta.TeamName)
	}
	if meta.SubscriptionType != "max" {
		t.Errorf("expected max, got %s", meta.SubscriptionType)
	}
	if meta.RepoRemoteHash != "abc123" {
		t.Errorf("expected abc123, got %s", meta.RepoRemoteHash)
	}
}

func TestBuildEnvContextBasic(t *testing.T) {
	params := EnrichmentParams{
		EnvVersion:   "1.0.0",
		EnvBuildTime: "2026-01-01",
	}
	env := buildEnvContext(params)
	if env.Platform == "" {
		t.Error("expected non-empty platform")
	}
	if env.Arch == "" {
		t.Error("expected non-empty arch")
	}
	if env.Version != "1.0.0" {
		t.Errorf("expected 1.0.0, got %s", env.Version)
	}
}

func TestBuildEnvContextPlatformMapping(t *testing.T) {
	// Test the platform mapping function
	expected := map[string]string{
		"darwin":  "macos",
		"linux":   "linux",
		"windows": "windows",
		"freebsd": "other",
	}
	for from, want := range expected {
		got := platformForAnalytics(from)
		if got != want {
			t.Errorf("platformForAnalytics(%q) = %q, want %q", from, got, want)
		}
	}
}

func TestBuildEnvContextEnvVars(t *testing.T) {
	// Save and restore env
	oldCI := os.Getenv("CI")
	oldGHA := os.Getenv("GITHUB_ACTIONS")
	defer func() {
		os.Setenv("CI", oldCI)
		os.Setenv("GITHUB_ACTIONS", oldGHA)
	}()

	os.Setenv("CI", "true")
	os.Setenv("GITHUB_ACTIONS", "true")

	params := EnrichmentParams{
		EnvVersion:   "1.0.0",
		EnvBuildTime: "2026-01-01",
	}
	env := buildEnvContext(params)
	if !env.IsCI {
		t.Error("expected CI to be true")
	}
	if !env.IsGHAction {
		t.Error("expected GHAction to be true")
	}
}

func TestBuildEnvContextOverridePlatform(t *testing.T) {
	params := EnrichmentParams{
		PlatformRaw:  "darwin",
		EnvVersion:   "1.0.0",
		EnvBuildTime: "2026-01-01",
	}
	env := buildEnvContext(params)
	if env.PlatformRaw != "darwin" {
		t.Errorf("expected darwin, got %s", env.PlatformRaw)
	}
}

func TestBuildProcessMetricsBasic(t *testing.T) {
	pm := buildProcessMetrics()
	if pm == nil {
		t.Fatal("expected non-nil metrics")
	}
	if pm.Uptime <= 0 {
		t.Errorf("expected positive uptime, got %f", pm.Uptime)
	}
	if pm.RSS == 0 {
		t.Error("expected non-zero RSS")
	}
}

func TestProcessStartTime(t *testing.T) {
	if processStartTime.IsZero() {
		t.Error("expected non-zero processStartTime")
	}
	// Should be in the past
	if !processStartTime.Before(timeNow()) {
		t.Error("processStartTime should be in the past")
	}
}

func TestExtractVersionBase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"2.0.36-dev.20251107", "2.0.36-dev"},
		{"1.0.0", "1.0.0"},
		{"0.5.0-beta", "0.5.0-beta"},
		{"", ""},
	}
	for _, tc := range tests {
		result := extractVersionBase(tc.input)
		if result != tc.expected {
			t.Errorf("extractVersionBase(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestDetectDeployEnv(t *testing.T) {
	// Without env vars set, should return "local"
	if detectDeployEnv() != "local" {
		t.Errorf("expected local without env vars, got %s", detectDeployEnv())
	}
}

func TestGetAgentIdentification(t *testing.T) {
	id := GetAgentIdentification()
	// Currently returns empty values
	if id.AgentID != "" {
		t.Errorf("expected empty AgentID, got %s", id.AgentID)
	}
}

func TestFmtBool(t *testing.T) {
	if fmtBool(true) != "true" {
		t.Error("expected true")
	}
	if fmtBool(false) != "false" {
		t.Error("expected false")
	}
}

func TestGetEnrichedMetadataProcessMetrics(t *testing.T) {
	params := EnrichmentParams{
		SessionID:     "s-1",
		Model:         "test-model",
		EnvVersion:    "1.0.0",
		EnvBuildTime:  "2026-01-01",
	}
	meta := GetEnrichedMetadata(params)
	if meta.ProcessMetrics == nil {
		t.Fatal("expected non-nil process metrics")
	}
	if meta.ProcessMetrics.Uptime <= 0 {
		t.Errorf("expected positive uptime, got %f", meta.ProcessMetrics.Uptime)
	}
	if meta.ProcessMetrics.RSS == 0 && meta.ProcessMetrics.HeapTotal == 0 {
		t.Error("expected some memory metrics")
	}
}

func TestGetEnrichedMetadataEnvContext(t *testing.T) {
	params := EnrichmentParams{
		SessionID:     "s-1",
		Model:         "test-model",
		EnvVersion:    "1.0.0",
		EnvBuildTime:  "2026-01-01",
		PlatformRaw:   "darwin",
		IsRemote:      true,
		IsLocalAgent:  true,
		IsConductor:   false,
		RemoteEnvType: "ssh",
	}
	meta := GetEnrichedMetadata(params)
	if meta.Env.PlatformRaw != "darwin" {
		t.Errorf("expected darwin, got %s", meta.Env.PlatformRaw)
	}
	if !meta.Env.IsRemote {
		t.Error("expected IsRemote true")
	}
	if !meta.Env.IsLocalAgent {
		t.Error("expected IsLocalAgent true")
	}
	if meta.Env.RemoteEnvType != "ssh" {
		t.Errorf("expected ssh, got %s", meta.Env.RemoteEnvType)
	}
	// GOARCH should match runtime.GOARCH
	if meta.Env.Arch != runtime.GOARCH {
		t.Errorf("expected %s, got %s", runtime.GOARCH, meta.Env.Arch)
	}
}

// timeNow returns current time for test comparisons
func timeNow() time.Time {
	return time.Now()
}
