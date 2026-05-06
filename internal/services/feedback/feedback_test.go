package feedback

import (
	"strings"
	"testing"
	"time"
)

func TestRedactSensitiveInfo_APIKey(t *testing.T) {
	input := `sk-ant-0123456789abcdef0123456789abcdef0123456789abcdef`
	result := RedactSensitiveInfo(input)
	if strings.Contains(result, "sk-ant") {
		t.Fatalf("API key was not redacted: %q", result)
	}
	if !strings.Contains(result, "[REDACTED_API_KEY]") {
		t.Fatalf("expected [REDACTED_API_KEY] in output, got: %q", result)
	}
}

func TestRedactSensitiveInfo_APIKeyQuoted(t *testing.T) {
	input := `"sk-ant0123456789abcdef0123456789abcdef0123456789abcdef"`
	result := RedactSensitiveInfo(input)
	if strings.Contains(result, "sk-ant") {
		t.Fatalf("quoted API key was not redacted: %q", result)
	}
}

func TestRedactSensitiveInfo_AWSKey(t *testing.T) {
	input := `AWS key: "AWSABCDEFGHIJKLMNOPQRST"`
	result := RedactSensitiveInfo(input)
	if strings.Contains(result, "AWSABCDEFGHI") {
		t.Fatalf("AWS key was not redacted: %q", result)
	}
	if !strings.Contains(result, "[REDACTED_AWS_KEY]") {
		t.Fatalf("expected [REDACTED_AWS_KEY] in output, got: %q", result)
	}
}

func TestRedactSensitiveInfo_AKIA(t *testing.T) {
	input := `AKIA1234567890123456`
	result := RedactSensitiveInfo(input)
	if strings.Contains(result, "AKIA") {
		t.Fatalf("AKIA key was not redacted: %q", result)
	}
}

func TestRedactSensitiveInfo_GCPKey(t *testing.T) {
	// Exact 35-char payload after AIza prefix (AIza + 35 chars = 39 total).
	input := `AIza0123456789abcdef0123456789abcdef012`
	result := RedactSensitiveInfo(input)
	if strings.Contains(result, "AIza") {
		t.Fatalf("GCP key was not redacted: %q", result)
	}
}

func TestRedactSensitiveInfo_GCPAccount(t *testing.T) {
	input := `my-service-account@my-project.iam.gserviceaccount.com`
	result := RedactSensitiveInfo(input)
	if strings.Contains(result, "iam.gserviceaccount.com") {
		t.Fatalf("GCP service account was not redacted: %q", result)
	}
}

func TestRedactSensitiveInfo_XAPIKeyHeader(t *testing.T) {
	input := `x-api-key: my-secret-api-key-value`
	result := RedactSensitiveInfo(input)
	if strings.Contains(result, "my-secret-api-key-value") {
		t.Fatalf("x-api-key value was not redacted: %q", result)
	}
}

func TestRedactSensitiveInfo_AuthorizationBearer(t *testing.T) {
	input := `Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.test`
	result := RedactSensitiveInfo(input)
	if strings.Contains(result, "eyJhbGciOiJIUzI1NiJ9") {
		t.Fatalf("Bearer token was not redacted: %q", result)
	}
}

func TestRedactSensitiveInfo_AWSEnvVar(t *testing.T) {
	input := `AWS_SECRET_ACCESS_KEY = supersecretkeyvalue`
	result := RedactSensitiveInfo(input)
	if strings.Contains(result, "supersecretkeyvalue") {
		t.Fatalf("AWS env var value was not redacted: %q", result)
	}
}

func TestRedactSensitiveInfo_SafeTextUnchanged(t *testing.T) {
	input := `This is a normal description with no sensitive data. Version 1.0.`
	result := RedactSensitiveInfo(input)
	if result != input {
		t.Fatalf("safe text was modified: %q -> %q", input, result)
	}
}

func TestSanitizeErrors_Empty(t *testing.T) {
	result := SanitizeErrors(nil)
	if result != nil {
		t.Fatalf("expected nil for empty input, got %#v", result)
	}
}

func TestSanitizeErrors_RedactsContent(t *testing.T) {
	input := []ErrorInfo{
		{Error: "api key sk-ant-abcdef1234567890abcdef12 in request", Timestamp: "2024-01-01"},
	}
	result := SanitizeErrors(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if strings.Contains(result[0].Error, "sk-ant") {
		t.Fatalf("error was not redacted: %q", result[0].Error)
	}
	if result[0].Timestamp != "2024-01-01" {
		t.Fatalf("timestamp was changed: %q", result[0].Timestamp)
	}
}

func TestNowISO(t *testing.T) {
	now := NowISO()
	_, err := time.Parse(time.RFC3339, now)
	if err != nil {
		t.Fatalf("NowISO returned invalid ISO 8601: %q, err: %v", now, err)
	}
}

func TestCreateFallbackTitle_ShortLine(t *testing.T) {
	title := CreateFallbackTitle("Short bug description")
	if title != "Short bug description" {
		t.Fatalf("expected short line as title, got: %q", title)
	}
}

func TestCreateFallbackTitle_LongLine(t *testing.T) {
	longDesc := "This is a very long bug description that should definitely exceed the sixty character limit in the fallback title generation code path"
	title := CreateFallbackTitle(longDesc)
	if len(title) > 70 {
		t.Fatalf("fallback title too long (%d chars): %q", len(title), title)
	}
	if !strings.HasSuffix(title, "...") {
		t.Fatalf("expected truncated title to end with '...', got: %q", title)
	}
}

func TestCreateFallbackTitle_Empty(t *testing.T) {
	title := CreateFallbackTitle("")
	if title != "Bug Report" {
		t.Fatalf("expected 'Bug Report' for empty input, got: %q", title)
	}
}

func TestCreateFallbackTitle_VeryShort(t *testing.T) {
	title := CreateFallbackTitle("Hi")
	if title != "Bug Report" {
		t.Fatalf("expected 'Bug Report' for very short input, got: %q", title)
	}
}

func TestCreateGitHubIssueURL_Basic(t *testing.T) {
	cfg := DefaultConfig()
	url := CreateGitHubIssueURL(cfg, "fb-123", "Test Title", "Test description", nil)
	if !strings.HasPrefix(url, cfg.GitHubIssuesRepoURL) {
		t.Fatalf("URL doesn't start with repo URL: %q", url)
	}
	if !strings.Contains(url, "fb-123") {
		t.Fatalf("URL missing feedback_id: %q", url)
	}
}
