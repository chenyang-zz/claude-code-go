package vcr

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// RecordedRequest is the serializable form of an HTTP request for fixture storage.
type RecordedRequest struct {
	Method string            `json:"method"`
	URL    string            `json:"url"`
	Header map[string]string `json:"header"`
	Body   string            `json:"body,omitempty"`
}

// RecordedResponse is the serializable form of an HTTP response for fixture storage.
type RecordedResponse struct {
	StatusCode int               `json:"status_code"`
	Header     map[string]string `json:"header"`
	Body       string            `json:"body"`
}

// RecordedInteraction pairs one request and one response in a fixture.
type RecordedInteraction struct {
	Request  RecordedRequest  `json:"request"`
	Response RecordedResponse `json:"response"`
}

// unstableHeaders are removed from recorded fixtures to keep them deterministic.
var unstableHeaders = map[string]bool{
	"date":                  true,
	"x-request-id":          true,
	"x-request-idempotency": true,
	"set-cookie":            true,
	"x-api-key":             true,
	"authorization":         true,
	"cookie":                true,
}

// RequestHashInput produces the input for fixture hash computation from an HTTP request.
// It dehydrates environment-specific values before hashing.
func RequestHashInput(method, url string, body []byte) string {
	// Use method + path (without query params) + body for the hash key
	path, _, _ := strings.Cut(url, "?")
	raw := fmt.Sprintf("%s %s\n%s", method, path, string(body))
	return DehydrateValue(raw)
}

// TokenCountHashInput produces dehydrated hash input for token-count fixtures.
func TokenCountHashInput(messages, toolsJSON string) string {
	raw := DehydrateTokenCountInput(messages + "\n" + toolsJSON)
	return raw
}

// Recorder wraps an http.RoundTripper to record or replay HTTP interactions.
//
// In passthrough mode (default): delegates to the inner transport.
// In record mode (VCR_RECORD=true): makes real requests, saves fixtures.
// In replay mode (VCR_ENABLED=true): serves fixtures without network calls.
type Recorder struct {
	inner http.RoundTripper
	name  string
}

// NewRecorder creates a new Recorder with the given fixture name and inner transport.
func NewRecorder(name string, inner http.RoundTripper) *Recorder {
	if inner == nil {
		inner = http.DefaultTransport
	}
	return &Recorder{inner: inner, name: name}
}

// RoundTrip implements http.RoundTripper.
func (r *Recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	if !Enabled() {
		return r.inner.RoundTrip(req)
	}

	// Read and buffer the request body for hashing
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("vcr: read request body: %w", err)
		}
		// Restore the body for downstream use
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	hashRaw := RequestHashInput(req.Method, req.URL.String(), bodyBytes)
	hash, err := hashInput(hashRaw)
	if err != nil {
		return nil, err
	}

	if Recording() {
		return r.record(req, bodyBytes, hash)
	}
	return r.replay(hash)
}

func (r *Recorder) record(req *http.Request, bodyBytes []byte, hash string) (*http.Response, error) {
	// NOTE: This reads the entire response body into memory.
	// For SSE streaming responses (Content-Type: text/event-stream), reading
	// the full body blocks until the server closes the connection, which may
	// deadlock for indefinite-length streams. Use WrapModelClient (which
	// records at the model.Event level) for streaming scenarios.
	// Make the real request
	resp, err := r.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vcr: read response body: %w", err)
	}

	recReq := recordedRequest(req, bodyBytes)
	recResp := recordedResponse(resp, respBody)

	interaction := RecordedInteraction{
		Request:  recReq,
		Response: recResp,
	}

	if err := writeFixture(r.name, hash, interaction); err != nil {
		return nil, err
	}

	// Return the response with restored body
	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	return resp, nil
}

func (r *Recorder) replay(hash string) (*http.Response, error) {
	cached, err := readFixture[RecordedInteraction](r.name, hash)
	if err != nil {
		return nil, fmt.Errorf(
			"vcr: fixture missing for %s (hash=%s). "+
				"Re-run tests with VCR_RECORD=true, then commit the result. "+
				"Fixture path: %s", r.name, hash, fixturePath(r.name, hash),
		)
	}

	resp := &http.Response{
		StatusCode: cached.Response.StatusCode,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body: io.NopCloser(bytes.NewReader([]byte(cached.Response.Body))),
	}

	for k, v := range cached.Response.Header {
		resp.Header.Set(k, v)
	}
	if cached.Response.StatusCode == 0 {
		resp.StatusCode = 200
	}
	resp.Status = http.StatusText(resp.StatusCode)

	return resp, nil
}

func recordedRequest(req *http.Request, body []byte) RecordedRequest {
	rr := RecordedRequest{
		Method: req.Method,
		URL:    req.URL.String(),
		Header: make(map[string]string),
	}
	for k, v := range req.Header {
		if !unstableHeaders[strings.ToLower(k)] {
			rr.Header[k] = strings.Join(v, ",")
		}
	}
	if len(body) > 0 {
		rr.Body = string(body)
	}
	return rr
}

func recordedResponse(resp *http.Response, body []byte) RecordedResponse {
	rr := RecordedResponse{
		StatusCode: resp.StatusCode,
		Header:     make(map[string]string),
	}
	for k, v := range resp.Header {
		if !unstableHeaders[strings.ToLower(k)] {
			rr.Header[k] = strings.Join(v, ",")
		}
	}
	if len(body) > 0 {
		rr.Body = string(body)
	}
	return rr
}

// roundTripperFunc adapts a function to http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// NewTestRecorder creates a Recorder that uses an explicit handler for the inner transport.
// Useful in tests where you supply the mock handler directly.
func NewTestRecorder(name string, handler func(*http.Request) (*http.Response, error)) *Recorder {
	return NewRecorder(name, roundTripperFunc(handler))
}

// stripHeadersRe is used internally to clean up auth tokens in recorded fixtures.
var stripHeadersRe = regexp.MustCompile(`(?i)(x-api-key|authorization|bearer):\s*\S+`)

// SanitizeRequestBody strips sensitive headers from request bodies for fixture storage.
func SanitizeRequestBody(body []byte) []byte {
	return stripHeadersRe.ReplaceAll(body, []byte("$1: [REDACTED]"))
}
