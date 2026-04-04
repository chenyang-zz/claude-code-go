package config

import "context"

// Loader returns the effective runtime configuration after reading the supported sources.
type Loader interface {
	Load(ctx context.Context) (Config, error)
}
