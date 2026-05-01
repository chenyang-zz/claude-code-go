package anthropic

import (
	"fmt"
	"os"
	"strings"
)

// defaultVertexRegion is the fallback region when no override is configured.
const defaultVertexRegion = "us-east5"

// vertexRegionOverrides maps model name prefixes to their environment variable
// names for region overrides. Order matters: more specific prefixes must come
// before less specific ones.
var vertexRegionOverrides = []struct {
	prefix  string
	envVar  string
}{
	{"claude-haiku-4-5", "VERTEX_REGION_CLAUDE_HAIKU_4_5"},
	{"claude-3-5-haiku", "VERTEX_REGION_CLAUDE_3_5_HAIKU"},
	{"claude-3-5-sonnet", "VERTEX_REGION_CLAUDE_3_5_SONNET"},
	{"claude-3-7-sonnet", "VERTEX_REGION_CLAUDE_3_7_SONNET"},
	{"claude-opus-4-1", "VERTEX_REGION_CLAUDE_4_1_OPUS"},
	{"claude-opus-4", "VERTEX_REGION_CLAUDE_4_0_OPUS"},
	{"claude-sonnet-4-6", "VERTEX_REGION_CLAUDE_4_6_SONNET"},
	{"claude-sonnet-4-5", "VERTEX_REGION_CLAUDE_4_5_SONNET"},
	{"claude-sonnet-4", "VERTEX_REGION_CLAUDE_4_0_SONNET"},
}

// resolveVertexRegion returns the GCP region for a given model.
// It checks model-specific environment variables first, then falls back to
// CLOUD_ML_REGION, then the default region.
func resolveVertexRegion(model string) string {
	for _, override := range vertexRegionOverrides {
		if strings.HasPrefix(model, override.prefix) {
			if v := os.Getenv(override.envVar); v != "" {
				return v
			}
			break
		}
	}
	if v := os.Getenv("CLOUD_ML_REGION"); v != "" {
		return v
	}
	return defaultVertexRegion
}

// buildVertexEndpoint constructs the Vertex AI streaming endpoint URL.
// Format: https://{region}-aiplatform.googleapis.com/v1/projects/{projectId}/locations/{region}/publishers/anthropic/models/{model}:streamRawPredict
func buildVertexEndpoint(region, projectID, model string) string {
	return buildVertexEndpointWithHost("", region, projectID, model)
}

// buildVertexEndpointWithHost constructs the Vertex AI streaming endpoint URL
// with an optional host override. When host is empty it uses the default
// aiplatform.googleapis.com host. When host already contains a scheme
// (e.g. "http://localhost:1234") it is used as-is.
func buildVertexEndpointWithHost(host, region, projectID, model string) string {
	if host == "" {
		host = fmt.Sprintf("https://%s-aiplatform.googleapis.com", region)
	}
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}
	return fmt.Sprintf(
		"%s/v1/projects/%s/locations/%s/publishers/anthropic/models/%s:streamRawPredict",
		host,
		projectID,
		region,
		model,
	)
}
