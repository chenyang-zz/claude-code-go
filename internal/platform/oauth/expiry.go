package oauth

import "time"

// OAuthRefreshBuffer is the duration before a token's nominal expiration at
// which it should be considered expired. It mirrors the TypeScript reference
// `bufferTime = 5 * 60 * 1000` in src/services/oauth/client.ts and accounts
// for clock drift plus the latency of an incoming request.
const OAuthRefreshBuffer = 5 * time.Minute

// IsOAuthTokenExpired returns true when the token whose absolute expiration
// is encoded in `expiresAt` (Unix milliseconds) is already expired or will
// expire within OAuthRefreshBuffer from now.
//
// Special case: when expiresAt is 0 (the zero value used by env-var or
// file-descriptor injected tokens that have no known expiry), the function
// returns false to mirror the TS `expiresAt === null -> false` branch.
//
// The comparison `now + buffer >= expiresAt` matches the TS reference
// exactly, including the inclusive boundary at the buffer edge.
func IsOAuthTokenExpired(expiresAt int64) bool {
	if expiresAt == 0 {
		return false
	}
	now := nowMillis()
	expiresWithBuffer := now + OAuthRefreshBuffer.Milliseconds()
	return expiresWithBuffer >= expiresAt
}
