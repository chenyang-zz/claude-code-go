package oauth

import (
	"testing"
	"time"
)

func TestIsOAuthTokenExpired_ZeroExpiresAt(t *testing.T) {
	if got := IsOAuthTokenExpired(0); got != false {
		t.Fatalf("expected false for expiresAt=0, got %v", got)
	}
}

func TestIsOAuthTokenExpired_FarFuture(t *testing.T) {
	// 1 hour in the future, well beyond the 5 min buffer.
	expiresAt := time.Now().Add(1 * time.Hour).UnixMilli()
	if got := IsOAuthTokenExpired(expiresAt); got != false {
		t.Fatalf("expected false for token 1h in the future, got %v", got)
	}
}

func TestIsOAuthTokenExpired_AlreadyPast(t *testing.T) {
	// 1 minute in the past.
	expiresAt := time.Now().Add(-1 * time.Minute).UnixMilli()
	if got := IsOAuthTokenExpired(expiresAt); got != true {
		t.Fatalf("expected true for token 1min in the past, got %v", got)
	}
}

func TestIsOAuthTokenExpired_WithinBuffer(t *testing.T) {
	// 4 minutes in the future — within the 5 minute buffer.
	expiresAt := time.Now().Add(4 * time.Minute).UnixMilli()
	if got := IsOAuthTokenExpired(expiresAt); got != true {
		t.Fatalf("expected true for token within 5min buffer, got %v", got)
	}
}

func TestIsOAuthTokenExpired_JustBeyondBuffer(t *testing.T) {
	// 6 minutes in the future — beyond the 5 minute buffer.
	expiresAt := time.Now().Add(6 * time.Minute).UnixMilli()
	if got := IsOAuthTokenExpired(expiresAt); got != false {
		t.Fatalf("expected false for token beyond 5min buffer, got %v", got)
	}
}

func TestOAuthRefreshBufferConstant(t *testing.T) {
	// Match the TypeScript reference value of 5 * 60 * 1000 ms.
	if OAuthRefreshBuffer != 5*time.Minute {
		t.Fatalf("OAuthRefreshBuffer = %v, want 5m", OAuthRefreshBuffer)
	}
	if OAuthRefreshBuffer.Milliseconds() != 5*60*1000 {
		t.Fatalf("OAuthRefreshBuffer = %d ms, want 300000 ms", OAuthRefreshBuffer.Milliseconds())
	}
}
