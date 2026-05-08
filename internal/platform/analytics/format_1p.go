package analytics

// FirstPartyCoreMetadata is the snake_case core metadata for 1P event logging.
// Go equivalent of the TS FirstPartyEventLoggingCoreMetadata type.
type FirstPartyCoreMetadata struct {
	SessionID       string `json:"session_id"`
	Model           string `json:"model"`
	UserType        string `json:"user_type"`
	Betas           string `json:"betas,omitempty"`
	Entrypoint      string `json:"entrypoint,omitempty"`
	AgentSDKVersion string `json:"agent_sdk_version,omitempty"`
	IsInteractive   bool   `json:"is_interactive"`
	ClientType      string `json:"client_type"`
	SWEBenchRunID   string `json:"swe_bench_run_id,omitempty"`
	SWEBenchInstID  string `json:"swe_bench_instance_id,omitempty"`
	SWEBenchTaskID  string `json:"swe_bench_task_id,omitempty"`
	AgentID         string `json:"agent_id,omitempty"`
	ParentSessionID string `json:"parent_session_id,omitempty"`
	AgentType       string `json:"agent_type,omitempty"`
	TeamName        string `json:"team_name,omitempty"`
}

// FirstPartyEnvMetadata is the environment metadata for 1P event logging.
// Go equivalent of the EnvironmentMetadata proto type.
type FirstPartyEnvMetadata struct {
	Platform               string `json:"platform"`
	PlatformRaw            string `json:"platform_raw"`
	Arch                   string `json:"arch"`
	NodeVersion            string `json:"node_version"`
	Terminal               string `json:"terminal"`
	PackageManagers        string `json:"package_managers"`
	Runtimes               string `json:"runtimes"`
	IsRunningWithBun       bool   `json:"is_running_with_bun"`
	IsCI                   bool   `json:"is_ci"`
	IsClaubbit             bool   `json:"is_claubbit"`
	IsClaudeCodeRemote     bool   `json:"is_claude_code_remote"`
	IsLocalAgentMode       bool   `json:"is_local_agent_mode"`
	IsConductor            bool   `json:"is_conductor"`
	IsGithubAction         bool   `json:"is_github_action"`
	IsClaudeCodeAction     bool   `json:"is_claude_code_action"`
	IsClaudeAiAuth         bool   `json:"is_claude_ai_auth"`
	Version                string `json:"version"`
	VersionBase            string `json:"version_base,omitempty"`
	BuildTime              string `json:"build_time"`
	DeploymentEnvironment  string `json:"deployment_environment"`
	// optional fields
	RemoteEnvironmentType   string   `json:"remote_environment_type,omitempty"`
	ClaudeCodeContainerID   string   `json:"claude_code_container_id,omitempty"`
	ClaudeCodeRemoteSessID  string   `json:"claude_code_remote_session_id,omitempty"`
	Tags                    []string `json:"tags,omitempty"`
	GithubEventName         string   `json:"github_event_name,omitempty"`
	GithubActionsRunnerEnv  string   `json:"github_actions_runner_environment,omitempty"`
	GithubActionsRunnerOS   string   `json:"github_actions_runner_os,omitempty"`
	GithubActionRef         string   `json:"github_action_ref,omitempty"`
	WSLVersion              string   `json:"wsl_version,omitempty"`
	LinuxDistroID           string   `json:"linux_distro_id,omitempty"`
	LinuxDistroVersion      string   `json:"linux_distro_version,omitempty"`
	LinuxKernel             string   `json:"linux_kernel,omitempty"`
	VCS                     string   `json:"vcs,omitempty"`
}

// FirstPartyEventMetadata is the complete metadata for 1P event logging.
// Go equivalent of the TS FirstPartyEventLoggingMetadata type.
type FirstPartyEventMetadata struct {
	Env       FirstPartyEnvMetadata `json:"env"`
	Process   string                `json:"process,omitempty"` // base64-encoded JSON of ProcessMetrics
	Core      FirstPartyCoreMetadata `json:"core"`
	Auth      map[string]string     `json:"auth,omitempty"`
	Additional map[string]any       `json:"additional,omitempty"`
}

