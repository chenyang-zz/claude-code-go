package oauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseScopes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"only-spaces", "   ", nil},
		{"single", "user:inference", []string{"user:inference"}},
		{"multiple", "user:profile user:inference user:mcp_servers", []string{"user:profile", "user:inference", "user:mcp_servers"}},
		{"extra-spaces", "  user:profile   user:inference  ", []string{"user:profile", "user:inference"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseScopes(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseScopes(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestShouldUseClaudeAIAuth(t *testing.T) {
	if !ShouldUseClaudeAIAuth([]string{"user:inference"}) {
		t.Fatalf("expected true for [user:inference]")
	}
	if !ShouldUseClaudeAIAuth([]string{"user:profile", "user:inference"}) {
		t.Fatalf("expected true for mixed scopes containing user:inference")
	}
	if ShouldUseClaudeAIAuth([]string{"user:profile"}) {
		t.Fatalf("expected false when user:inference is absent")
	}
	if ShouldUseClaudeAIAuth(nil) {
		t.Fatalf("expected false for empty scopes")
	}
}

func TestBuildAuthURL_ConsoleDefault(t *testing.T) {
	got, err := BuildAuthURL(OAuthAuthURLOptions{
		CodeChallenge: "challenge-1",
		State:         "state-1",
		Port:          5050,
	})
	if err != nil {
		t.Fatalf("BuildAuthURL: %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	if parsed.Scheme+"://"+parsed.Host+parsed.Path != DefaultConsoleAuthorizeURL {
		t.Fatalf("base URL mismatch: %s", got)
	}
	q := parsed.Query()
	if q.Get("code") != "true" || q.Get("response_type") != "code" {
		t.Fatalf("query missing code/response_type: %s", got)
	}
	if q.Get("client_id") != DefaultClientID {
		t.Fatalf("client_id = %q, want %q", q.Get("client_id"), DefaultClientID)
	}
	if q.Get("redirect_uri") != "http://localhost:5050/callback" {
		t.Fatalf("redirect_uri = %q", q.Get("redirect_uri"))
	}
	if q.Get("code_challenge") != "challenge-1" || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("PKCE params missing or wrong: %s", got)
	}
	if q.Get("state") != "state-1" {
		t.Fatalf("state = %q", q.Get("state"))
	}
	if got := q.Get("scope"); got != strings.Join(DefaultAllScopes, " ") {
		t.Fatalf("scope = %q, want default ALL set", got)
	}
}

func TestBuildAuthURL_ClaudeAISwitch(t *testing.T) {
	got, err := BuildAuthURL(OAuthAuthURLOptions{
		CodeChallenge:     "challenge",
		State:             "state",
		Port:              1234,
		LoginWithClaudeAi: true,
	})
	if err != nil {
		t.Fatalf("BuildAuthURL: %v", err)
	}
	if !strings.HasPrefix(got, DefaultClaudeAIAuthorizeURL) {
		t.Fatalf("expected ClaudeAI authorize prefix, got %s", got)
	}
}

func TestBuildAuthURL_ManualRedirect(t *testing.T) {
	got, err := BuildAuthURL(OAuthAuthURLOptions{
		CodeChallenge: "c",
		State:         "s",
		Port:          1111,
		IsManual:      true,
	})
	if err != nil {
		t.Fatalf("BuildAuthURL: %v", err)
	}
	parsed, _ := url.Parse(got)
	if parsed.Query().Get("redirect_uri") != DefaultManualRedirectURL {
		t.Fatalf("manual redirect_uri = %q", parsed.Query().Get("redirect_uri"))
	}
}

func TestBuildAuthURL_InferenceOnlyScope(t *testing.T) {
	got, err := BuildAuthURL(OAuthAuthURLOptions{
		CodeChallenge: "c",
		State:         "s",
		Port:          0,
		InferenceOnly: true,
	})
	if err != nil {
		t.Fatalf("BuildAuthURL: %v", err)
	}
	parsed, _ := url.Parse(got)
	if parsed.Query().Get("scope") != ScopeUserInference {
		t.Fatalf("inference-only scope = %q", parsed.Query().Get("scope"))
	}
}

func TestBuildAuthURL_OptionalQueryParams(t *testing.T) {
	got, err := BuildAuthURL(OAuthAuthURLOptions{
		CodeChallenge:    "c",
		State:            "s",
		Port:             1,
		OrganizationUUID: "org-1",
		LoginHint:        "alice@example.com",
		LoginMethod:      "saml",
	})
	if err != nil {
		t.Fatalf("BuildAuthURL: %v", err)
	}
	parsed, _ := url.Parse(got)
	q := parsed.Query()
	if q.Get("organization_uuid") != "org-1" {
		t.Fatalf("organization_uuid = %q", q.Get("organization_uuid"))
	}
	if q.Get("login_hint") != "alice@example.com" {
		t.Fatalf("login_hint = %q", q.Get("login_hint"))
	}
	if q.Get("login_method") != "saml" {
		t.Fatalf("login_method = %q", q.Get("login_method"))
	}
}

func TestBuildAuthURL_ClientIDOverride(t *testing.T) {
	got, err := BuildAuthURL(OAuthAuthURLOptions{
		CodeChallenge: "c",
		State:         "s",
		Port:          1,
		ClientID:      "override-client",
	})
	if err != nil {
		t.Fatalf("BuildAuthURL: %v", err)
	}
	parsed, _ := url.Parse(got)
	if parsed.Query().Get("client_id") != "override-client" {
		t.Fatalf("client_id override = %q", parsed.Query().Get("client_id"))
	}
}

func TestExchangeCodeForTokens_Success(t *testing.T) {
	const wantCode = "code-1"
	const wantState = "state-1"
	const wantVerifier = "verifier-1"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("anthropic-beta") != OAuthBetaHeader {
			t.Errorf("anthropic-beta = %q", r.Header.Get("anthropic-beta"))
		}
		body, _ := io.ReadAll(r.Body)
		var req OAuthTokenExchangeRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}
		if req.GrantType != "authorization_code" {
			t.Errorf("grant_type = %q", req.GrantType)
		}
		if req.Code != wantCode || req.State != wantState || req.CodeVerifier != wantVerifier {
			t.Errorf("payload mismatch: %+v", req)
		}
		if req.ClientID != DefaultClientID {
			t.Errorf("client_id = %q", req.ClientID)
		}
		if req.RedirectURI != "http://localhost:7000/callback" {
			t.Errorf("redirect_uri = %q", req.RedirectURI)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token": "at-1",
			"refresh_token": "rt-1",
			"expires_in": 3600,
			"scope": "user:profile user:inference",
			"account": {"uuid": "acct-1", "email_address": "u@example.com"},
			"organization": {"uuid": "org-1"}
		}`))
	}))
	defer srv.Close()

	resp, err := ExchangeCodeForTokens(context.Background(), ExchangeCodeForTokensOptions{
		AuthorizationCode: wantCode,
		State:             wantState,
		CodeVerifier:      wantVerifier,
		Port:              7000,
		TokenURL:          srv.URL,
	})
	if err != nil {
		t.Fatalf("ExchangeCodeForTokens: %v", err)
	}
	if resp.AccessToken != "at-1" || resp.RefreshToken != "rt-1" {
		t.Fatalf("token mismatch: %+v", resp)
	}
	if resp.ExpiresIn != 3600 {
		t.Fatalf("expires_in = %d", resp.ExpiresIn)
	}
	if resp.Account == nil || resp.Account.UUID != "acct-1" {
		t.Fatalf("account = %+v", resp.Account)
	}
	if resp.Organization == nil || resp.Organization.UUID != "org-1" {
		t.Fatalf("organization = %+v", resp.Organization)
	}
}

func TestExchangeCodeForTokens_ManualRedirectURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req OAuthTokenExchangeRequest
		_ = json.Unmarshal(body, &req)
		if req.RedirectURI != DefaultManualRedirectURL {
			t.Errorf("manual redirect_uri = %q, want %q", req.RedirectURI, DefaultManualRedirectURL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at","expires_in":1}`))
	}))
	defer srv.Close()

	_, err := ExchangeCodeForTokens(context.Background(), ExchangeCodeForTokensOptions{
		AuthorizationCode: "code",
		State:             "s",
		CodeVerifier:      "v",
		UseManualRedirect: true,
		TokenURL:          srv.URL,
	})
	if err != nil {
		t.Fatalf("ExchangeCodeForTokens: %v", err)
	}
}

