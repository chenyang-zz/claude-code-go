package anthropic

import (
	"fmt"
	"os"
	"strings"
)

// defaultBedrockRegion is the fallback region when no override is configured.
const defaultBedrockRegion = "us-east-1"

// bedrockModelMappings maps canonical first-party model IDs to their Bedrock
// model IDs. These values are extracted from src/utils/model/configs.ts.
//
// The Bedrock ID format follows the cross-region inference pattern:
//   us.anthropic.claude-{model}-{version}-v{N}:0
//
// For the newest model families (opus-4-6, sonnet-4-6) the format is:
//   us.anthropic.claude-{model}-v1
var bedrockModelMappings = map[string]string{
	"claude-3-5-haiku-20241022":   "us.anthropic.claude-3-5-haiku-20241022-v1:0",
	"claude-haiku-4-5-20251001":   "us.anthropic.claude-haiku-4-5-20251001-v1:0",
	"claude-3-5-sonnet-20241022":  "anthropic.claude-3-5-sonnet-20241022-v2:0",
	"claude-3-7-sonnet-20250219":  "us.anthropic.claude-3-7-sonnet-20250219-v1:0",
	"claude-sonnet-4-20250514":    "us.anthropic.claude-sonnet-4-20250514-v1:0",
	"claude-sonnet-4-5-20250929":  "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
	"claude-opus-4-20250514":      "us.anthropic.claude-opus-4-20250514-v1:0",
	"claude-opus-4-1-20250805":    "us.anthropic.claude-opus-4-1-20250805-v1:0",
	"claude-opus-4-5-20251101":    "us.anthropic.claude-opus-4-5-20251101-v1:0",
	"claude-opus-4-6":             "us.anthropic.claude-opus-4-6-v1",
	"claude-sonnet-4-6":           "us.anthropic.claude-sonnet-4-6-v1",
}

// resolveBedrockRegion returns the AWS region for Bedrock requests.
// It follows the same fallback order as the TS implementation:
//   1. AWS_REGION environment variable
//   2. AWS_DEFAULT_REGION environment variable
//   3. Default "us-east-1"
func resolveBedrockRegion() string {
	for _, key := range []string{"AWS_REGION", "aws_region"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	for _, key := range []string{"AWS_DEFAULT_REGION", "aws_default_region"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return defaultBedrockRegion
}

// toBedrockModelID maps a canonical first-party model ID to its Bedrock
// equivalent. If the model is not in the mapping, it is returned as-is
// (allows passthrough of already-Bedrock-formatted IDs).
func toBedrockModelID(model string) string {
	if mapped, ok := bedrockModelMappings[model]; ok {
		return mapped
	}
	return model
}

// buildBedrockEndpoint constructs the Bedrock InvokeModel streaming endpoint URL.
// Format: https://bedrock-runtime.{region}.amazonaws.com/model/{modelId}/invoke-with-response-stream
func buildBedrockEndpoint(region, modelID string) string {
	return buildBedrockEndpointWithHost("", region, modelID)
}

// buildBedrockEndpointWithHost constructs the Bedrock endpoint URL with an
// optional host override. When host is empty it uses the default
// bedrock-runtime.{region}.amazonaws.com host. When host already contains a
// scheme (e.g. "http://localhost:1234") it is used as-is.
func buildBedrockEndpointWithHost(host, region, modelID string) string {
	if host == "" {
		host = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", region)
	}
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}
	return fmt.Sprintf("%s/model/%s/invoke-with-response-stream", host, modelID)
}
