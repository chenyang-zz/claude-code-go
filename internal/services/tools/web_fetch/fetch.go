package web_fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// maxURLLength caps the length of URLs to lower the potential for data exfiltration.
	maxURLLength = 2000
	// maxHTTPContentLength caps the response body size at 10 MB.
	maxHTTPContentLength = 10 * 1024 * 1024
	// fetchTimeout is the timeout for the main HTTP fetch request.
	fetchTimeout = 60 * time.Second
	// maxRedirects caps same-host redirect hops to prevent redirect loops.
	maxRedirects = 10
)

// FetchedContent stores the normalized result of a successful HTTP fetch.
type FetchedContent struct {
	Content       string
	Bytes         int
	Code          int
	CodeText      string
	ContentType   string
	PersistedPath string
	PersistedSize int
}

// RedirectInfo stores the details of a cross-host redirect that the caller must follow manually.
type RedirectInfo struct {
	OriginalUrl  string
	RedirectUrl  string
	StatusCode   int
	StatusText   string
}

// Fetcher performs HTTP GET requests with custom redirect handling and safety checks.
type Fetcher struct {
	// HTTPClient is used for outbound requests. When nil, http.DefaultClient is used.
	HTTPClient *http.Client
	// Cache stores fetched results to avoid repeated network round-trips.
	Cache *Cache
	// UserAgent is sent with every request.
	UserAgent string
}

// NewFetcher builds a Fetcher with the provided cache and sensible defaults.
func NewFetcher(cache *Cache) *Fetcher {
	return &Fetcher{
		HTTPClient: &http.Client{
			Timeout:       fetchTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		},
		Cache:     cache,
		UserAgent: "Claude-Code-Go/1.0",
	}
}

// FetchURLMarkdownContent fetches the given URL, converts HTML to markdown, and returns the result.
// If the URL triggers a cross-host redirect, a *RedirectInfo is returned instead.
func (f *Fetcher) FetchURLMarkdownContent(ctx context.Context, rawURL string) (*FetchedContent, *RedirectInfo, error) {
	if !validateURL(rawURL) {
		return nil, nil, fmt.Errorf("invalid URL")
	}

	// Check cache first.
	if f.Cache != nil {
		if entry, ok := f.Cache.Get(rawURL); ok {
			return &FetchedContent{
				Content:       entry.Content,
				Bytes:         entry.Bytes,
				Code:          entry.Code,
				CodeText:      entry.CodeText,
				ContentType:   entry.ContentType,
				PersistedPath: entry.PersistedPath,
				PersistedSize: entry.PersistedSize,
			}, nil, nil
		}
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, fmt.Errorf("parse URL: %w", err)
	}

	// Upgrade http to https, except for localhost/loopback addresses used in tests.
	upgradedURL := rawURL
	if parsedURL.Scheme == "http" && !isLoopback(parsedURL.Hostname()) {
		parsedURL.Scheme = "https"
		upgradedURL = parsedURL.String()
	}

	resp, redirect, err := f.getWithPermittedRedirects(ctx, upgradedURL, 0)
	if err != nil {
		return nil, nil, err
	}
	if redirect != nil {
		return nil, redirect, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPContentLength+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read response body: %w", err)
	}
	if len(body) > maxHTTPContentLength {
		return nil, nil, fmt.Errorf("response body exceeds maximum allowed size of %d bytes", maxHTTPContentLength)
	}

	contentType := resp.Header.Get("Content-Type")
	bytesLen := len(body)

	var markdownContent string
	if strings.Contains(contentType, "text/html") {
		markdownContent = htmlToMarkdown(string(body))
	} else {
		markdownContent = string(body)
	}

	entry := CacheEntry{
		Bytes:       bytesLen,
		Code:        resp.StatusCode,
		CodeText:    resp.Status,
		Content:     markdownContent,
		ContentType: contentType,
	}

	if f.Cache != nil {
		f.Cache.Set(rawURL, entry)
	}

	return &FetchedContent{
		Content:     markdownContent,
		Bytes:       bytesLen,
		Code:        resp.StatusCode,
		CodeText:    resp.Status,
		ContentType: contentType,
	}, nil, nil
}

// getWithPermittedRedirects performs an HTTP GET and follows redirects that stay on the same host (allowing www. variants).
func (f *Fetcher) getWithPermittedRedirects(ctx context.Context, targetURL string, depth int) (*http.Response, *RedirectInfo, error) {
	if depth > maxRedirects {
		return nil, nil, fmt.Errorf("too many redirects (exceeded %d)", maxRedirects)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "text/markdown, text/html, */*")
	req.Header.Set("User-Agent", f.UserAgent)

	client := f.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch URL: %w", err)
	}

	if isRedirectStatus(resp.StatusCode) {
		location := resp.Header.Get("Location")
		resp.Body.Close()
		if location == "" {
			return nil, nil, fmt.Errorf("redirect missing Location header")
		}

		redirectURL, err := resolveReference(targetURL, location)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve redirect: %w", err)
		}

		if isPermittedRedirect(targetURL, redirectURL) {
			return f.getWithPermittedRedirects(ctx, redirectURL, depth+1)
		}

		statusText := redirectStatusText(resp.StatusCode)
		return nil, &RedirectInfo{
			OriginalUrl: targetURL,
			RedirectUrl: redirectURL,
			StatusCode:  resp.StatusCode,
			StatusText:  statusText,
		}, nil
	}

	return resp, nil, nil
}

// validateURL checks whether the URL is well-formed and safe to fetch.
func validateURL(rawURL string) bool {
	if len(rawURL) > maxURLLength {
		return false
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	if parsed.User != nil {
		return false
	}

	hostname := parsed.Hostname()
	parts := strings.Split(hostname, ".")
	if len(parts) < 2 {
		return false
	}

	return true
}

// isPermittedRedirect reports whether a redirect is safe to follow.
// Allowed redirects add or remove "www." or keep the origin the same.
func isPermittedRedirect(originalURL, redirectURL string) bool {
	orig, err := url.Parse(originalURL)
	if err != nil {
		return false
	}
	redir, err := url.Parse(redirectURL)
	if err != nil {
		return false
	}

	if orig.Scheme != redir.Scheme {
		return false
	}
	if orig.Port() != redir.Port() {
		return false
	}
	if redir.User != nil {
		return false
	}

	stripWww := func(h string) string {
		return strings.TrimPrefix(h, "www.")
	}
	return stripWww(orig.Hostname()) == stripWww(redir.Hostname())
}

func isRedirectStatus(code int) bool {
	switch code {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

func redirectStatusText(code int) string {
	switch code {
	case http.StatusMovedPermanently:
		return "Moved Permanently"
	case http.StatusFound:
		return "Found"
	case http.StatusSeeOther:
		return "See Other"
	case http.StatusTemporaryRedirect:
		return "Temporary Redirect"
	case http.StatusPermanentRedirect:
		return "Permanent Redirect"
	default:
		return "Redirect"
	}
}

func isLoopback(hostname string) bool {
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return true
	}
	return false
}

func resolveReference(base, ref string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(refURL).String(), nil
}
