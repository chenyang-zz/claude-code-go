package oauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// CredentialsDir is the subdirectory under the user's home directory where
// OAuth credentials are stored. Mirrors the TS side `getClaudeConfigHomeDir()`.
const CredentialsDir = ".claude"

// CredentialsFile is the filename used by the plain-text credential store.
// Matches the TS plainTextStorage default in src/utils/auth.ts.
const CredentialsFile = ".credentials.json"

// CredentialsFileMode is the POSIX permission bits applied to the
// credentials file. Owner read/write only.
const CredentialsFileMode os.FileMode = 0o600

// CredentialsDirMode is the permission applied when the credentials dir is
// auto-created.
const CredentialsDirMode os.FileMode = 0o755

// credentialsBundle is the on-disk schema. Top-level field names are stable
// and only the `claudeAiOauth` slot is owned by this package; other slots
// (if any are added later) round-trip unchanged through Save().
type credentialsBundle struct {
	ClaudeAIOauth *persistedOAuthTokens  `json:"claudeAiOauth,omitempty"`
	Other         map[string]json.RawMessage `json:"-"`
}

// persistedOAuthTokens is the JSON shape stored under `claudeAiOauth`. It
// mirrors the saveOAuthTokensIfNeeded() write set on the TS side.
type persistedOAuthTokens struct {
	AccessToken          string   `json:"accessToken"`
	RefreshToken         string   `json:"refreshToken,omitempty"`
	ExpiresAt            int64    `json:"expiresAt,omitempty"`
	Scopes               []string `json:"scopes,omitempty"`
	SubscriptionType     string   `json:"subscriptionType,omitempty"`
	RateLimitTier        string   `json:"rateLimitTier,omitempty"`
	HasExtraUsageEnabled bool     `json:"hasExtraUsageEnabled,omitempty"`
	BillingType          string   `json:"billingType,omitempty"`
}

// MarshalJSON merges the typed claudeAiOauth slot back together with any
// unknown fields from a previously read bundle.
func (b credentialsBundle) MarshalJSON() ([]byte, error) {
	merged := make(map[string]json.RawMessage, len(b.Other)+1)
	for k, v := range b.Other {
		merged[k] = v
	}
	if b.ClaudeAIOauth != nil {
		raw, err := json.Marshal(b.ClaudeAIOauth)
		if err != nil {
			return nil, err
		}
		merged["claudeAiOauth"] = raw
	} else {
		delete(merged, "claudeAiOauth")
	}
	return json.Marshal(merged)
}

// UnmarshalJSON splits the typed claudeAiOauth slot from the rest of the
// document so that unknown sibling fields round-trip unchanged.
func (b *credentialsBundle) UnmarshalJSON(data []byte) error {
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	b.Other = make(map[string]json.RawMessage, len(raw))
	for k, v := range raw {
		if k == "claudeAiOauth" {
			if string(v) == "null" {
				continue
			}
			var tokens persistedOAuthTokens
			if err := json.Unmarshal(v, &tokens); err != nil {
				return fmt.Errorf("oauth credentials: decode claudeAiOauth: %w", err)
			}
			b.ClaudeAIOauth = &tokens
			continue
		}
		b.Other[k] = v
	}
	return nil
}

// OAuthCredentialStore reads and writes the on-disk credentials file. It
// owns the `claudeAiOauth` slot and is safe to call from a single goroutine
// at a time; concurrent writes are serialized by the OS-level rename.
type OAuthCredentialStore struct {
	homeDir string
}

// NewOAuthCredentialStore constructs a store rooted at homeDir/.claude. An
// empty homeDir is rejected so callers cannot accidentally write to the
// process working directory.
func NewOAuthCredentialStore(homeDir string) (*OAuthCredentialStore, error) {
	if homeDir == "" {
		return nil, fmt.Errorf("oauth credential store: home directory is empty")
	}
	return &OAuthCredentialStore{homeDir: homeDir}, nil
}

// Path returns the absolute credentials file path managed by the store.
func (s *OAuthCredentialStore) Path() string {
	return filepath.Join(s.homeDir, CredentialsDir, CredentialsFile)
}

// Load reads the credentials bundle and returns the projected OAuthTokens
// when present. Returns (nil, nil) when the file does not exist.
func (s *OAuthCredentialStore) Load() (*OAuthTokens, error) {
	path := s.Path()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("oauth credential store: read %s: %w", path, err)
	}
	var bundle credentialsBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("oauth credential store: decode %s: %w", path, err)
	}
	if bundle.ClaudeAIOauth == nil {
		return nil, nil
	}
	tokens := &OAuthTokens{
		AccessToken:          bundle.ClaudeAIOauth.AccessToken,
		RefreshToken:         bundle.ClaudeAIOauth.RefreshToken,
		ExpiresAt:            bundle.ClaudeAIOauth.ExpiresAt,
		Scopes:               append([]string(nil), bundle.ClaudeAIOauth.Scopes...),
		SubscriptionType:     SubscriptionType(bundle.ClaudeAIOauth.SubscriptionType),
		RateLimitTier:        RateLimitTier(bundle.ClaudeAIOauth.RateLimitTier),
		HasExtraUsageEnabled: bundle.ClaudeAIOauth.HasExtraUsageEnabled,
		BillingType:          BillingType(bundle.ClaudeAIOauth.BillingType),
	}
	return tokens, nil
}

// SaveDecision describes whether Save persisted the supplied tokens.
type SaveDecision struct {
	Persisted bool
	// SkipReason explains why a Save call was a no-op. Empty when
	// Persisted is true.
	SkipReason string
}