// To1PEventFormat converts an EnrichedMetadata to 1P event logging format (snake_case).
// Returns the JSON-serialisable metadata struct for the 1P event logging pipeline.
func To1PEventFormat(meta EnrichedMetadata) FirstPartyEventMetadata {
	// Convert EnvContext to FirstPartyEnvMetadata
	env := FirstPartyEnvMetadata{
		Platform:              meta.Env.Platform,
		PlatformRaw:           meta.Env.PlatformRaw,
		Arch:                  meta.Env.Arch,
		NodeVersion:           meta.Env.NodeVersion,
		Terminal:              meta.Env.Terminal,
		PackageManagers:       meta.Env.PackageMgrs,
		Runtimes:              meta.Env.Runtimes,
		IsRunningWithBun:      meta.Env.IsRunningBun,
		IsCI:                  meta.Env.IsCI,
		IsClaubbit:            meta.Env.IsClaubbit,
		IsClaudeCodeRemote:    meta.Env.IsRemote,
		IsLocalAgentMode:      meta.Env.IsLocalAgent,
		IsConductor:           meta.Env.IsConductor,
		IsGithubAction:        meta.Env.IsGHAction || meta.Env.IsCCAction,
		IsClaudeCodeAction:    meta.Env.IsCCAction,
		IsClaudeAiAuth:        meta.Env.IsClaudeAiAuth,
		Version:               meta.Env.Version,
		VersionBase:           meta.Env.VersionBase,
		BuildTime:             meta.Env.BuildTime,
		DeploymentEnvironment: meta.Env.DeployEnv,
		RemoteEnvironmentType: meta.Env.RemoteEnvType,
		ClaudeCodeContainerID: meta.Env.ClaudeCodeContainerID,
	}

	// Convert optional fields
	if meta.Env.Tags != "" {
		env.Tags = splitAndTrim(meta.Env.Tags, ",")
	}
	if meta.Env.GithubEventName != "" {
		env.GithubEventName = meta.Env.GithubEventName
	}
	if meta.Env.WSLVersion != "" {
		env.WSLVersion = meta.Env.WSLVersion
	}
	if meta.Env.LinuxDistroID != "" {
		env.LinuxDistroID = meta.Env.LinuxDistroID
	}
	if meta.Env.LinuxDistroVersion != "" {
		env.LinuxDistroVersion = meta.Env.LinuxDistroVersion
	}
	if meta.Env.LinuxKernel != "" {
		env.LinuxKernel = meta.Env.LinuxKernel
	}
	if meta.Env.VCS != "" {
		env.VCS = meta.Env.VCS
	}

	// Build core metadata (snake_case)
	core := FirstPartyCoreMetadata{
		SessionID:       meta.SessionID,
		Model:           meta.Model,
		UserType:        meta.UserType,
		IsInteractive:   meta.IsInteractive == "true",
		ClientType:      meta.ClientType,
	}
	if meta.Betas != "" {
		core.Betas = meta.Betas
	}
	if meta.Entrypoint != "" {
		core.Entrypoint = meta.Entrypoint
	}
	if meta.AgentSDKVersion != "" {
		core.AgentSDKVersion = meta.AgentSDKVersion
	}
	if meta.SWEBenchRunID != "" {
		core.SWEBenchRunID = meta.SWEBenchRunID
	}
	if meta.SWEBenchInstID != "" {
		core.SWEBenchInstID = meta.SWEBenchInstID
	}
	if meta.SWEBenchTaskID != "" {
		core.SWEBenchTaskID = meta.SWEBenchTaskID
	}
	if meta.AgentID != "" {
		core.AgentID = meta.AgentID
	}
	if meta.ParentSessionID != "" {
		core.ParentSessionID = meta.ParentSessionID
	}
	if meta.AgentType != "" {
		core.AgentType = meta.AgentType
	}
	if meta.TeamName != "" {
		core.TeamName = meta.TeamName
	}

	// Build additional metadata
	additional := make(map[string]any)
	if meta.RepoRemoteHash != "" {
		additional["rh"] = meta.RepoRemoteHash
	}

	return FirstPartyEventMetadata{
		Env:  env,
		Core: core,
		Additional: additional,
	}
}

// splitAndTrim splits a string by the given separator and trims whitespace.
func splitAndTrim(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = trimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// split splits a string by separator.
func split(s, sep string) []string {
	if s == "" {
		return nil
	}
	var result []string
	n := len(sep)
	start := 0
	for i := 0; i <= len(s)-n; i++ {
		if s[i:i+n] == sep {
			result = append(result, s[start:i])
			start = i + n
			i = start - 1
		}
	}
	result = append(result, s[start:])
	return result
}

// trimSpace removes leading and trailing whitespace.
func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
