package anthropic

import (
	"os"
	"testing"
)

func TestResolveVertexRegion_Default(t *testing.T) {
	// Clear all env vars that could interfere.
	for _, override := range vertexRegionOverrides {
		os.Unsetenv(override.envVar)
	}
	os.Unsetenv("CLOUD_ML_REGION")

	if got := resolveVertexRegion("claude-sonnet-4-5"); got != defaultVertexRegion {
		t.Fatalf("resolveVertexRegion = %q, want %q", got, defaultVertexRegion)
	}
}

func TestResolveVertexRegion_ModelSpecificOverride(t *testing.T) {
	os.Setenv("VERTEX_REGION_CLAUDE_3_7_SONNET", "us-west1")
	defer os.Unsetenv("VERTEX_REGION_CLAUDE_3_7_SONNET")

	if got := resolveVertexRegion("claude-3-7-sonnet-20250219"); got != "us-west1" {
		t.Fatalf("resolveVertexRegion = %q, want us-west1", got)
	}
}

func TestResolveVertexRegion_GlobalOverride(t *testing.T) {
	os.Setenv("CLOUD_ML_REGION", "europe-west4")
	defer os.Unsetenv("CLOUD_ML_REGION")

	if got := resolveVertexRegion("unknown-model"); got != "europe-west4" {
		t.Fatalf("resolveVertexRegion = %q, want europe-west4", got)
	}
}

func TestResolveVertexRegion_ModelSpecificWinsOverGlobal(t *testing.T) {
	os.Setenv("VERTEX_REGION_CLAUDE_3_7_SONNET", "asia-east1")
	os.Setenv("CLOUD_ML_REGION", "europe-west4")
	defer os.Unsetenv("VERTEX_REGION_CLAUDE_3_7_SONNET")
	defer os.Unsetenv("CLOUD_ML_REGION")

	if got := resolveVertexRegion("claude-3-7-sonnet-20250219"); got != "asia-east1" {
		t.Fatalf("resolveVertexRegion = %q, want asia-east1", got)
	}
}

func TestBuildVertexEndpoint(t *testing.T) {
	endpoint := buildVertexEndpoint("us-east5", "my-project", "claude-3-7-sonnet@20250219")
	want := "https://us-east5-aiplatform.googleapis.com/v1/projects/my-project/locations/us-east5/publishers/anthropic/models/claude-3-7-sonnet@20250219:streamRawPredict"
	if endpoint != want {
		t.Fatalf("buildVertexEndpoint = %q, want %q", endpoint, want)
	}
}
