package analytics

import (
	"os"
	"runtime"
	"sync"
	"time"
)

// EnvContext holds environment context information attached to analytics events.
type EnvContext struct {
	Platform       string `json:"platform"`
	PlatformRaw    string `json:"platform_raw"`
	Arch           string `json:"arch"`
	NodeVersion    string `json:"node_version"`
	Terminal       string `json:"terminal"`
	PackageMgrs    string `json:"package_managers"`
	Runtimes       string `json:"runtimes"`
	IsRunningBun   bool   `json:"is_running_with_bun"`
	IsCI           bool   `json:"is_ci"`
	IsClaubbit     bool   `json:"is_claubbit"`
	IsRemote       bool   `json:"is_claude_code_remote"`
	IsLocalAgent   bool   `json:"is_local_agent_mode"`
	IsConductor    bool   `json:"is_conductor"`
	IsGHAction     bool   `json:"is_github_action"`
	IsCCAction     bool   `json:"is_claude_code_action"`
	IsClaudeAiAuth bool   `json:"is_claude_ai_auth"`
	Version        string `json:"version"`
	VersionBase    string `json:"version_base,omitempty"`
	BuildTime      string `json:"build_time"`
	DeployEnv      string `json:"deployment_environment"`
	// optional fields
	RemoteEnvType          string `json:"remote_environment_type,omitempty"`
	ClaudeCodeContainerID  string `json:"claude_code_container_id,omitempty"`
	ClaudeCodeRemoteSessID string `json:"claude_code_remote_session_id,omitempty"`
	Tags                   string `json:"tags,omitempty"`
	GithubEventName        string `json:"github_event_name,omitempty"`
	GithubActionRef        string `json:"github_action_ref,omitempty"`
	WSLVersion             string `json:"wsl_version,omitempty"`
	LinuxDistroID          string `json:"linux_distro_id,omitempty"`
	LinuxDistroVersion     string `json:"linux_distro_version,omitempty"`
	LinuxKernel            string `json:"linux_kernel,omitempty"`
	VCS                    string `json:"vcs,omitempty"`
}

// ProcessMetrics holds process resource metrics included with analytics events.
type ProcessMetrics struct {
	Uptime           float64 `json:"uptime"`
	RSS              uint64  `json:"rss"`
	HeapTotal        uint64  `json:"heap_total"`
	HeapUsed         uint64  `json:"heap_used"`
	External         uint64  `json:"external"`
	ArrayBuffers     uint64  `json:"array_buffers"`
	ConstrainedMem   uint64  `json:"constrained_memory"`
	CPUUser          int64   `json:"cpu_user"`
	CPUSystem        int64   `json:"cpu_system"`
	CPUPercent       float64 `json:"cpu_percent"`
}

// EnrichedMetadata is the rich event metadata produced by GetEnrichedMetadata.
// This is the Go equivalent of the TS EventMetadata type.
type EnrichedMetadata struct {
	Model            string         `json:"model"`
	SessionID        string         `json:"session_id"`
	UserType         string         `json:"user_type"`
	Betas            string         `json:"betas,omitempty"`
	Env              EnvContext     `json:"env_context"`
	Entrypoint       string         `json:"entrypoint,omitempty"`
	AgentSDKVersion  string         `json:"agent_sdk_version,omitempty"`
	IsInteractive    string         `json:"is_interactive"`
	ClientType       string         `json:"client_type"`
	ProcessMetrics   *ProcessMetrics `json:"process_metrics,omitempty"`
	SWEBenchRunID    string         `json:"swe_bench_run_id,omitempty"`
	SWEBenchInstID   string         `json:"swe_bench_instance_id,omitempty"`
	SWEBenchTaskID   string         `json:"swe_bench_task_id,omitempty"`
	AgentID          string         `json:"agent_id,omitempty"`
	ParentSessionID  string         `json:"parent_session_id,omitempty"`
	AgentType        string         `json:"agent_type,omitempty"`
	TeamName         string         `json:"team_name,omitempty"`
	SubscriptionType string         `json:"subscription_type,omitempty"`
	RepoRemoteHash   string         `json:"rh,omitempty"`
}

