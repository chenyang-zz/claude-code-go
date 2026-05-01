package oauth

import (
	"errors"
	"strings"
	"testing"
)

func TestIsOAuthAuthError_StatusMatrix(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   bool
	}{
		{"200 OK", 200, false},
		{"301 redirect", 301, false},
		{"400 bad request", 400, false},
		{"401 unauthorized (auth error)", 401, true},
		{"403 forbidden (not auth in scope)", 403, false},
		{"404 not found", 404, false},
		{"429 rate limited", 429, false},
		{"500 internal", 500, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsOAuthAuthError(tc.status); got != tc.want {
				t.Fatalf("IsOAuthAuthError(%d) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

func TestOAuthTokenExpiredError_Error(t *testing.T) {
	err := &OAuthTokenExpiredError{
		Status:            401,
		FailedAccessToken: "tok-abc",
	}
	msg := err.Error()
	if !strings.Contains(msg, "401") {
		t.Fatalf("expected error message to include status 401, got %q", msg)
	}
	if !strings.Contains(msg, "expired") {
		t.Fatalf("expected error message to mention expiry, got %q", msg)
	}
}

func TestOAuthTokenExpiredError_FailedAccessTokenAccessible(t *testing.T) {
	err := &OAuthTokenExpiredError{
		Status:            401,
		FailedAccessToken: "tok-correlation-key",
	}
	if err.FailedAccessToken != "tok-correlation-key" {
		t.Fatalf("FailedAccessToken roundtrip mismatch: %q", err.FailedAccessToken)
	}
}

func TestErrOAuthRefreshFailed_Identity(t *testing.T) {
	if ErrOAuthRefreshFailed == nil {
		t.Fatal("ErrOAuthRefreshFailed must be non-nil")
	}
	if !errors.Is(ErrOAuthRefreshFailed, ErrOAuthRefreshFailed) {
		t.Fatal("errors.Is must match its own sentinel")
	}
	other := errors.New("other")
	if errors.Is(other, ErrOAuthRefreshFailed) {
		t.Fatal("unrelated error should not match ErrOAuthRefreshFailed")
	}
	if !strings.Contains(ErrOAuthRefreshFailed.Error(), "/login") {
		t.Fatalf("expected sentinel message to direct user to /login, got %q", ErrOAuthRefreshFailed.Error())
	}
}
