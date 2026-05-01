package anthropic

import (
	"os"
	"testing"
)

func TestResolveBedrockRegion_Default(t *testing.T) {
	os.Unsetenv("AWS_REGION")
	os.Unsetenv("AWS_DEFAULT_REGION")

	if got := resolveBedrockRegion(); got != defaultBedrockRegion {
		t.Fatalf("resolveBedrockRegion = %q, want %q", got, defaultBedrockRegion)
	}
}

func TestResolveBedrockRegion_AWSRegion(t *testing.T) {
	os.Setenv("AWS_REGION", "us-west-2")
	defer os.Unsetenv("AWS_REGION")
	os.Unsetenv("AWS_DEFAULT_REGION")

	if got := resolveBedrockRegion(); got != "us-west-2" {
		t.Fatalf("resolveBedrockRegion = %q, want us-west-2", got)
	}
}

func TestResolveBedrockRegion_AWSDefaultRegion(t *testing.T) {
	os.Unsetenv("AWS_REGION")
	os.Setenv("AWS_DEFAULT_REGION", "eu-west-1")
	defer os.Unsetenv("AWS_DEFAULT_REGION")

	if got := resolveBedrockRegion(); got != "eu-west-1" {
		t.Fatalf("resolveBedrockRegion = %q, want eu-west-1", got)
	}
}

func TestResolveBedrockRegion_AWSRegionWinsOverDefault(t *testing.T) {
	os.Setenv("AWS_REGION", "ap-northeast-1")
	os.Setenv("AWS_DEFAULT_REGION", "eu-west-1")
	defer os.Unsetenv("AWS_REGION")
	defer os.Unsetenv("AWS_DEFAULT_REGION")

	if got := resolveBedrockRegion(); got != "ap-northeast-1" {
		t.Fatalf("resolveBedrockRegion = %q, want ap-northeast-1", got)
	}
}

func TestToBedrockModelID_KnownModel(t *testing.T) {
	got := toBedrockModelID("claude-sonnet-4-5-20250929")
	want := "us.anthropic.claude-sonnet-4-5-20250929-v1:0"
	if got != want {
		t.Fatalf("toBedrockModelID = %q, want %q", got, want)
	}
}

func TestToBedrockModelID_LatestModel(t *testing.T) {
	got := toBedrockModelID("claude-opus-4-6")
	want := "us.anthropic.claude-opus-4-6-v1"
	if got != want {
		t.Fatalf("toBedrockModelID = %q, want %q", got, want)
	}
}

func TestToBedrockModelID_UnknownPassthrough(t *testing.T) {
	got := toBedrockModelID("some-unknown-model")
	if got != "some-unknown-model" {
		t.Fatalf("toBedrockModelID = %q, want passthrough", got)
	}
}

func TestBuildBedrockEndpoint(t *testing.T) {
	endpoint := buildBedrockEndpoint("us-east-1", "us.anthropic.claude-sonnet-4-5-v1:0")
	want := "https://bedrock-runtime.us-east-1.amazonaws.com/model/us.anthropic.claude-sonnet-4-5-v1:0/invoke-with-response-stream"
	if endpoint != want {
		t.Fatalf("buildBedrockEndpoint = %q, want %q", endpoint, want)
	}
}

func TestBuildBedrockEndpointWithHost_CustomHost(t *testing.T) {
	endpoint := buildBedrockEndpointWithHost("http://localhost:8080", "us-east-1", "anthropic.claude-test-v1:0")
	want := "http://localhost:8080/model/anthropic.claude-test-v1:0/invoke-with-response-stream"
	if endpoint != want {
		t.Fatalf("buildBedrockEndpointWithHost = %q, want %q", endpoint, want)
	}
}

func TestBuildBedrockEndpointWithHost_NoScheme(t *testing.T) {
	endpoint := buildBedrockEndpointWithHost("localhost:8080", "us-east-1", "model-id")
	want := "https://localhost:8080/model/model-id/invoke-with-response-stream"
	if endpoint != want {
		t.Fatalf("buildBedrockEndpointWithHost = %q, want %q", endpoint, want)
	}
}