// Save persists the supplied tokens to disk under the `claudeAiOauth` slot,
// preserving any unknown sibling fields. Mirrors the TS-side
// saveOAuthTokensIfNeeded gating: tokens without `user:inference`, without a
// refresh token, or without an expiry are skipped on the TS side; we expose
// the decision via SaveDecision so callers can log it.
func (s *OAuthCredentialStore) Save(tokens *OAuthTokens) (SaveDecision, error) {
	if tokens == nil {
		return SaveDecision{}, fmt.Errorf("oauth credential store: tokens are nil")
	}
	if !ShouldUseClaudeAIAuth(tokens.Scopes) {
		return SaveDecision{SkipReason: "scope set does not include user:inference"}, nil
	}
	if tokens.RefreshToken == "" || tokens.ExpiresAt == 0 {
		return SaveDecision{SkipReason: "refresh token or expiry missing (inference-only short-lived ticket)"}, nil
	}

	path := s.Path()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, CredentialsDirMode); err != nil {
		return SaveDecision{}, fmt.Errorf("oauth credential store: create directory %s: %w", dir, err)
	}

	bundle := credentialsBundle{Other: map[string]json.RawMessage{}}
	if existing, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(existing, &bundle); err != nil {
			return SaveDecision{}, fmt.Errorf("oauth credential store: decode existing %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return SaveDecision{}, fmt.Errorf("oauth credential store: read existing %s: %w", path, err)
	}

	persisted := &persistedOAuthTokens{
		AccessToken:          tokens.AccessToken,
		RefreshToken:         tokens.RefreshToken,
		ExpiresAt:            tokens.ExpiresAt,
		Scopes:               append([]string(nil), tokens.Scopes...),
		SubscriptionType:     string(tokens.SubscriptionType),
		RateLimitTier:        string(tokens.RateLimitTier),
		HasExtraUsageEnabled: tokens.HasExtraUsageEnabled,
		BillingType:          string(tokens.BillingType),
	}
	// Three-way fallback for subscription type / rate-limit tier so a refresh
	// that fails to fetch profile info doesn't blank out previously known
	// values. Mirrors `tokens.X ?? existingOauth?.X ?? null` on the TS side.
	if persisted.SubscriptionType == "" && bundle.ClaudeAIOauth != nil {
		persisted.SubscriptionType = bundle.ClaudeAIOauth.SubscriptionType
	}
	if persisted.RateLimitTier == "" && bundle.ClaudeAIOauth != nil {
		persisted.RateLimitTier = bundle.ClaudeAIOauth.RateLimitTier
	}
	if persisted.BillingType == "" && bundle.ClaudeAIOauth != nil {
		persisted.BillingType = bundle.ClaudeAIOauth.BillingType
	}
	// HasExtraUsageEnabled is a boolean so we cannot use the empty-string
	// fallback above. We only fall back to the previously persisted bit
	// when the incoming tokens carry no profile envelope at all — i.e.
	// this is a refresh that did not re-fetch the profile and so cannot
	// authoritatively report a new value. When `tokens.Profile != nil`
	// we trust the freshly fetched bit even if it transitioned from
	// true to false (the org-level overage may have just been disabled).
	if tokens.Profile == nil && bundle.ClaudeAIOauth != nil {
		persisted.HasExtraUsageEnabled = bundle.ClaudeAIOauth.HasExtraUsageEnabled
	}
	bundle.ClaudeAIOauth = persisted

	encoded, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return SaveDecision{}, fmt.Errorf("oauth credential store: encode bundle: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".credentials.json.tmp.")
	if err != nil {
		return SaveDecision{}, fmt.Errorf("oauth credential store: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		cleanup()
		return SaveDecision{}, fmt.Errorf("oauth credential store: write temp: %w", err)
	}
	if err := tmp.Chmod(CredentialsFileMode); err != nil {
		_ = tmp.Close()
		cleanup()
		return SaveDecision{}, fmt.Errorf("oauth credential store: chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return SaveDecision{}, fmt.Errorf("oauth credential store: close temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return SaveDecision{}, fmt.Errorf("oauth credential store: rename to %s: %w", path, err)
	}

	// Restore the desired mode after rename in case umask altered it.
	if err := os.Chmod(path, CredentialsFileMode); err != nil {
		return SaveDecision{}, fmt.Errorf("oauth credential store: chmod %s: %w", path, err)
	}
	return SaveDecision{Persisted: true}, nil
}

// Delete removes any stored claudeAiOauth slot. When other sibling fields
// are present the file is rewritten without claudeAiOauth; otherwise the
// file is removed entirely. Returns nil when the file does not exist.
func (s *OAuthCredentialStore) Delete() error {
	path := s.Path()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("oauth credential store: read %s: %w", path, err)
	}

	var bundle credentialsBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return fmt.Errorf("oauth credential store: decode %s: %w", path, err)
	}
	if bundle.ClaudeAIOauth == nil && len(bundle.Other) == 0 {
		// Empty bundle, just remove the file.
		return os.Remove(path)
	}

	bundle.ClaudeAIOauth = nil
	if len(bundle.Other) == 0 {
		return os.Remove(path)
	}

	encoded, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("oauth credential store: encode bundle: %w", err)
	}
	if err := os.WriteFile(path, encoded, CredentialsFileMode); err != nil {
		return fmt.Errorf("oauth credential store: write %s: %w", path, err)
	}
	return os.Chmod(path, CredentialsFileMode)
}
