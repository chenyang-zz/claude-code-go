package teleport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// FetchEnvironments fetches the list of available environments from the
// Environment API. It mirrors fetchEnvironments() in
// src/utils/teleport/environments.ts.
func FetchEnvironments(ctx context.Context, baseURL, accessToken, orgUUID string) ([]EnvironmentResource, error) {
	url := fmt.Sprintf("%s/v1/environment_providers", strings.TrimRight(baseURL, "/"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("teleport: build fetch environments request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("x-organization-uuid", orgUUID)

	client := &http.Client{Timeout: teleportAPITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("teleport: fetch environments: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("teleport: read fetch environments response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("teleport: fetch environments failed (status %d %s)",
			resp.StatusCode, strings.TrimSpace(http.StatusText(resp.StatusCode)))
	}

	var listResp EnvironmentListResponse
	if err := json.Unmarshal(rawBody, &listResp); err != nil {
		return nil, fmt.Errorf("teleport: decode fetch environments response: %w", err)
	}

	logger.DebugCF("teleport", "fetched environments", map[string]any{
		"count": len(listResp.Environments),
	})

	return listResp.Environments, nil
}

// CreateDefaultCloudEnvironment creates a default anthropic_cloud environment
// for users who have none. It mirrors createDefaultCloudEnvironment() in
// src/utils/teleport/environments.ts.
func CreateDefaultCloudEnvironment(ctx context.Context, baseURL, accessToken, orgUUID, name string) (*EnvironmentResource, error) {
	url := fmt.Sprintf("%s/v1/environment_providers/cloud/create", strings.TrimRight(baseURL, "/"))

	requestBody := map[string]interface{}{
		"name":        name,
		"kind":        "anthropic_cloud",
		"description": "",
		"config": map[string]interface{}{
			"environment_type": "anthropic",
			"cwd":             "/home/user",
			"init_script":     nil,
			"environment":     map[string]string{},
			"languages": []map[string]interface{}{
				{"name": "python", "version": "3.11"},
				{"name": "node", "version": "20"},
			},
			"network_config": map[string]interface{}{
				"allowed_hosts":      []string{},
				"allow_default_hosts": true,
			},
		},
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("teleport: marshal create environment request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("teleport: build create environment request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("anthropic-beta", CCRBetaHeader)
	req.Header.Set("x-organization-uuid", orgUUID)

	client := &http.Client{Timeout: teleportAPITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("teleport: create environment: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("teleport: read create environment response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("teleport: create environment failed (status %d %s)",
			resp.StatusCode, strings.TrimSpace(http.StatusText(resp.StatusCode)))
	}

	var env EnvironmentResource
	if err := json.Unmarshal(rawBody, &env); err != nil {
		return nil, fmt.Errorf("teleport: decode create environment response: %w", err)
	}

	logger.DebugCF("teleport", "created default cloud environment", map[string]any{
		"environment_id": env.EnvironmentID,
		"name":           env.Name,
	})

	return &env, nil
}

// GetEnvironmentSelectionInfo gets information about available environments and
// the currently selected one. It mirrors getEnvironmentSelectionInfo() in
// src/utils/teleport/environmentSelection.ts.
//
// defaultEnvID is the environment ID from user settings (remote.defaultEnvironmentId).
// When non-empty, the function tries to select the matching environment. When empty
// or no match is found, the first non-bridge environment (or the first available)
// is selected instead.
func GetEnvironmentSelectionInfo(ctx context.Context, baseURL, accessToken, orgUUID, defaultEnvID string) (*EnvironmentSelectionInfo, error) {
	environments, err := FetchEnvironments(ctx, baseURL, accessToken, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("teleport: get environment selection info: %w", err)
	}

	if len(environments) == 0 {
		return &EnvironmentSelectionInfo{
			AvailableEnvironments: []EnvironmentResource{},
			SelectedEnvironment:   nil,
			SelectedEnvironmentSource: "",
		}, nil
	}

	var selected *EnvironmentResource
	var source string

	if defaultEnvID != "" {
		for i := range environments {
			if environments[i].EnvironmentID == defaultEnvID {
				selected = &environments[i]
				source = "settings"
				break
			}
		}
	}

	if selected == nil {
		// Prefer non-bridge environments.
		for i := range environments {
			if environments[i].Kind != EnvironmentBridge {
				selected = &environments[i]
				break
			}
		}
		// If only bridge environments exist, use the first one.
		if selected == nil {
			selected = &environments[0]
		}
		source = "default"
	}

	return &EnvironmentSelectionInfo{
		AvailableEnvironments:     environments,
		SelectedEnvironment:       selected,
		SelectedEnvironmentSource: source,
	}, nil
}
