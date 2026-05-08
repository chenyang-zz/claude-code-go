package analytics

import "testing"

func TestTo1PEventFormatBasic(t *testing.T) {
	meta := EnrichedMetadata{
		Model:          "claude-opus-4",
		SessionID:      "sess-1",
		UserType:       "pro",
		IsInteractive:  "true",
		ClientType:     "cli",
		Env: EnvContext{
			Platform:  "macos",
			Arch:      "arm64",
			Version:   "2.0.0",
			BuildTime: "2026-01-01",
			DeployEnv: "local",
		},
	}

	converted := To1PEventFormat(meta)
	if converted.Core.SessionID != "sess-1" {
		t.Errorf("expected sess-1, got %s", converted.Core.SessionID)
	}
	if converted.Core.Model != "claude-opus-4" {
		t.Errorf("expected claude-opus-4, got %s", converted.Core.Model)
	}
	if !converted.Core.IsInteractive {
		t.Error("expected IsInteractive true")
	}
}

func TestTo1PEventFormatCoreFields(t *testing.T) {
	meta := EnrichedMetadata{
		Model:           "claude-sonnet-4",
		SessionID:       "s-2",
		UserType:        "max",
		Betas:           "max-output-20k",
		Entrypoint:      "cli",
		IsInteractive:   "false",
		ClientType:      "ide",
		AgentID:         "agent-1",
		ParentSessionID: "parent-1",
		AgentType:       "teammate",
		TeamName:        "my-team",
		SubscriptionType: "max",
		Env: EnvContext{
			Version:   "2.0.0",
			BuildTime: "2026-01-01",
		},
	}

	converted := To1PEventFormat(meta)
	if converted.Core.Betas != "max-output-20k" {
		t.Errorf("expected max-output-20k, got %s", converted.Core.Betas)
	}
	if converted.Core.Entrypoint != "cli" {
		t.Errorf("expected cli, got %s", converted.Core.Entrypoint)
	}
	if converted.Core.IsInteractive {
		t.Error("expected IsInteractive false")
	}
	if converted.Core.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", converted.Core.AgentID)
	}
	if converted.Core.ParentSessionID != "parent-1" {
		t.Errorf("expected parent-1, got %s", converted.Core.ParentSessionID)
	}
	if converted.Core.AgentType != "teammate" {
		t.Errorf("expected teammate, got %s", converted.Core.AgentType)
	}
	if converted.Core.TeamName != "my-team" {
		t.Errorf("expected my-team, got %s", converted.Core.TeamName)
	}
}

func TestTo1PEventFormatEnvFields(t *testing.T) {
	meta := EnrichedMetadata{
		SessionID:     "s-1",
		Model:         "test",
		UserType:      "pro",
		IsInteractive: "true",
		ClientType:    "cli",
		Env: EnvContext{
			Platform:     "macos",
			PlatformRaw:  "darwin",
			Arch:         "arm64",
			NodeVersion:  "go1.22.0",
			Terminal:     "xterm-256color",
			PackageMgrs:  "brew",
			Runtimes:     "go",
			IsRunningBun: false,
			IsCI:         true,
			IsRemote:     true,
			Version:      "1.0.0",
			BuildTime:    "2026-01-01",
			DeployEnv:    "remote",
			RemoteEnvType: "ssh",
			VCS:          "git",
		},
	}

	converted := To1PEventFormat(meta)
	if converted.Env.Platform != "macos" {
		t.Errorf("expected macos, got %s", converted.Env.Platform)
	}
	if converted.Env.PlatformRaw != "darwin" {
		t.Errorf("expected darwin, got %s", converted.Env.PlatformRaw)
	}
	if converted.Env.Arch != "arm64" {
		t.Errorf("expected arm64, got %s", converted.Env.Arch)
	}
	if converted.Env.IsCI != true {
		t.Error("expected IsCI true")
	}
	if converted.Env.RemoteEnvironmentType != "ssh" {
		t.Errorf("expected ssh, got %s", converted.Env.RemoteEnvironmentType)
	}
	if converted.Env.IsClaudeCodeRemote != true {
		t.Error("expected IsClaudeCodeRemote true")
	}
}

func TestTo1PEventFormatAdditional(t *testing.T) {
	meta := EnrichedMetadata{
		SessionID:      "s-1",
		Model:          "test",
		UserType:       "pro",
		IsInteractive:  "true",
		ClientType:     "cli",
		RepoRemoteHash: "abc123def456",
		Env: EnvContext{
			Version:   "1.0.0",
			BuildTime: "2026-01-01",
		},
	}

	converted := To1PEventFormat(meta)
	if converted.Additional == nil {
		t.Fatal("expected non-nil Additional map")
	}
	rh, ok := converted.Additional["rh"]
	if !ok {
		t.Fatal("expected rh in additional metadata")
	}
	if rh != "abc123def456" {
		t.Errorf("expected abc123def456, got %v", rh)
	}
}

func TestTo1PEventFormatEmptyRepositoryHash(t *testing.T) {
	meta := EnrichedMetadata{
		SessionID:     "s-1",
		Model:         "test",
		UserType:      "pro",
		IsInteractive: "true",
		ClientType:    "cli",
		Env: EnvContext{
			Version:   "1.0.0",
			BuildTime: "2026-01-01",
		},
	}

	converted := To1PEventFormat(meta)
	if converted.Additional == nil {
		// nil is acceptable when there's no additional data
		return
	}
	if len(converted.Additional) > 0 {
		t.Errorf("expected empty additional, got %d items", len(converted.Additional))
	}
}

func TestSplitAndTrim(t *testing.T) {
	result := splitAndTrim("a, b, c", ",")
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}
	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestSplitAndTrimEmpty(t *testing.T) {
	result := splitAndTrim("", ",")
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestSplitAndTrimTrailingComma(t *testing.T) {
	result := splitAndTrim("a,b,", ",")
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
}

func TestTo1PEventFormatSWEBenchFields(t *testing.T) {
	meta := EnrichedMetadata{
		SessionID:      "s-1",
		Model:          "test",
		UserType:       "pro",
		IsInteractive:  "true",
		ClientType:     "cli",
		SWEBenchRunID:  "run-1",
		SWEBenchInstID: "inst-1",
		SWEBenchTaskID: "task-1",
		Env: EnvContext{
			Version:   "1.0.0",
			BuildTime: "2026-01-01",
		},
	}

	converted := To1PEventFormat(meta)
	if converted.Core.SWEBenchRunID != "run-1" {
		t.Errorf("expected run-1, got %s", converted.Core.SWEBenchRunID)
	}
	if converted.Core.SWEBenchInstID != "inst-1" {
		t.Errorf("expected inst-1, got %s", converted.Core.SWEBenchInstID)
	}
	if converted.Core.SWEBenchTaskID != "task-1" {
		t.Errorf("expected task-1, got %s", converted.Core.SWEBenchTaskID)
	}
}