func TestExchangeCodeForTokens_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	_, err := ExchangeCodeForTokens(context.Background(), ExchangeCodeForTokensOptions{
		AuthorizationCode: "x",
		State:             "x",
		CodeVerifier:      "x",
		TokenURL:          srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid authorization code") {
		t.Fatalf("expected invalid-authorization-code error, got %v", err)
	}
}

func TestExchangeCodeForTokens_GenericError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := ExchangeCodeForTokens(context.Background(), ExchangeCodeForTokensOptions{
		AuthorizationCode: "x",
		State:             "x",
		CodeVerifier:      "x",
		TokenURL:          srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "token exchange failed") {
		t.Fatalf("expected token-exchange-failed error, got %v", err)
	}
}

func TestExchangeCodeForTokens_MissingAccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"refresh_token":"rt","expires_in":1}`))
	}))
	defer srv.Close()

	_, err := ExchangeCodeForTokens(context.Background(), ExchangeCodeForTokensOptions{
		AuthorizationCode: "x",
		State:             "x",
		CodeVerifier:      "x",
		TokenURL:          srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "missing access_token") {
		t.Fatalf("expected missing-access-token error, got %v", err)
	}
}

func TestExchangeCodeForTokens_ContextDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at","expires_in":1}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := ExchangeCodeForTokens(ctx, ExchangeCodeForTokensOptions{
		AuthorizationCode: "x",
		State:             "x",
		CodeVerifier:      "x",
		TokenURL:          srv.URL,
	})
	if err == nil {
		t.Fatalf("expected context-related error")
	}
}

func TestRefreshToken_Success(t *testing.T) {
	const wantRefreshToken = "old-rt"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]string
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}
		if req["grant_type"] != "refresh_token" {
			t.Errorf("grant_type = %q", req["grant_type"])
		}
		if req["refresh_token"] != wantRefreshToken {
			t.Errorf("refresh_token = %q", req["refresh_token"])
		}
		if req["client_id"] != DefaultClientID {
			t.Errorf("client_id = %q", req["client_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token": "new-at",
			"refresh_token": "new-rt",
			"expires_in": 3600,
			"scope": "user:profile user:inference"
		}`))
	}))
	defer srv.Close()

	resp, err := RefreshToken(context.Background(), RefreshTokenOptions{
		RefreshToken: wantRefreshToken,
		TokenURL:     srv.URL,
	})
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if resp.AccessToken != "new-at" {
		t.Fatalf("access_token = %q, want new-at", resp.AccessToken)
	}
	if resp.RefreshToken != "new-rt" {
		t.Fatalf("refresh_token = %q, want new-rt", resp.RefreshToken)
	}
	if resp.ExpiresIn != 3600 {
		t.Fatalf("expires_in = %d", resp.ExpiresIn)
	}
}

