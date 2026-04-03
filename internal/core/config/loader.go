package config

import "context"

type Loader interface {
	Load(ctx context.Context) (Config, error)
}
