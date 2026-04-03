package model

import "context"

type Client interface {
	Stream(ctx context.Context, req Request) (Stream, error)
}
