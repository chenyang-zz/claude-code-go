package session

import "context"

type Repository interface {
	Save(ctx context.Context, session Session) error
	Load(ctx context.Context, id string) (Session, error)
}
