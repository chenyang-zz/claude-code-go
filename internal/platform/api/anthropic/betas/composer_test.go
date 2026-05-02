package betas

import (
	"testing"
)

func TestCompose_DefaultSonnet(t *testing.T) {
	opts := BetaOptions{Provider: ProviderFirstParty}
	betas := Compose("claude-sonnet-4-6", opts)

	assertContains(t, betas, ClaudeCodeBeta)
	assertContains(t, betas, Context1MBeta)
	assertContains(t, betas, InterleavedThinkingBeta)
	assertContains(t, betas, PromptCachingScopeBeta)
	assertNotContains(t, betas, OAuthBeta)
}

func TestCompose_Haiku(t *testing.T) {
	opts := BetaOptions{Provider: ProviderFirstParty}
	betas := Compose("claude-haiku-4-5", opts)

	assertNotContains(t, betas, ClaudeCodeBeta)
	assertNotContains(t, betas, CLIInternalBeta)
}

func TestCompose_HaikuAgenticQuery(t *testing.T) {
	opts := BetaOptions{Provider: ProviderFirstParty, IsAgenticQuery: true}
	betas := Compose("claude-haiku-4-5", opts)

	assertContains(t, betas, ClaudeCodeBeta)
}

func TestCompose_HaikuAgenticQueryAntCLI(t *testing.T) {
	opts := BetaOptions{Provider: ProviderFirstParty, IsAgenticQuery: true, UserType: "ant", Entrypoint: "cli"}
	betas := Compose("claude-haiku-4-5", opts)

	assertContains(t, betas, ClaudeCodeBeta)
	assertContains(t, betas, CLIInternalBeta)
}

func TestCompose_BedrockFilter(t *testing.T) {
	opts := BetaOptions{Provider: ProviderBedrock}
	all := Compose("claude-sonnet-4-6", opts)

	assertContains(t, all, InterleavedThinkingBeta)
	assertContains(t, all, Context1MBeta)

	httpBetas := GetModelBetas("claude-sonnet-4-6", opts)
	assertNotContains(t, httpBetas, InterleavedThinkingBeta)
	assertNotContains(t, httpBetas, Context1MBeta)

	extra := GetExtraBodyParamsBetas("claude-sonnet-4-6", opts)
	assertContains(t, extra, InterleavedThinkingBeta)
	assertContains(t, extra, Context1MBeta)
}

func TestCompose_VertexWebSearch(t *testing.T) {
	opts := BetaOptions{Provider: ProviderVertex}
	betas := Compose("claude-sonnet-4-6", opts)

	assertContains(t, betas, WebSearchBeta)
}

func TestCompose_VertexNoWebSearchForClaude3(t *testing.T) {
	opts := BetaOptions{Provider: ProviderVertex}
	betas := Compose("claude-3-5-sonnet", opts)

	assertNotContains(t, betas, WebSearchBeta)
}

func TestCompose_Foundry(t *testing.T) {
	opts := BetaOptions{Provider: ProviderFoundry}
	betas := Compose("claude-sonnet-4-6", opts)

	assertContains(t, betas, WebSearchBeta)
	assertContains(t, betas, PromptCachingScopeBeta)
}

func TestCompose_AntOnlyBetas(t *testing.T) {
	opts := BetaOptions{
		Provider: ProviderFirstParty,
		UserType: "ant",
		Entrypoint: "cli",
	}
	betas := Compose("claude-sonnet-4-6", opts)

	assertContains(t, betas, CLIInternalBeta)
}

func TestCompose_ConnectorTextFeatureFlag(t *testing.T) {
	opts := BetaOptions{
		Provider: ProviderFirstParty,
		UserType: "ant",
		FeatureFlags: map[string]bool{"connector_text_summarization": true},
	}
	betas := Compose("claude-sonnet-4-6", opts)

	assertContains(t, betas, SummarizeConnectorTextBeta)
}

func TestCompose_TokenEfficientToolsFeatureFlag(t *testing.T) {
	opts := BetaOptions{
		Provider: ProviderFirstParty,
		UserType: "ant",
		FeatureFlags: map[string]bool{"token_efficient_tools": true},
	}
	betas := Compose("claude-sonnet-4-6", opts)

	assertContains(t, betas, TokenEfficientToolsBeta)
}

func TestCompose_Subscriber(t *testing.T) {
	opts := BetaOptions{Provider: ProviderFirstParty, IsSubscriber: true}
	betas := Compose("claude-sonnet-4-6", opts)

	assertContains(t, betas, OAuthBeta)
}

func TestCompose_1MContext(t *testing.T) {
	opts := BetaOptions{Provider: ProviderFirstParty}

	assertContains(t, Compose("claude-sonnet-4-6", opts), Context1MBeta)
	assertContains(t, Compose("claude-opus-4-6", opts), Context1MBeta)
	assertNotContains(t, Compose("claude-opus-4-1", opts), Context1MBeta)
	assertNotContains(t, Compose("claude-3-5-sonnet", opts), Context1MBeta)
}

