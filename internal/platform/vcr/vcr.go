// Package vcr provides fixture-based recording and replay of API interactions.
//
// It is a Go port of the TypeScript VCR service in src/services/vcr.ts with an
// HTTP transport-level approach that works naturally with Go's http.Client.
//
// # Quick start
//
//	package mytest
//
//	import (
//		"github.com/sheepzhao/claude-code-go/internal/platform/vcr"
//	)
//
//	func TestWithVCR(t *testing.T) {
//		// At low level (HTTP transport)
//		transport := vcr.NewRecorder("my-test", http.DefaultTransport)
//		client := &http.Client{Transport: transport}
//
//		// Or at model.Client level (streaming-friendly)
//		inner := anthropic.NewClient(cfg)
//		wrapped := vcr.WrapModelClient("my-test", inner)
//		stream, _ := wrapped.Stream(ctx, req)
//	}
//
// # Modes
//
//	Passthrough (default) — no recording, no replay; real API calls pass through.
//	Record (VCR_RECORD=true) — makes real API calls and saves responses as fixtures.
//	Replay (VCR_ENABLED=true) — reads fixtures instead of hitting the API.
//
// # Environment variables
//
//	VCR_ENABLED            Enable replay mode (read fixtures, no network calls)
//	VCR_RECORD             Enable record mode (call real API, save fixtures)
//	FORCE_VCR              Force VCR mode (combined replay + record fallback)
//	CLAUDE_CODE_TEST_FIXTURES_ROOT  Fixture directory (default: current directory)
//
// # Usage examples
//
//	# Record mode:
//	VCR_RECORD=true ANTHROPIC_API_KEY=sk-... \
//	  go test ./internal/platform/api/anthropic/ -run TestVCR -v
//
//	# Replay mode (no API key needed):
//	VCR_ENABLED=true \
//	  go test ./internal/platform/api/anthropic/ -run TestVCR -v
//
//	# Normal test run (VCR skipped):
//	go test ./internal/platform/api/anthropic/ -run TestVCR -v
package vcr

import (
	"os"
	"strconv"
)

// Enabled returns true when VCR replay mode is active.
func Enabled() bool {
	return IsEnabled() || Recording()
}

// IsEnabled returns true when the user has explicitly enabled VCR replay mode.
func IsEnabled() bool {
	return isEnvTruthy(os.Getenv("VCR_ENABLED"))
}

// Recording returns true when VCR should record new fixtures instead of replaying.
func Recording() bool {
	return isEnvTruthy(os.Getenv("VCR_RECORD")) ||
		isEnvTruthy(os.Getenv("RECORD"))
}

// ForceVCR returns true when FORCE_VCR is set (overrides normal passthrough).
func ForceVCR() bool {
	return isEnvTruthy(os.Getenv("FORCE_VCR"))
}

// FixtureRoot returns the base directory for fixture files.
// Falls back to the current working directory when
// CLAUDE_CODE_TEST_FIXTURES_ROOT is not set.
func FixtureRoot() string {
	if root := os.Getenv("CLAUDE_CODE_TEST_FIXTURES_ROOT"); root != "" {
		return root
	}
	cwd, _ := os.Getwd()
	return cwd
}

func isEnvTruthy(v string) bool {
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err == nil {
		return b
	}
	return v == "1" || v == "yes" || v == "true"
}
