package oauth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// DefaultCallbackPath is the URL path the listener captures by default.
// It mirrors the TypeScript src/services/oauth/auth-code-listener.ts default.
const DefaultCallbackPath = "/callback"

// claudeAIInferenceScope is the scope string that selects the Claude.ai
// success page after an automatic flow. Any other scope falls back to the
// console success page. Mirrors shouldUseClaudeAIAuth() on the TS side.
const claudeAIInferenceScope = "user:inference"

// SuccessRedirectURLs configures the post-callback browser landing pages.
// Both fields default to the production endpoints when empty.
type SuccessRedirectURLs struct {
	// ClaudeAI is the redirect target when the granted scopes include
	// `user:inference` (i.e. a Claude.ai login).
	ClaudeAI string
	// Console is the redirect target for console-only logins.
	Console string
}

func (u SuccessRedirectURLs) resolve(scopes []string) string {
	for _, s := range scopes {
		if s == claudeAIInferenceScope {
			if u.ClaudeAI != "" {
				return u.ClaudeAI
			}
			return DefaultClaudeAISuccessURL
		}
	}
	if u.Console != "" {
		return u.Console
	}
	return DefaultConsoleSuccessURL
}

// errAlreadyResolved is returned by waitForAuthorization when a different
// error path (Close, manual override, context cancel) already finalized the
// pending request.
var errAlreadyResolved = errors.New("oauth listener: callback already resolved")

// pendingCallback holds the in-flight HTTP handler for a captured callback.
// The handler blocks until redirectCh receives the final URL or it is closed,
// at which point it writes a 302 response and returns.
type pendingCallback struct {
	redirectCh chan string
	doneCh    chan struct{}
}

// AuthCodeListener boots a localhost HTTP server that captures the OAuth
// provider's redirect, validates the state parameter, and surfaces the
// authorization code to a waiting caller. It mirrors the TypeScript
// AuthCodeListener class in src/services/oauth/auth-code-listener.ts.
type AuthCodeListener struct {
	callbackPath string

	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	port     int
	closed   bool

	// expectedState is the CSRF token required on incoming callbacks. It is
	// installed by WaitForAuthorization and cleared when a callback is
	// resolved (success or failure).
	expectedState string

	// resolvedCh is closed once a code or error has been delivered for the
	// current WaitForAuthorization call.
	resolvedCh chan struct{}
	authCode   string
	authErr    error

	// pending is non-nil while a captured request is parked waiting for the
	// caller to invoke HandleSuccessRedirect or HandleErrorRedirect.
	pending *pendingCallback
}

// NewAuthCodeListener returns a listener that captures requests on the given
// callback path. An empty callbackPath falls back to DefaultCallbackPath.
func NewAuthCodeListener(callbackPath string) *AuthCodeListener {
	if strings.TrimSpace(callbackPath) == "" {
		callbackPath = DefaultCallbackPath
	}
	return &AuthCodeListener{callbackPath: callbackPath}
}

// Port returns the port the listener bound to. Returns 0 before Start is called.
func (l *AuthCodeListener) Port() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.port
}

// CallbackPath returns the URL path the listener captures.
func (l *AuthCodeListener) CallbackPath() string {
	return l.callbackPath
}

// Start binds an HTTP server to localhost on the requested port (0 selects an
// OS-assigned port). The server is started on a background goroutine; the
// returned port is the bound port. Repeated calls return an error.
func (l *AuthCodeListener) Start(port int) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.server != nil {
		return 0, fmt.Errorf("oauth listener: already started on port %d", l.port)
	}
	if l.closed {
		return 0, fmt.Errorf("oauth listener: closed")
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	tcpListener, err := net.Listen("tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("oauth listener: bind %s: %w", addr, err)
	}
	tcpAddr, ok := tcpListener.Addr().(*net.TCPAddr)
	if !ok {
		_ = tcpListener.Close()
		return 0, fmt.Errorf("oauth listener: unexpected addr type %T", tcpListener.Addr())
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", l.handle)
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	l.listener = tcpListener
	l.server = srv
	l.port = tcpAddr.Port

	go func() {
		err := srv.Serve(tcpListener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.WarnCF("oauth", "callback listener serve error", map[string]any{
				"error": err.Error(),
				"port":  l.port,
			})
		}
	}()

	logger.DebugCF("oauth", "callback listener started", map[string]any{
		"port":          l.port,
		"callback_path": l.callbackPath,
	})
	return l.port, nil
}

// WaitForAuthorization blocks until the callback HTTP server captures a valid
// callback (returning the authorization code) or the context expires. The
// expectedState argument enables CSRF protection: the listener rejects any
// callback whose `state` query parameter does not match.
//
// onReady, if non-nil, is invoked on a fresh goroutine immediately after the
// listener arms its handlers; it is used by the OAuthService to open the
// browser or print the manual URL only once the server is ready to accept
// callbacks. WaitForAuthorization is not safe to call concurrently with itself
// on the same listener.
func (l *AuthCodeListener) WaitForAuthorization(ctx context.Context, expectedState string, onReady func() error) (string, error) {
	l.mu.Lock()
	if l.server == nil {
		l.mu.Unlock()
		return "", fmt.Errorf("oauth listener: Start has not been called")
	}
	if l.closed {
		l.mu.Unlock()
		return "", fmt.Errorf("oauth listener: closed")
	}
	if l.resolvedCh != nil {
		l.mu.Unlock()
		return "", fmt.Errorf("oauth listener: WaitForAuthorization is already in progress")
	}
	l.expectedState = expectedState
	l.resolvedCh = make(chan struct{})
	resolved := l.resolvedCh
	l.mu.Unlock()

	if onReady != nil {
		go func() {
			if err := onReady(); err != nil {
				l.mu.Lock()
				if l.authErr == nil && l.authCode == "" {
					l.authErr = fmt.Errorf("oauth listener: onReady: %w", err)
					close(resolved)
					l.resolvedCh = nil
				}
				l.mu.Unlock()
			}
		}()
	}

	select {
	case <-resolved:
	case <-ctx.Done():
		l.mu.Lock()
		if l.authErr == nil && l.authCode == "" {
			l.authErr = ctx.Err()
			close(resolved)
			l.resolvedCh = nil
		}
		l.mu.Unlock()
	}

	l.mu.Lock()
	code := l.authCode
	err := l.authErr
	l.authCode = ""
	l.authErr = nil
	l.expectedState = ""
	l.resolvedCh = nil
	l.mu.Unlock()
	return code, err
}

// HasPendingResponse reports whether a captured request is parked waiting for
// the caller to invoke HandleSuccessRedirect or HandleErrorRedirect. The
// OAuthService uses this to distinguish automatic flows (true) from manual
// flows (false) so it can route the browser to the correct success page.
func (l *AuthCodeListener) HasPendingResponse() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.pending != nil
}

