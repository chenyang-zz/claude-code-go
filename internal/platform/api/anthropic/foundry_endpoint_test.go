package anthropic

import (
	"os"
	"strings"
	"testing"
)

func TestResolveFoundryBaseURL_FromBaseURLParam(t *testing.T) {
	got, err := resolveFoundryBaseURL("", "https://custom.azure.com")
	if err != nil {
		t.Fatalf("resolveFoundryBaseURL() error = %v, want nil", err)
	}
	if want := "https://custom.azure.com"; got != want {
		t.Fatalf("resolveFoundryBaseURL() = %q, want %q", got, want)
	}
}

func TestResolveFoundryBaseURL_FromBaseURLEnv(t *testing.T) {
	os.Setenv("ANTHROPIC_FOUNDRY_BASE_URL", "https://env-base-url.azure.com")
	defer os.Unsetenv("ANTHROPIC_FOUNDRY_BASE_URL")

	got, err := resolveFoundryBaseURL("", "")
	if err != nil {
		t.Fatalf("resolveFoundryBaseURL() error = %v, want nil", err)
	}
	if want := "https://env-base-url.azure.com"; got != want {
		t.Fatalf("resolveFoundryBaseURL() = %q, want %q", got, want)
	}
}

func TestResolveFoundryBaseURL_FromResourceParam(t *testing.T) {
	got, err := resolveFoundryBaseURL("my-resource", "")
	if err != nil {
		t.Fatalf("resolveFoundryBaseURL() error = %v, want nil", err)
	}
	if want := "https://my-resource.services.ai.azure.com"; got != want {
		t.Fatalf("resolveFoundryBaseURL() = %q, want %q", got, want)
	}
}

func TestResolveFoundryBaseURL_FromResourceEnv(t *testing.T) {
	os.Setenv("ANTHROPIC_FOUNDRY_RESOURCE", "env-resource")
	defer os.Unsetenv("ANTHROPIC_FOUNDRY_RESOURCE")

	got, err := resolveFoundryBaseURL("", "")
	if err != nil {
		t.Fatalf("resolveFoundryBaseURL() error = %v, want nil", err)
	}
	if want := "https://env-resource.services.ai.azure.com"; got != want {
		t.Fatalf("resolveFoundryBaseURL() = %q, want %q", got, want)
	}
}

func TestResolveFoundryBaseURL_Missing(t *testing.T) {
	_, err := resolveFoundryBaseURL("", "")
	if err == nil {
		t.Fatal("resolveFoundryBaseURL() error = nil, want error for missing configuration")
	}
}

func TestResolveFoundryBaseURL_BaseURLParamOverridesResource(t *testing.T) {
	got, err := resolveFoundryBaseURL("my-resource", "https://override.azure.com")
	if err != nil {
		t.Fatalf("resolveFoundryBaseURL() error = %v, want nil", err)
	}
	if want := "https://override.azure.com"; got != want {
		t.Fatalf("resolveFoundryBaseURL() = %q, want %q", got, want)
	}
}

func TestResolveFoundryBaseURL_BaseURLParamOverridesResourceEnv(t *testing.T) {
	os.Setenv("ANTHROPIC_FOUNDRY_RESOURCE", "env-resource")
	defer os.Unsetenv("ANTHROPIC_FOUNDRY_RESOURCE")

	got, err := resolveFoundryBaseURL("", "https://override.azure.com")
	if err != nil {
		t.Fatalf("resolveFoundryBaseURL() error = %v, want nil", err)
	}
	if want := "https://override.azure.com"; got != want {
		t.Fatalf("resolveFoundryBaseURL() = %q, want %q", got, want)
	}
}

func TestResolveFoundryBaseURL_ResourceParamOverridesResourceEnv(t *testing.T) {
	os.Setenv("ANTHROPIC_FOUNDRY_RESOURCE", "env-resource")
	defer os.Unsetenv("ANTHROPIC_FOUNDRY_RESOURCE")

	got, err := resolveFoundryBaseURL("param-resource", "")
	if err != nil {
		t.Fatalf("resolveFoundryBaseURL() error = %v, want nil", err)
	}
	if want := "https://param-resource.services.ai.azure.com"; got != want {
		t.Fatalf("resolveFoundryBaseURL() = %q, want %q", got, want)
	}
}

func TestResolveFoundryBaseURL_TrailingSlash(t *testing.T) {
	got, err := resolveFoundryBaseURL("", "https://custom.azure.com/")
	if err != nil {
		t.Fatalf("resolveFoundryBaseURL() error = %v, want nil", err)
	}
	if strings.HasSuffix(got, "/") {
		t.Fatalf("resolveFoundryBaseURL() = %q, should not have trailing slash", got)
	}
}

func TestBuildFoundryEndpoint(t *testing.T) {
	endpoint := buildFoundryEndpoint("https://my-resource.services.ai.azure.com")
	want := "https://my-resource.services.ai.azure.com/anthropic/v1/messages"
	if endpoint != want {
		t.Fatalf("buildFoundryEndpoint() = %q, want %q", endpoint, want)
	}
}