func TestCompose_DisableInterleavedThinking(t *testing.T) {
	opts := BetaOptions{Provider: ProviderFirstParty, DisableInterleavedThinking: true}
	betas := Compose("claude-sonnet-4-6", opts)

	assertNotContains(t, betas, InterleavedThinkingBeta)
}

func TestCompose_AnthropicBetasEnv(t *testing.T) {
	opts := BetaOptions{
		Provider:       ProviderFirstParty,
		AnthropicBetas: "custom-beta-1, custom-beta-2",
	}
	betas := Compose("claude-sonnet-4-6", opts)

	assertContains(t, betas, "custom-beta-1")
	assertContains(t, betas, "custom-beta-2")
}

func TestCompose_ExperimentalBetasDisabled(t *testing.T) {
	opts := BetaOptions{Provider: ProviderFirstParty, DisableExperimentalBetas: true}
	betas := Compose("claude-sonnet-4-6", opts)

	assertNotContains(t, betas, PromptCachingScopeBeta)
	assertNotContains(t, betas, RedactThinkingBeta)
}

func TestCompose_ContextManagement(t *testing.T) {
	opts := BetaOptions{Provider: ProviderFirstParty}
	betas := Compose("claude-sonnet-4-6", opts)

	assertContains(t, betas, ContextManagementBeta)
}

func TestCompose_ContextManagementAntOptIn(t *testing.T) {
	opts := BetaOptions{
		Provider:                ProviderFirstParty,
		UserType:                "ant",
		UseAPIContextManagement: true,
	}
	betas := Compose("claude-haiku-4-5", opts)

	assertContains(t, betas, ContextManagementBeta)
}

func TestCompose_StructuredOutputsFeatureFlag(t *testing.T) {
	opts := BetaOptions{
		Provider:     ProviderFirstParty,
		FeatureFlags: map[string]bool{"strict_tools": true},
	}
	betas := Compose("claude-sonnet-4-6", opts)

	assertContains(t, betas, StructuredOutputsBeta)
}

func TestCompose_StructuredOutputsNotOnBedrock(t *testing.T) {
	opts := BetaOptions{
		Provider:     ProviderBedrock,
		FeatureFlags: map[string]bool{"strict_tools": true},
	}
	betas := Compose("claude-sonnet-4-6", opts)

	assertNotContains(t, betas, StructuredOutputsBeta)
}

func TestCompose_CacheMemoization(t *testing.T) {
	ClearCache()
	opts := BetaOptions{Provider: ProviderFirstParty}

	betas1 := Compose("claude-sonnet-4-6", opts)
	betas2 := Compose("claude-sonnet-4-6", opts)

	if len(betas1) == 0 {
		t.Fatal("expected non-empty betas")
	}

	// Same pointer check — memoization should return the cached slice.
	// Note: slice header comparison checks pointer, len, cap.
	if &betas1[0] != &betas2[0] {
		t.Error("expected memoized result to return the same cached slice")
	}
}

func TestGetExtraBodyParamsBetas_NonBedrock(t *testing.T) {
	opts := BetaOptions{Provider: ProviderFirstParty}
	extra := GetExtraBodyParamsBetas("claude-sonnet-4-6", opts)

	if len(extra) != 0 {
		t.Errorf("expected no extra body params for non-Bedrock, got %v", extra)
	}
}

func TestMergeSDKBetas(t *testing.T) {
	modelBetas := []string{"beta-a", "beta-b"}
	sdkBetas := []string{"beta-b", "beta-c"}
	merged := MergeSDKBetas(modelBetas, sdkBetas)

	assertContains(t, merged, "beta-a")
	assertContains(t, merged, "beta-b")
	assertContains(t, merged, "beta-c")

	// Verify no duplicates.
	count := 0
	for _, b := range merged {
		if b == "beta-b" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected beta-b exactly once, got %d", count)
	}
}

func TestMergeSDKBetas_EmptySDK(t *testing.T) {
	modelBetas := []string{"beta-a"}
	merged := MergeSDKBetas(modelBetas, nil)

	if len(merged) != 1 || merged[0] != "beta-a" {
		t.Errorf("expected [beta-a], got %v", merged)
	}
}

func assertContains(t *testing.T, slice []string, item string) {
	t.Helper()
	for _, s := range slice {
		if s == item {
			return
		}
	}
	t.Errorf("expected %q in %v", item, slice)
}

func assertNotContains(t *testing.T, slice []string, item string) {
	t.Helper()
	for _, s := range slice {
		if s == item {
			t.Errorf("expected %q not in %v", item, slice)
			return
		}
	}
}