// HandleSuccessRedirect releases any pending request by writing a 302 to the
// scope-appropriate success page. It is a no-op when no request is parked.
func (l *AuthCodeListener) HandleSuccessRedirect(scopes []string, urls SuccessRedirectURLs) {
	l.mu.Lock()
	pending := l.pending
	l.pending = nil
	l.mu.Unlock()

	if pending == nil {
		return
	}
	target := urls.resolve(scopes)
	select {
	case pending.redirectCh <- target:
	default:
	}
	close(pending.redirectCh)
	<-pending.doneCh
}

// HandleErrorRedirect releases any pending request by writing a 302 to the
// Claude.ai success page (mirroring the TypeScript fallback behavior). It is
// a no-op when no request is parked.
func (l *AuthCodeListener) HandleErrorRedirect(urls SuccessRedirectURLs) {
	l.mu.Lock()
	pending := l.pending
	l.pending = nil
	l.mu.Unlock()

	if pending == nil {
		return
	}
	target := urls.ClaudeAI
	if target == "" {
		target = DefaultClaudeAISuccessURL
	}
	select {
	case pending.redirectCh <- target:
	default:
	}
	close(pending.redirectCh)
	<-pending.doneCh
}

// Close shuts down the HTTP server. If a pending request is still parked it
// is released with an error redirect. Subsequent calls are no-ops.
func (l *AuthCodeListener) Close() error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	pending := l.pending
	l.pending = nil
	srv := l.server
	resolved := l.resolvedCh
	l.resolvedCh = nil
	if l.authErr == nil && l.authCode == "" && resolved != nil {
		l.authErr = fmt.Errorf("oauth listener: closed before callback")
		close(resolved)
	}
	l.mu.Unlock()

	if pending != nil {
		target := DefaultClaudeAISuccessURL
		select {
		case pending.redirectCh <- target:
		default:
		}
		close(pending.redirectCh)
		<-pending.doneCh
	}

	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
	return nil
}

// handle is the HTTP handler for the callback server. It validates the path,
// extracts the authorization code, performs the state CSRF check, and parks
// the response writer until the caller decides which page to redirect to.
func (l *AuthCodeListener) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != l.callbackPath {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	l.mu.Lock()
	expected := l.expectedState
	resolved := l.resolvedCh
	closed := l.closed
	l.mu.Unlock()

	if closed {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("OAuth listener closed"))
		return
	}

	if resolved == nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("OAuth listener is not waiting for a callback"))
		return
	}

	if code == "" {
		l.failWaiter(resolved, "Authorization code not found")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Authorization code not found"))
		return
	}

	if state != expected {
		l.failWaiter(resolved, "Invalid state parameter")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Invalid state parameter"))
		return
	}

	pending := &pendingCallback{
		redirectCh: make(chan string, 1),
		doneCh:    make(chan struct{}),
	}

	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	l.pending = pending
	l.authCode = code
	l.authErr = nil
	if l.resolvedCh != nil {
		close(l.resolvedCh)
		l.resolvedCh = nil
	}
	l.mu.Unlock()

	logger.DebugCF("oauth", "callback captured", map[string]any{
		"code_present":  true,
		"state_matches": true,
	})

	target, ok := <-pending.redirectCh
	if !ok || target == "" {
		target = DefaultClaudeAISuccessURL
	}
	w.Header().Set("Location", target)
	w.WriteHeader(http.StatusFound)
	close(pending.doneCh)
}

// failWaiter records a callback-error and unblocks WaitForAuthorization.
func (l *AuthCodeListener) failWaiter(resolved chan struct{}, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.authErr != nil || l.authCode != "" {
		return
	}
	if l.resolvedCh != resolved {
		return
	}
	l.authErr = fmt.Errorf("oauth listener: %s", msg)
	close(l.resolvedCh)
	l.resolvedCh = nil
}
