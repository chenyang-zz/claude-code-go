package growthbook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Default API endpoints and timeouts.
const (
	defaultAPIHost   = "https://api.anthropic.com"
	initTimeout      = 5 * time.Second
	refreshIntervalAnt = 20 * time.Minute
	refreshIntervalDefault = 6 * time.Hour
)

// Client manages the connection to the GrowthBook remote eval API.
type Client struct {
	apiHost      string
	clientKey    string
	httpClient   *http.Client
	authHeaders  map[string]string
	hasAuth      bool
	mu           sync.RWMutex
	initialized  bool
	initErr      error
	stopRefresh  chan struct{}
	refreshDone  chan struct{}
}

var (
	defaultClient     *Client
	clientMu          sync.Mutex
	clientCreatedWithAuth bool
	reinitializing    chan struct{}
	reinitMu          sync.Mutex
)

// ClientConfig holds the configuration for creating a GrowthBook client.
type ClientConfig struct {
	APIHost     string
	ClientKey   string
	AuthHeaders map[string]string
	Enabled     bool
	HTTPClient  *http.Client
}

// newClient creates a new GrowthBook client with the given config.
func newClient(cfg ClientConfig) *Client {
	host := cfg.APIHost
	if host == "" {
		host = defaultAPIHost
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: initTimeout}
	}
	c := &Client{
		apiHost:     host,
		clientKey:   cfg.ClientKey,
		httpClient:  hc,
		authHeaders: cfg.AuthHeaders,
		hasAuth:     len(cfg.AuthHeaders) > 0,
		stopRefresh: make(chan struct{}),
		refreshDone: make(chan struct{}),
	}
	return c
}

// GetDefaultClient returns the default GrowthBook client instance.
func GetDefaultClient() *Client {
	clientMu.Lock()
	defer clientMu.Unlock()
	return defaultClient
}

// SetDefaultClient sets the default client used by the package.
// This is called during initialization.
func SetDefaultClient(c *Client) {
	clientMu.Lock()
	defer clientMu.Unlock()
	defaultClient = c
}