func TestRefreshToken_EmptyRefreshToken(t *testing.T) {
	_, err := RefreshToken(context.Background(), RefreshTokenOptions{RefreshToken: ""})
	if err == nil || !strings.Contains(err.Error(), "refresh token is empty") {
		t.Fatalf("expected empty-refresh-token error, got %v", err)
	}
}

func TestRefreshToken_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	_, err := RefreshToken(context.Background(), RefreshTokenOptions{
		RefreshToken: "rt",
		TokenURL:     srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "token refresh failed") {
		t.Fatalf("expected token-refresh-failed error, got %v", err)
	}
}

func TestRefreshToken_MissingAccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"refresh_token":"rt","expires_in":1}`))
	}))
	defer srv.Close()

	_, err := RefreshToken(context.Background(), RefreshTokenOptions{
		RefreshToken: "rt",
		TokenURL:     srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "missing access_token") {
		t.Fatalf("expected missing-access-token error, got %v", err)
	}
}

func TestRefreshToken_FallbackRefreshToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Server returns a new access token but omits refresh_token
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-at","expires_in":3600}`))
	}))
	defer srv.Close()

	resp, err := RefreshToken(context.Background(), RefreshTokenOptions{
		RefreshToken: "old-rt",
		TokenURL:     srv.URL,
	})
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if resp.RefreshToken != "old-rt" {
		t.Fatalf("refresh_token fallback = %q, want old-rt", resp.RefreshToken)
	}
}

func TestRefreshToken_CustomScopes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]string
		_ = json.Unmarshal(body, &req)
		if req["scope"] != "user:profile" {
			t.Errorf("scope = %q, want user:profile", req["scope"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at","expires_in":1}`))
	}))
	defer srv.Close()

	_, err := RefreshToken(context.Background(), RefreshTokenOptions{
		RefreshToken: "rt",
		Scopes:       []string{"user:profile"},
		TokenURL:     srv.URL,
	})
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
}