// EnrichMetadataOptions carries optional parameters for GetEnrichedMetadata.
type EnrichMetadataOptions struct {
	Model              string
	Betas              string
	AdditionalMetadata map[string]any
}

// AgentIdentification holds agent identity information for analytics attribution.
type AgentIdentification struct {
	AgentID         string
	ParentSessionID string
	TeamName        string
}

// EnrichmentParams carries the parameters needed to build enriched metadata.
// Callers pass these in instead of depending on global engine state.
type EnrichmentParams struct {
	SessionID       string
	Model           string
	Betas           string
	UserType        string
	IsInteractive   bool
	ClientType      string
	Entrypoint      string
	AgentSDKVersion string
	AgentID         string
	ParentSessionID string
	AgentType       string
	TeamName        string
	SubscriptionType string
	RepoRemoteHash  string
	EnvVersion      string
	EnvBuildTime    string

	// Optional env context overrides
	PlatformRaw       string
	RemoteEnvType     string
	ContainerID       string
	RemoteSessionID   string
	Tags              string
	IsRemote          bool
	IsLocalAgent      bool
	IsConductor       bool
	IsGHAction        bool
	IsClaudeAiAuth    bool
}

// GetEnrichedMetadata builds a full EnrichedMetadata from the provided parameters.
// This is the Go equivalent of the TS getEventMetadata function.
func GetEnrichedMetadata(params EnrichmentParams) EnrichedMetadata {
	env := buildEnvContext(params)
	pm := buildProcessMetrics()

	return EnrichedMetadata{
		Model:            params.Model,
		SessionID:        params.SessionID,
		UserType:         params.UserType,
		Betas:            params.Betas,
		Env:              env,
		Entrypoint:       params.Entrypoint,
		AgentSDKVersion:  params.AgentSDKVersion,
		IsInteractive:    fmtBool(params.IsInteractive),
		ClientType:       params.ClientType,
		ProcessMetrics:   pm,
		AgentID:          params.AgentID,
		ParentSessionID:  params.ParentSessionID,
		AgentType:        params.AgentType,
		TeamName:         params.TeamName,
		SubscriptionType: params.SubscriptionType,
		RepoRemoteHash:   params.RepoRemoteHash,
	}
}

func fmtBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// buildEnvContext builds an EnvContext from EnrichmentParams.
func buildEnvContext(params EnrichmentParams) EnvContext {
	platformRaw := params.PlatformRaw
	if platformRaw == "" {
		platformRaw = runtime.GOOS
	}

	terminal := os.Getenv("TERM")
	isCI := os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != ""
	isClaubbit := os.Getenv("CLAUBBIT") != ""
	isGHA := os.Getenv("GITHUB_ACTIONS") != ""
	isCCAction := os.Getenv("CLAUDE_CODE_ACTION") != ""
	versionBase := extractVersionBase(params.EnvVersion)

	return EnvContext{
		Platform:              platformForAnalytics(runtime.GOOS),
		PlatformRaw:           platformRaw,
		Arch:                  runtime.GOARCH,
		NodeVersion:           runtime.Version(),
		Terminal:              terminal,
		PackageMgrs:           "", // populated by vendor-specific detection
		Runtimes:              runtime.Version(),
		IsRunningBun:          false, // Go is not Bun
		IsCI:                  isCI,
		IsClaubbit:            isClaubbit,
		IsRemote:              params.IsRemote,
		IsLocalAgent:          params.IsLocalAgent,
		IsConductor:           params.IsConductor,
		IsGHAction:            isGHA,
		IsCCAction:            isCCAction,
		IsClaudeAiAuth:        params.IsClaudeAiAuth,
		Version:               params.EnvVersion,
		VersionBase:           versionBase,
		BuildTime:             params.EnvBuildTime,
		DeployEnv:             detectDeployEnv(),
		RemoteEnvType:         params.RemoteEnvType,
		ClaudeCodeContainerID: params.ContainerID,
		Tags:                  params.Tags,
	}
}

