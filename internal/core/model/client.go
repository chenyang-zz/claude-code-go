package model

import "context"

// Client streams model responses for one request at a time.
type Client interface {
	Stream(ctx context.Context, req Request) (Stream, error)
}
