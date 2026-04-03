package command

import "context"

type Command interface {
	Name() string
	Execute(ctx context.Context, args Args) error
}
