package vcr

import (
	"net/http"
	"time"
)

// WrapHTTPClient wraps an *http.Client with a VCR Recorder transport.
//
// When VCR is not enabled (neither VCR_ENABLED nor VCR_RECORD is set), it
// returns the inner client unchanged so there is zero overhead in normal runs.
//
// When VCR is enabled, it creates a new client whose transport records or
// replays interactions through the given fixture name.
//
// Example:
//
//	inner := &http.Client{Timeout: 30 * time.Second}
//	client := vcr.WrapHTTPClient("my-test", inner)
//	// client now goes through VCR when VCR is active
func WrapHTTPClient(name string, inner *http.Client) *http.Client {
	if inner == nil {
		inner = http.DefaultClient
	}

	if !Enabled() {
		return inner
	}

	transport := inner.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &http.Client{
		Transport:     NewRecorder(name, transport),
		Timeout:       inner.Timeout,
		CheckRedirect: inner.CheckRedirect,
		Jar:           inner.Jar,
	}
}

// NewRecordClient creates a new http.Client with a VCR Recorder transport.
// Equivalent to WrapHTTPClient with http.DefaultClient.
func NewRecordClient(name string) *http.Client {
	return WrapHTTPClient(name, &http.Client{
		Timeout: 30 * time.Second,
	})
}
