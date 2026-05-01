package anthropic

import (
	"fmt"
	"os"
	"strings"
)

// resolveFoundryBaseURL returns the full base URL for Foundry requests.
// It follows the same priority as the TS implementation:
//   1. Explicit baseURL parameter (from Config.FoundryBaseURL)
//   2. ANTHROPIC_FOUNDRY_BASE_URL environment variable
//   3. resource parameter (from Config.FoundryResource)
//   4. ANTHROPIC_FOUNDRY_RESOURCE environment variable
//   5. Error if none is available
func resolveFoundryBaseURL(resource, baseURL string) (string, error) {
	if baseURL != "" {
		return strings.TrimRight(baseURL, "/"), nil
	}
	if v := os.Getenv("ANTHROPIC_FOUNDRY_BASE_URL"); v != "" {
		return strings.TrimRight(v, "/"), nil
	}
	if resource != "" {
		return fmt.Sprintf("https://%s.services.ai.azure.com", resource), nil
	}
	if v := os.Getenv("ANTHROPIC_FOUNDRY_RESOURCE"); v != "" {
		return fmt.Sprintf("https://%s.services.ai.azure.com", v), nil
	}
	return "", fmt.Errorf("missing Foundry endpoint configuration (set ANTHROPIC_FOUNDRY_RESOURCE or ANTHROPIC_FOUNDRY_BASE_URL)")
}

// buildFoundryEndpoint constructs the Foundry messages endpoint URL.
// Format: {baseURL}/anthropic/v1/messages
func buildFoundryEndpoint(baseURL string) string {
	return baseURL + "/anthropic/v1/messages"
}