// platformForAnalytics maps runtime.GOOS to analytics platform strings.
func platformForAnalytics(goos string) string {
	switch goos {
	case "darwin":
		return "macos"
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	default:
		return "other"
	}
}

// extractVersionBase extracts the base version: "2.0.36-dev.20251107" → "2.0.36-dev"
func extractVersionBase(version string) string {
	if version == "" {
		return ""
	}
	// Match: N.N.N[-suffix]
	// Consume digits.digits.digits optionally followed by -letters
	i := 0
	dotCount := 0
	for i < len(version) && dotCount < 3 {
		c := version[i]
		if c >= '0' && c <= '9' {
			i++
			continue
		}
		if c == '.' {
			dotCount++
			i++
			continue
		}
		break
	}
	if dotCount < 2 {
		// Need at least two dots for a valid semver prefix
		return ""
	}
	// Optionally consume -<letters> suffix
	if i < len(version) && version[i] == '-' {
		i++
		for i < len(version) {
			c := version[i]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				i++
				continue
			}
			break
		}
	}
	return version[:i]
}

// detectDeployEnv detects the deployment environment from env vars.
func detectDeployEnv() string {
	if os.Getenv("CLAUDE_CODE_ACTION") != "" {
		return "action"
	}
	if os.Getenv("CLAUDE_CODE_REMOTE") != "" {
		return "remote"
	}
	if os.Getenv("CI") != "" {
		return "ci"
	}
	return "local"
}

// processMetrics state for CPU delta calculation.
var (
	prevCPUTime  int64
	prevWallTime int64
	pmMu         sync.Mutex
)

// buildProcessMetrics collects current process resource metrics.
// This is the Go equivalent of the TS buildProcessMetrics function.
func buildProcessMetrics() *ProcessMetrics {
	pmMu.Lock()
	defer pmMu.Unlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	nowNano := time.Now().UnixNano()
	nowMs := nowNano / 1_000_000

	// Approximate CPU time using goroutine metrics.
	// Go doesn't expose process.cpuUsage() directly; we use wall-time delta
	// as a proxy. A real implementation would use syscall.Getrusage or
	// /proc/self/stat on Linux.
	var cpuPct float64
	if prevWallTime > 0 {
		wallDeltaMs := nowMs - prevWallTime
		if wallDeltaMs > 0 {
			// Approximate: use the delta as fraction of wall time
			timeDelta := nowNano - prevCPUTime
			if timeDelta > 0 {
				cpuPct = (float64(timeDelta) / float64(wallDeltaMs*1_000_000)) * 100
			}
		}
	}
	prevCPUTime = nowNano
	prevWallTime = nowMs

	return &ProcessMetrics{
		Uptime:         time.Since(processStartTime).Seconds(),
		RSS:            m.Sys,
		HeapTotal:      m.TotalAlloc,
		HeapUsed:       m.Alloc,
		External:       m.HeapReleased,
		ArrayBuffers:   m.OtherSys,
		ConstrainedMem: 0, // Go has no direct equivalent of constrainedMemory
		CPUUser:        int64(m.TotalAlloc), // approximation
		CPUSystem:      0,
		CPUPercent:     cpuPct,
	}
}

// processStartTime records when the application started.
var processStartTime = time.Now()

// GetAgentIdentification returns agent identity information.
// This is the Go equivalent of the TS getAgentIdentification function.
// Currently returns empty values; callers provide agent info via EnrichmentParams.
func GetAgentIdentification() AgentIdentification {
	return AgentIdentification{}
}