// Init initializes the GrowthBook client by fetching features from the remote API.
// It applies the client key and auth headers from the config.
func (c *Client) Init(ctx context.Context) error {
	c.mu.Lock()
	if c.initialized {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	// Validate client key
	if c.clientKey == "" {
		err := fmt.Errorf("growthbook: client key is required")
		c.mu.Lock()
		c.initErr = err
		c.initialized = true
		c.mu.Unlock()
		return err
	}

	if !c.hasAuth {
		// No auth: rely on disk-cached values, mark as initialized
		c.mu.Lock()
		c.initialized = true
		c.mu.Unlock()
		logger.DebugCF("growthbook", "no auth headers, using disk cache only", nil)
		return nil
	}

	logger.DebugCF("growthbook", "initializing with client key", map[string]interface{}{
		"clientKey": c.clientKey,
	})

	// Fetch features
	features, err := c.fetchRemoteFeatures(ctx)
	if err != nil {
		c.mu.Lock()
		c.initErr = err
		c.initialized = true
		c.mu.Unlock()
		return err
	}

	// Process the payload
	c.processPayload(features)

	c.mu.Lock()
	c.initialized = true
	c.mu.Unlock()

	return nil
}

// fetchRemoteFeatures fetches feature definitions from the GrowthBook API.
func (c *Client) fetchRemoteFeatures(ctx context.Context) ([]byte, error) {
	url := fmt.Sprintf("%s/api/v1/growthbook/features?clientKey=%s", c.apiHost, c.clientKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("growthbook: create request: %w", err)
	}

	for k, v := range c.authHeaders {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("growthbook: fetch features: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("growthbook: fetch features: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("growthbook: read response: %w", err)
	}
	return body, nil
}

// processPayload handles the remote eval payload from the API.
func (c *Client) processPayload(body []byte) {
	var payload struct {
		Features map[string]json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		logger.WarnCF("growthbook", "failed to unmarshal feature payload", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	if len(payload.Features) == 0 {
		logger.DebugCF("growthbook", "empty feature payload, skipping", nil)
		return
	}

	processRemoteEvalPayload(payload.Features)
}

// processRemoteEvalPayload processes the raw feature map from the API,
// handling the malformed API response where value is used instead of defaultValue.
func processRemoteEvalPayload(features map[string]json.RawMessage) {
	featuresByKeyMu.Lock()
	defer featuresByKeyMu.Unlock()

	// Clear before rebuild
	featuresByKey = make(map[string]*featureState)
	experimentData.clear()
	remoteEvalValues.clear()

	for key, raw := range features {
		var fp struct {
			DefaultValue    interface{}              `json:"defaultValue"`
			Value           interface{}              `json:"value"`
			Source          string                   `json:"source"`
			ExperimentResult map[string]interface{}  `json:"experimentResult"`
			Experiment      map[string]interface{}   `json:"experiment"`
		}
		if err := json.Unmarshal(raw, &fp); err != nil {
			logger.WarnCF("growthbook", "skipping malformed feature", map[string]interface{}{
				"feature": key,
				"error":   err.Error(),
			})
			continue
		}

		// Handle malformed API: if value is set but defaultValue is not, treat value as defaultValue
		defaultVal := fp.DefaultValue
		if defaultVal == nil && fp.Value != nil {
			defaultVal = fp.Value
		}

		state := &featureState{
			key:          key,
			defaultValue: defaultVal,
		}

		// Store experiment data for later exposure logging
		if fp.Experiment != nil || fp.ExperimentResult != nil {
			state.hasExperiment = true
			expData := StoredExperimentData{}
			if exp, ok := fp.Experiment["key"]; ok {
				if keyStr, ok := exp.(string); ok {
					expData.ExperimentID = keyStr
				}
			}
			if vr, ok := fp.ExperimentResult["variationId"]; ok {
				if vf, ok := vr.(float64); ok {
					expData.VariationID = int(vf)
				}
			}
			experimentData.set(key, expData)
		}

		featuresByKey[key] = state
		remoteEvalValues.set(key, defaultVal)
	}
}

// ClientKeyFromEnv returns the GrowthBook client key from the environment.
func ClientKeyFromEnv() string {
	// In order of precedence: direct env var > config deduction
	if key := os.Getenv("CLAUDE_CODE_GROWTHBOOK_CLIENT_KEY"); key != "" {
		return key
	}
	return os.Getenv("CLAUDE_CODE_CLIENT_KEY")
}

// RefreshFeatures performs a light refresh - re-fetches features without
// recreating the client.
func (c *Client) RefreshFeatures(ctx context.Context) error {
	features, err := c.fetchRemoteFeatures(ctx)
	if err != nil {
		return err
	}
	c.processPayload(features)
	syncRemoteEvalToDisk()
	refreshed.emit()
	return nil
}

// startPeriodicRefresh begins periodic feature refresh in a background goroutine.
func (c *Client) startPeriodicRefresh() {
	interval := refreshIntervalDefault
	if isAnt() {
		interval = refreshIntervalAnt
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer close(c.refreshDone)
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), initTimeout)
				if err := c.RefreshFeatures(ctx); err != nil {
					logger.WarnCF("growthbook", "periodic refresh failed", map[string]interface{}{
						"error": err.Error(),
					})
				}
				cancel()
			case <-c.stopRefresh:
				ticker.Stop()
				return
			}
		}
	}()
}

// stopPeriodicRefresh stops the background refresh goroutine.
func (c *Client) stopPeriodicRefresh() {
	select {
	case <-c.stopRefresh:
		// Already stopped
	default:
		close(c.stopRefresh)
		<-c.refreshDone
	}
}

// Close cleans up the client resources.
func (c *Client) Close() {
	c.stopPeriodicRefresh()
}

// NewClientKeyFromConfig returns the client key based on the environment.
// This can be replaced by a more sophisticated provider during initialization.
func NewClientKeyFromConfig() string {
	return ClientKeyFromEnv()
}

// collectReinitPromise tracks re-initialization for security gate checks.
func collectReinitPromise() chan struct{} {
	reinitMu.Lock()
	defer reinitMu.Unlock()
	if reinitializing != nil {
		return reinitializing
	}
	reinitializing = make(chan struct{})
	return reinitializing
}

// closeReinitPromise signals that re-initialization is complete.
func closeReinitPromise() {
	reinitMu.Lock()
	defer reinitMu.Unlock()
	if reinitializing != nil {
		close(reinitializing)
		reinitializing = nil
	}
}
