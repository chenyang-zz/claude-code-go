package anthropic

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// AWSAuthenticator abstracts the credential source used for Bedrock authentication.
type AWSAuthenticator interface {
	// SignRequest adds AWS Signature V4 headers to the given HTTP request.
	// The body parameter is the raw request body bytes used for payload hash computation.
	SignRequest(req *http.Request, region string, body []byte) error
}

// DefaultAWSAuthenticator implements AWS Signature V4 signing using environment
// variables AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, and AWS_SESSION_TOKEN.
type DefaultAWSAuthenticator struct{}

// SignRequest signs the HTTP request with AWS Signature V4.
func (a *DefaultAWSAuthenticator) SignRequest(req *http.Request, region string, body []byte) error {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	sessionToken := os.Getenv("AWS_SESSION_TOKEN")

	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("missing AWS credentials (set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY)")
	}

	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	req.Header.Set("x-amz-date", amzDate)
	if sessionToken != "" {
		req.Header.Set("x-amz-security-token", sessionToken)
	}

	payloadHash := sha256Hash(body)

	canonicalHeaders, signedHeaders := buildCanonicalHeaders(req)
	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/bedrock/aws4_request", dateStamp, region)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hash([]byte(canonicalRequest)),
	}, "\n")

	signingKey := getSigningKey(secretKey, dateStamp, region, "bedrock")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credentialScope, signedHeaders, signature,
	)
	req.Header.Set("Authorization", authHeader)

	return nil
}

// noopAWSAuthenticator does not sign requests. It is used when
// CLAUDE_CODE_SKIP_BEDROCK_AUTH is set (testing / proxy scenarios).
type noopAWSAuthenticator struct{}

func (a *noopAWSAuthenticator) SignRequest(_ *http.Request, _ string, _ []byte) error {
	return nil
}

// bearerTokenAWSAuthenticator uses a static Bearer token for Bedrock API key authentication.
type bearerTokenAWSAuthenticator struct {
	token string
}

func (a *bearerTokenAWSAuthenticator) SignRequest(req *http.Request, _ string, _ []byte) error {
	req.Header.Set("Authorization", "Bearer "+a.token)
	return nil
}

// newAWSAuthenticator builds the appropriate authenticator based on environment.
// Priority: Bearer token (AWS_BEARER_TOKEN_BEDROCK) > Skip auth > Default AWS credentials.
func newAWSAuthenticator(skipAuth bool) AWSAuthenticator {
	if skipAuth {
		return &noopAWSAuthenticator{}
	}
	if token := os.Getenv("AWS_BEARER_TOKEN_BEDROCK"); token != "" {
		return &bearerTokenAWSAuthenticator{token: token}
	}
	return &DefaultAWSAuthenticator{}
}

// buildCanonicalHeaders builds the canonical headers and signed headers strings
// for AWS Signature V4.
func buildCanonicalHeaders(req *http.Request) (canonical, signed string) {
	headers := make(map[string]string)
	for k, v := range req.Header {
		lower := strings.ToLower(k)
		// Skip headers that should not be signed
		if lower == "authorization" || lower == "connection" {
			continue
		}
		headers[lower] = strings.TrimSpace(strings.Join(v, ","))
	}

	// Ensure host header is present
	if req.Host != "" {
		headers["host"] = strings.ToLower(req.Host)
	} else if req.URL.Host != "" {
		headers["host"] = strings.ToLower(req.URL.Host)
	}

	var keys []string
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var canonicalParts []string
	var signedParts []string
	for _, k := range keys {
		canonicalParts = append(canonicalParts, k+":"+headers[k])
		signedParts = append(signedParts, k)
	}

	return strings.Join(canonicalParts, "\n") + "\n", strings.Join(signedParts, ";")
}

// getSigningKey derives the AWS Signature V4 signing key from the secret key.
func getSigningKey(secretKey, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

// hmacSHA256 computes the HMAC-SHA256 of data using key.
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// sha256Hash returns the hex-encoded SHA-256 hash of data.
func sha256Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
