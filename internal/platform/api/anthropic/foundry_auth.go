package anthropic

import (
	"fmt"
	"net/http"
	"os"
)

// FoundryAuthenticator abstracts the credential source used for Foundry authentication.
type FoundryAuthenticator interface {
	// Authenticate adds the Foundry api-key header to the given HTTP request.
	Authenticate(req *http.Request) error
}

// apiKeyFoundryAuthenticator implements Foundry authentication using an API key.
type apiKeyFoundryAuthenticator struct {
	apiKey string
}

// Authenticate injects the api-key header into the request.
// It returns an error when the API key is empty.
func (a *apiKeyFoundryAuthenticator) Authenticate(req *http.Request) error {
	if a.apiKey == "" {
		return fmt.Errorf("missing Foundry API key (set ANTHROPIC_FOUNDRY_API_KEY or Config.FoundryAPIKey)")
	}
	req.Header.Set("api-key", a.apiKey)
	return nil
}

// noopFoundryAuthenticator does not authenticate requests. It is used when
// CLAUDE_CODE_SKIP_FOUNDRY_AUTH is set (testing / proxy scenarios).
type noopFoundryAuthenticator struct{}

func (a *noopFoundryAuthenticator) Authenticate(_ *http.Request) error {
	return nil
}

// newFoundryAuthenticator builds the appropriate authenticator based on configuration.
// Priority: Skip auth > Config API key > ANTHROPIC_FOUNDRY_API_KEY environment variable.
func newFoundryAuthenticator(skipAuth bool, cfgAPIKey string) FoundryAuthenticator {
	if skipAuth {
		return &noopFoundryAuthenticator{}
	}
	if cfgAPIKey != "" {
		return &apiKeyFoundryAuthenticator{apiKey: cfgAPIKey}
	}
	if key := os.Getenv("ANTHROPIC_FOUNDRY_API_KEY"); key != "" {
		return &apiKeyFoundryAuthenticator{apiKey: key}
	}
	return &apiKeyFoundryAuthenticator{}
}
